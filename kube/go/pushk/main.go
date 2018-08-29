// pushk pushes a new version of an app.
//
// pushk
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
	"sort"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcr"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	repoUrlTemplate = "https://skia.googlesource.com/%s-config"
	repoBaseDir     = "/tmp"
	repoDirTemplate = "/tmp/%s-config"

	containerRegistryProject = "skia-public"
)

// Project is used to map cluster name to GCE project info.
type Project struct {
	Zone    string
	Project string // The full project name, e.g. google.com:skia-corp.
}

var (
	clusters = map[string]*Project{
		"skia-public": &Project{
			Zone:    "us-central1-a",
			Project: "skia-public",
		},
		"skia-corp": &Project{
			Zone:    "us-central1-a",
			Project: "google.com:skia-corp",
		},
	}
)

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: pushk <flags> [zero or more image names]\n\n")
		fmt.Printf(`pushk pushes a new version of an app.

The command:
  1. Modifies the kubernetes yaml files with the new image.
  2. Commits the changes to the config repo.
  3. Applies the changes with kubectl.

The config is stored in a separate repo that will automaticaly be checked out
under /tmp.

The command applies the changes by default, or just changes the local yaml files
if --dry-run is supplied.

If no image names are supplied then pushk looks through all the yaml files for
appropriate images (ones that match the SERVER and project) and tries to push a
new image for each of them.

Examples:
  # Push the latest version of all images from the given container repository.
  pushk

  # Pusk an exact tag.
  pushk gcr.io/skia-public/fiddler:694900e3ca9468784a5794dc53382d1c8411ab07

  # Push the latest version of docserver.
  pushk docserver --message="Fix bug #1234"

  # Push the latest version of docserver to the skia-corp cluster.
  pushk docserver --project=skia-corp --message="Fix bug #1234"

  # Push the latest version of docserver and iap-proxy
  pushk docserver iap-proxy

  # Rollback docserver.
  pushk --rollback docserver

  # Compute any changes a push to docserver will make, but do not apply them.
  pushk --dry-run docserver

`)
		flag.PrintDefaults()
	}
}

// toFullRepoURL converts the project name into a git repo URL.
func toFullRepoURL(s string) string {
	return fmt.Sprintf(repoUrlTemplate, s)

}

// toRepoDir converts the project name into a git repo directory name.
func toRepoDir(s string) string {
	return fmt.Sprintf(repoDirTemplate, s)
}

// flags
var (
	cluster  = flag.String("cluster", "skia-public", "Either 'skia-public' or 'skia-corp'.")
	dryRun   = flag.Bool("dry-run", false, "If true then do not run the kubectl command to apply the changes, and do not commit the changes to the config repo.")
	message  = flag.String("message", "Push", "Message to go along with the change.")
	rollback = flag.Bool("rollback", false, "If true go back to the second most recent image, otherwise use most recent image.")
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

// findAllImageNames searches for all the images that come from the given
// project container registry across all the yaml files listed in filenames.
func findAllImageNames(filenames []string, server, project string) []string {
	// allImageRegex has the following groups returned on match:
	// 0 - the entire line
	// 1 - the image name
	allImageRegex := regexp.MustCompile(fmt.Sprintf(`(?m)^\s+image:\s+%s/%s/([^:]+):\S+\s*$`, server, project))
	filenameSet := util.StringSet{}
	for _, filename := range filenames {
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			sklog.Errorf("Failed to read %q (skipping): %s", filename, err)
			continue
		}
		matches := allImageRegex.FindAllStringSubmatch(string(b), -1)
		for _, m := range matches {
			filenameSet[m[1]] = true
		}
	}
	return filenameSet.Keys()
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

func main() {
	common.Init()

	ctx := context.Background()
	repoDir := toRepoDir(*cluster)
	checkout, err := git.NewCheckout(ctx, toFullRepoURL(*cluster), repoBaseDir)
	if err != nil {
		sklog.Fatalf("Failed to check out config repo: %s", err)
	}
	if err := checkout.Update(ctx); err != nil {
		sklog.Fatalf("Failed to update repo: %s", err)
	}

	// Switch kubectl to the right project.
	p := clusters[*cluster]
	if p == nil {
		fmt.Printf("Invalid value for --cluster flag: %q", *cluster)
		flag.Usage()
		os.Exit(1)
	}

	if err := exec.Run(context.Background(), &exec.Command{
		Name: "gcloud",
		Args: []string{
			"container",
			"clusters",
			"get-credentials",
			*cluster,
			"--zone",
			p.Zone,
			"--project",
			p.Project,
		},
		LogStderr: true,
		LogStdout: true,
	}); err != nil {
		sklog.Errorf("Failed to run: %s", err)
	}

	// Get all the yaml files.
	filenames, err := filepath.Glob(filepath.Join(repoDir, "*.yaml"))
	if err != nil {
		sklog.Fatal(err)
	}

	tokenSource := auth.NewGCloudTokenSource(containerRegistryProject)
	imageNames := flag.Args()
	if len(imageNames) == 0 {
		imageNames = findAllImageNames(filenames, gcr.SERVER, containerRegistryProject)
		if len(imageNames) == 0 {
			fmt.Printf("Failed to find any images that match kubernetes directory: %q and project: %q.", repoDir, containerRegistryProject)
			flag.Usage()
			os.Exit(1)
		}
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
		filenameFlag := fmt.Sprintf("--filename=%s\n", strings.Join(changed.Keys(), ","))
		if !*dryRun {
			for filename, _ := range changed {
				msg, err := checkout.Git(ctx, "add", filepath.Base(filename))
				if err != nil {
					sklog.Fatalf("Failed to stage changes to the config repo: %s: %q", err, msg)
				}
			}
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
	} else {
		fmt.Println("Nothing to do.")
	}
}
