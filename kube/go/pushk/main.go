// pushk pushes a new version of an app.
//
// See flag.Usage for details.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcr"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/kube/clusterconfig"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// containerRegistryProject is the GCP project in which we store our Docker
	// images via Google Cloud Container Registry.
	containerRegistryProject = "skia-public"

	// Max number of revisions of an image to print when using --list.
	maxListSize = 10

	// All dirty images are tagged with this suffix.
	dirtyImageTagSuffix = "-dirty"
)

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: pushk <flags> [one or more image names]\n\n")
		fmt.Printf(`pushk pushes a new version of an app.

The command:
  1. Searches through the checked in kubernetes yaml files to determine which use the image(s).
  2. Modifies the kubernetes yaml files with the new version of the image(s).
  3. Applies the changes with kubectl.
  4. Commits the changes to the config repo.

The config is stored in a separate repo that will automaticaly be checked out
under /tmp by default, or the value of the PUSHK_GITDIR environment variable if set.

The command applies the changes by default, or just changes the local yaml files
if --dry-run is supplied.

Examples:
  # Push an exact tag.
  pushk gcr.io/skia-public/fiddler:694900e3ca9468784a5794dc53382d1c8411ab07

  # Push the latest version of docserver.
  pushk docserver --message="Fix bug #1234"

  # Push the latest version of docserver to the skia-corp cluster.
  pushk docserver --only-cluster=skia-corp --message="Fix bug #1234"

  # Push the latest version of docserver and iap-proxy
  pushk docserver iap-proxy

  # Rollback docserver.
  pushk --rollback docserver

  # List the last few versions of the docserver image. Doesn't apply anything.
  pushk --list docserver

  # Compute any changes a push to docserver will make, but do not apply them.
  # Note that the YAML file(s) will be updated, but not committed or pushed.
  pushk --dry-run docserver

ENV:

  The config repo is checked out by default into '/tmp'. This can be
  changed by setting the environment variable PUSHK_GITDIR.
`)
		flag.PrintDefaults()
	}
}

// flags
var (
	onlyCluster             = flag.String("only-cluster", "", "If set then only push to the specified cluster.")
	configFile              = flag.String("config-file", "", "Absolute filename of the config.json file.")
	dryRun                  = flag.Bool("dry-run", false, "If true then do not run the kubectl command to apply the changes, and do not commit the changes to the config repo.")
	ignoreDirty             = flag.Bool("ignore-dirty", false, "If true, then do not fail out if the git repo is dirty.")
	list                    = flag.Bool("list", false, "List the last few versions of the given image.")
	message                 = flag.String("message", "Push", "Message to go along with the change.")
	rollback                = flag.Bool("rollback", false, "If true go back to the second most recent image, otherwise use most recent image.")
	runningInK8s            = flag.Bool("running-in-k8s", false, "If true, then does not use flags that do not work in the k8s environment. Eg: '--cluster' when doing 'kubectl apply'.")
	doNotOverrideDirtyImage = flag.Bool("do-not-override-dirty-image", false, "If true, then do not push if the latest checkedin image is dirty. Caveat: This only checks the k8s-config repository to determine if image is dirty, it does not check the live running k8s containers.")
	useTempCheckout         = flag.Bool("use-temp-checkout", false, "If true, checks out the config repo into a temporary directory and pushes from there.")
	verbose                 = flag.Bool("verbose", false, "Verbose runtime diagnostics.")
)

var (
	validTag = regexp.MustCompile(`^\d\d\d\d-\d\d-\d\dT\d\d_\d\d_\d\dZ-.+$`)
)

// filter strips the list of tags down to only the ones that conform to our
// constraints and also checks that there are enough tags. The results
// are sorted in ascending order, so oldest tags are first, newest tags
// are last.
func filter(tags []string) ([]string, error) {
	validTags := []string{}
	for _, t := range tags {
		if validTag.MatchString(t) {
			validTags = append(validTags, t)
		}
	}
	sort.Strings(validTags)
	if len(validTags) == 0 {
		return nil, fmt.Errorf("Not enough tags returned.")
	}
	return validTags, nil
}

// tagProvider is a type that returns the correct tag to push for the given imageName.
type tagProvider func(imageName string) ([]string, error)

// imageFromCmdLineImage handles image names, which can be either short, ala 'fiddler', or exact,
// such as gcr.io/skia-public/fiddler:694900e3ca9468784a5794dc53382d1c8411ab07, both of which can
// appear on the command-line.
func imageFromCmdLineImage(imageName string, tp tagProvider) (string, error) {
	if strings.HasPrefix(imageName, "gcr.io/") {
		if *rollback {
			return "", fmt.Errorf("Supplying a fully qualified image name and the --rollback flag are mutually exclusive.")
		}
		if *list {
			return "", fmt.Errorf("Supplying a fully qualified image name and the --list flag are mutually exclusive.")
		}
		return imageName, nil
	}
	// Get all the tags for the selected image.
	tags, err := tp(imageName)
	if err != nil {
		return "", fmt.Errorf("Tag provider failed: %s", err)
	}

	// Filter the tags
	tags, err = filter(tags)
	if err != nil {
		return "", fmt.Errorf("Failed to filter: %s", err)
	}

	if *list {
		if len(tags) > maxListSize {
			tags = tags[len(tags)-maxListSize:]
		}
		for _, tag := range tags {
			fmt.Println(tag)
		}
	}

	// Pick the target tag we want to move to.
	tag := tags[len(tags)-1]
	if *rollback {
		if len(tags) < 2 {
			return "", fmt.Errorf("No version to rollback to.")
		}
		tag = tags[len(tags)-2]
	}

	// The full docker image name and tag of the image we want to deploy.
	return fmt.Sprintf("%s/%s/%s:%s", gcr.SERVER, containerRegistryProject, imageName, tag), nil
}

// byClusterFromChanged returns a map from cluster name to the list of modified
// files in that cluster.
func byClusterFromChanged(gitDir string, changed util.StringSet) (map[string][]string, error) {
	// Find all the directory names, which are really cluster names.
	// filenames will be absolute directory names, e.g.
	// /tmp/k8s-config/skia-public/task-scheduler-be-staging.yaml
	byCluster := map[string][]string{}

	// The first part of that is
	for _, filename := range changed.Keys() {
		// /tmp/k8s-config/skia-public/task-scheduler-be-staging.yaml => skia-public/task-scheduler-be-staging.yaml
		rel, err := filepath.Rel(gitDir, filename)
		if err != nil {
			return nil, err
		}
		// skia-public/task-scheduler-be-staging.yaml => skia-public
		cluster := filepath.Dir(rel)
		arr, ok := byCluster[cluster]
		if !ok {
			arr = []string{filename}
		} else {
			arr = append(arr, filename)
		}
		byCluster[cluster] = arr
	}
	return byCluster, nil
}

func main() {
	common.Init()

	ctx := context.Background()
	cfg, checkout, err := clusterconfig.NewWithCheckout(ctx, *configFile)
	if err != nil {
		sklog.Fatal(err)
	}
	if *useTempCheckout {
		tmp, err := git.NewTempCheckout(ctx, cfg.Repo)
		if err != nil {
			sklog.Fatal(err)
		}
		defer tmp.Delete()
		checkout = (*git.Checkout)(tmp)
	}

	output, err := checkout.Git(ctx, "status", "-s")
	if err != nil {
		sklog.Fatal(err)
	}
	if strings.TrimSpace(output) != "" {
		if !*ignoreDirty {
			sklog.Fatalf("Found dirty checkout in %s:\n%s", checkout.Dir(), output)
		}
	} else {
		if err := checkout.Update(ctx); err != nil {
			sklog.Fatal(err)
		}
	}

	dirMatch := "*"
	if *onlyCluster != "" {
		dirMatch = *onlyCluster
	}
	glob := fmt.Sprintf("/%s/*.yaml", dirMatch)
	// Get all the yaml files.
	filenames, err := filepath.Glob(filepath.Join(checkout.Dir(), glob))
	if err != nil {
		sklog.Fatal(err)
	}

	tokenSource := auth.NewGCloudTokenSource(containerRegistryProject)
	imageNames := flag.Args()
	if len(imageNames) == 0 {
		fmt.Printf("At least one image name needs to be supplied.")
		flag.Usage()
		os.Exit(1)
	}
	sklog.Infof("Pushing the following images: %q", imageNames)

	gcrTagProvider := func(imageName string) ([]string, error) {
		return gcr.NewClient(tokenSource, containerRegistryProject, imageName).Tags()
	}

	// Search through the yaml files looking for those that use the provided image names.
	changed := util.StringSet{}
	for _, imageName := range imageNames {
		image, err := imageFromCmdLineImage(imageName, gcrTagProvider)
		if err != nil {
			sklog.Fatal(err)
		}
		if *list {
			// imageFromCmdLineImage printed out the tags, so nothing more to do.
			continue
		}

		// imageRegex has the following groups returned on match:
		// 0 - the entire line
		// 1 - the prefix, i.e. image:, with correct spacing.
		// 2 - full image name
		// 3 - just the tag
		//
		// We pull out the 'prefix' so we can use it when
		// we rewrite the image: line so the indent level is
		// unchanged.
		parts := strings.SplitN(image, ":", 2)
		if len(parts) != 2 {
			sklog.Fatalf("Failed to split imageName: %v", parts)
		}
		imageNoTag := parts[0]
		imageRegex := regexp.MustCompile(fmt.Sprintf(`^(\s+image:\s+)(%s):(.*)$`, imageNoTag))

		// Loop over all the yaml files and update tags for the given imageName.
		for _, filename := range filenames {
			b, err := ioutil.ReadFile(filename)
			if err != nil {
				sklog.Errorf("Failed to read %q (skipping): %s", filename, err)
				continue
			}
			lines := strings.Split(string(b), "\n")
			for i, line := range lines {
				matches := imageRegex.FindStringSubmatch(line)
				if len(matches) != 4 {
					continue
				}
				if *doNotOverrideDirtyImage && strings.HasSuffix(matches[3], dirtyImageTagSuffix) {
					sklog.Infof("%s is dirty. Not pushing to it since --do-not-override-dirty-image is set.", image)
					continue
				}

				if *verbose {
					fmt.Printf("Changed file: %s to image: %s\n", filename, image)
				}

				changed[filename] = true
				lines[i] = matches[1] + image
			}
			if changed[filename] {
				err := util.WithWriteFile(filename, func(w io.Writer) error {
					_, err := w.Write([]byte(strings.Join(lines, "\n")))
					return err
				})
				if err != nil {
					sklog.Fatalf("Failed to write update config file %q: %s", filename, err)
				}
			}
		}
	}

	// Were any files updated?
	if len(changed) != 0 {
		byCluster, err := byClusterFromChanged(checkout.Dir(), changed)
		if err != nil {
			sklog.Fatal(err)
		}

		// Find the location of the attach.sh shell script.
		_, filename, _, _ := runtime.Caller(0)
		attachFilename := filepath.Join(filepath.Dir(filename), "../../attach.sh")

		// Then loop over cluster names and apply all changed files for that
		// cluster.
		for cluster, files := range byCluster {
			if *verbose {
				fmt.Printf("Starting to apply changes to cluster: %s\n", cluster)
			}

			filenameFlag := fmt.Sprintf("--filename=%s\n", strings.Join(files, ","))

			// By default run everything through infra/kube/attach.sh.
			name := attachFilename
			kubectlArgs := []string{cluster, "kubectl", "apply", filenameFlag}
			// But not if we are running in k8s.
			if *runningInK8s {
				name = "kubectl"
				kubectlArgs = []string{"apply", filenameFlag}
			}
			fmt.Printf("\n%s %s\n", name, strings.Join(kubectlArgs, " "))

			if !*dryRun {
				for filename := range changed {
					// /tmp/k8s-config/skia-public/task-scheduler-be-staging.yaml => skia-public/task-scheduler-be-staging.yaml
					rel, err := filepath.Rel(checkout.Dir(), filename)
					if err != nil {
						sklog.Fatal(err)
					}
					msg, err := checkout.Git(ctx, "add", rel)
					if err != nil {
						sklog.Fatalf("Failed to stage changes to the config repo: %s: %q", err, msg)
					}
				}

				if err := exec.Run(context.Background(), &exec.Command{
					Name:      name,
					Args:      kubectlArgs,
					LogStderr: true,
					LogStdout: true,
				}); err != nil {
					sklog.Errorf("Failed to run: %s", err)
				}
			}
		}

		// Once everything is pushed, then commit and push the changes.
		msg, err := checkout.Git(ctx, "diff", "--cached", "--name-only")
		if err != nil {
			sklog.Fatalf("Failed to diff :%s: %q", err, msg)
		}
		if msg == "" {
			sklog.Infof("Not pushing since no files changed.")
			return
		}
		msg, err = checkout.Git(ctx, "commit", "-m", *message)
		if err != nil {
			sklog.Fatalf("Failed to commit to the config repo: %s: %q", err, msg)
		}
		msg, err = checkout.Git(ctx, "push", git.DefaultRemote, git.MasterBranch)
		if err != nil {
			sklog.Fatalf("Failed to push the config repo: %s: %q", err, msg)
		}
	} else {
		fmt.Println("Nothing to do.")
	}
}
