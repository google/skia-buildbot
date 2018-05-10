// pushk pushes a new version of an app.
//
// Actually just modifies kubernetes yaml files with the correct tag for the
// given image. Prints the kubectl command to run to apply the changes,
// or actually applied them if --apply is supplied.
//
// pushk
// pushk docserver
// pushk --rollback docserver
// pushk --project=skia-public docserver
// pushk --rollback --project=skia-public docserver
// pushk --kube_dir=../kube --rollback --apply --project=skia-public docserver
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
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: pushk <flags> [zero or more image names]\n\n")
		fmt.Printf(`pushk pushes a new version of an app.

Actually just modifies kubernetes yaml files with the correct tag for the given
image. Prints the kubectl command to run to apply the changes, or actually
applies them if --apply is supplied.

If no image names are supplied then pushk looks through all the yaml files for
appropriate images (ones that match the SERVER and project) and tries to push a
new image for each of them.

Examples:
  # Push the latest version of all images from the given container repository.
  pushk

  # Push the latest version of docserver.
  pushk docserver

  # Push the latest version of docserver and iap-proxy
  pushk docserver iap-proxy

  # Rollback docserver.
  pushk --rollback docserver

  # Rollback docserver in the skia-public project and immediately apply
  # the kubernetes configs.
  pushk --rollback --apply --project=skia-public docserver

`)
		flag.PrintDefaults()
	}
}

// flags
var (
	project  = flag.String("project", "skia-public", "The GCE project name.")
	kubeDir  = flag.String("kube_dir", "../kube", "The directory with the kubernetes config files.")
	rollback = flag.Bool("rollback", false, "If true go back to the second most recent image, otherwise use most recent image.")
	apply    = flag.Bool("apply", false, "If true then run the kubectl command to apply the changes immediately.")
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

// findAllImageNames searches for all the images that comes from the given
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

func main() {
	common.Init()
	if *apply {
		// If running --apply we force log to stderr to pass through the output of
		// running kubectl.
		_ = flag.Lookup("logtostderr").Value.Set("true")
	}

	// Get all the yaml files.
	filenames, err := filepath.Glob(filepath.Join(*kubeDir, *project, "*.yaml"))
	if err != nil {
		sklog.Fatal(err)
	}

	tokenSource := auth.NewGCloudTokenSource(*project)
	imageNames := flag.Args()
	if len(imageNames) == 0 {
		imageNames = findAllImageNames(filenames, gcr.SERVER, *project)
		if len(imageNames) == 0 {
			fmt.Printf("Failed to find any images that match kubernetes directory: %q and project: %q.", *kubeDir, *project)
			flag.Usage()
			os.Exit(1)
		}
	}
	sklog.Infof("Pushing the following images: %q", imageNames)

	changed := util.StringSet{}
	for _, imageName := range imageNames {
		// imageRegex has the following groups returned on match:
		// 0 - the entire line
		// 1 - the prefix, i.e. image:, with correct spacing.
		// 2 - full image name
		// 3 - just the tag
		//
		// We pull out the 'prefix' so we can use it when
		// we rewrite the image: line so the indent level is
		// unchanged.
		imageRegex := regexp.MustCompile(fmt.Sprintf(`^(\s+image:\s+)(%s/%s/%s:(\S+))\s*$`, gcr.SERVER, *project, imageName))

		// Get all the tags for the selected image.
		tags, err := gcr.NewClient(tokenSource, *project, imageName).Tags()
		if err != nil {
			sklog.Fatal(err)
		}

		// Filter the tags
		tags, err = filter(tags)
		if err != nil {
			sklog.Fatal(err)
		}

		// Pick the target tag we want to move to.
		tag := tags[len(tags)-1]
		if *rollback {
			if len(tags) < 2 {
				sklog.Fatal(fmt.Errorf("No version to rollback to."))
			}
			tag = tags[len(tags)-2]
		}

		// The full docker image name and tag of the image we want to deploy.
		image := fmt.Sprintf("%s/%s/%s:%s", gcr.SERVER, *project, imageName, tag)

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
				if matches[3] != tag {
					changed[filename] = true
					// Replace with the old 'prefix' and our new image.
					lines[i] = matches[1] + image
				}
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
		if *apply {
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
