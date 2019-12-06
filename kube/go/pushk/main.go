// pushk pushes a new version of an app.
//
// pushk docserver
// pushk --rollback docserver
// pushk --cluster=skia-public docserver
// pushk --rollback --cluster=skia-corp docserver
// pushk --dry-run my-new-service
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

	"github.com/spf13/viper"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcr"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	repoURLTemplate = "https://skia.googlesource.com/k8s-config"

	containerRegistryProject = "skia-public"

	maxListSize = 10
)

// Project is used to map cluster name to GCE project info.
type Project struct {
	Zone    string
	Project string // The full project name, e.g. google.com:skia-corp.
}

var (
	clusters = map[string]*Project{
		"skia-public": {
			Zone:    "us-central1-a",
			Project: "skia-public",
		},
		"skia-corp": {
			Zone:    "us-central1-a",
			Project: "google.com:skia-corp",
		},
	}
)

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: pushk <flags> [one or more image names]\n\n")
		fmt.Printf(`pushk pushes a new version of an app.

The command:
  1. Modifies the kubernetes yaml files with the new image.
  2. Commits the changes to the config repo.
  3. Applies the changes with kubectl.

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
  pushk docserver --cluster=skia-corp --message="Fix bug #1234"

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
  changed by setting the environment variable PUSKH_GITDIR.
`)
		flag.PrintDefaults()
	}
}

// flags
var (
	// TODO default back to false before sending CL.
	configPath  = flag.String("config-path", "", "Name of directory to find the config file. Config file must be named config.json.")
	dryRun      = flag.Bool("dry-run", true, "If true then do not run the kubectl command to apply the changes, and do not commit the changes to the config repo.")
	ignoreDirty = flag.Bool("ignore-dirty", false, "If true, then do not fail out if the git repo is dirty.")
	list        = flag.Bool("list", false, "List the last few versions of the given image.")
	message     = flag.String("message", "Push", "Message to go along with the change.")
	rollback    = flag.Bool("rollback", false, "If true go back to the second most recent image, otherwise use most recent image.")
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

func switchTo(clusterName string) error {
	// Switch kubectl to the right project.
	if viper.GetString(fmt.Sprintf("clusters.%s.type", clusterName)) != "gke" {
		// TODO - Add support for k3s clusters.
		return fmt.Errorf("Unknown type of cluster.")
	}
	zone := viper.GetString(fmt.Sprintf("clusters.%s.zone", clusterName))
	project := viper.GetString(fmt.Sprintf("clusters.%s.project", clusterName))
	return exec.Run(context.Background(), &exec.Command{
		Name: "gcloud",
		Args: []string{
			"container",
			"clusters",
			"get-credentials",
			clusterName,
			"--zone",
			zone,
			"--project",
			project,
		},
		LogStderr: true,
		LogStdout: true,
	})
}

func main() {
	common.Init()

	viper.SetEnvPrefix("pushk") // will be uppercased automatically
	viper.BindEnv("gitdir")     // PUSHK_GITDIR will override "gitdir" in the config file.

	// The config is stored in /infra/kube/clusters/config.json.
	viper.SetConfigName("config") // name of config file (without extension)

	// Set the config path, start with flag, fall back to relative location in
	// the source tree.
	configPath := *configPath
	if configPath == "" {
		_, filename, _, _ := runtime.Caller(0)
		configPath = filepath.Join(filepath.Dir(filename), "../../clusters")
	}
	viper.AddConfigPath(configPath)

	err := viper.ReadInConfig()
	if err != nil {
		sklog.Fatal(err)
	}

	ctx := context.Background()
	checkout, err := git.NewCheckout(ctx, viper.GetString("repo"), viper.GetString("gitdir"))
	if err != nil {
		sklog.Fatalf("Failed to check out config repo: %s", err)
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

	// Get all the yaml files.
	filenames, err := filepath.Glob(filepath.Join(checkout.Dir(), "/*/*.yaml"))
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Printf("filenames: %v", filenames)

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
		imageRegex := regexp.MustCompile(fmt.Sprintf(`^(\s+image:\s+)(%s):.*$`, imageNoTag))

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
				if len(matches) != 3 {
					continue
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

		// Find all the directory names, which are really cluster names.
		// filenames will be absolute directory names, e.g.
		// /tmp/k8s-config/skia-public/task-scheduler-be-staging.yaml
		byCluster := map[string][]string{}

		// The first part of that is
		for _, filename := range changed.Keys() {
			// /tmp/k8s-config/skia-public/task-scheduler-be-staging.yaml => skia-public/task-scheduler-be-staging.yaml
			rel, err := filepath.Rel(checkout.Dir(), filename)
			if err != nil {
				continue
			}
			// skia-public/task-scheduler-be-staging.yaml => skia-public   task-scheduler-be-staging.yaml
			cluster, _ := filepath.Split(rel)
			arr, ok := byCluster[cluster]
			if !ok {
				arr = []string{filename}
			} else {
				arr = append(arr, filename)
			}
			byCluster[cluster] = arr
		}

		// Then loop over cluster names and apply all changed files for that
		// cluster.
		for cluster, files := range byCluster {
			// Switch to the correct cluster.
			if err := switchTo(cluster); err != nil {
				sklog.Fatalf("Failed to switch to the right cluster: %s", err)
			}

			filenameFlag := fmt.Sprintf("--filename=%s\n", strings.Join(files, ","))
			if !*dryRun {
				for filename := range changed {
					msg, err := checkout.Git(ctx, "add", filepath.Base(filename))
					if err != nil {
						sklog.Fatalf("Failed to stage changes to the config repo: %s: %q", err, msg)
					}
				}

				if err := exec.Run(context.Background(), &exec.Command{
					Name:      "kubectl",
					Args:      []string{"apply", filenameFlag},
					LogStderr: true,
					LogStdout: true,
				}); err != nil {
					sklog.Errorf("Failed to run: %s", err)
				}
			} else {
				fmt.Printf("\nkubectl apply %s\n", filenameFlag)
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
		msg, err = checkout.Git(ctx, "push", "origin", "master")
		if err != nil {
			sklog.Fatalf("Failed to push the config repo: %s: %q", err, msg)
		}
	} else {
		fmt.Println("Nothing to do.")
	}
}
