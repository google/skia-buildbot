// pushk pushes a new version of an app.
//
// Actually just modifies kubernetes yaml files with the correct tag for the
// given image. Prints the kubectl command to run to apply the changes,
// or actually runs them if --apply is supplied.
//
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
)

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

func main() {
	common.Init()

	ts := auth.NewGCloudTokenSource(*project)

	imageName := flag.Arg(0)

	if *apply {
		flag.Lookup("logtostderr").Value.Set("true")
	}

	// imageRegex has the following groups returned on match:
	// 0 - the entire line
	// 1 - the prefix, i.e. image:, with correct spacing.
	// 2 - full image name
	// 3 - just the tag
	imageRegex := regexp.MustCompile(fmt.Sprintf(`^(\s+image:\s+)(%s\/%s\/%s:(\S+))\s*$`, gcr.SERVER, *project, imageName))

	// Get all the yaml files.
	filenames, err := filepath.Glob(filepath.Join(*kubeDir, *project, "*.yaml"))
	if err != nil {
		sklog.Fatal(err)
	}

	// Get all the tags for the selected image.
	tags, err := gcr.NewClient(ts, *project, imageName).Tags()
	if err != nil {
		sklog.Fatal(err)
	}
	if len(tags) == 0 {
		sklog.Fatal(fmt.Errorf("Not enough tags returned."))
	}
	validTags := []string{}
	for _, t := range tags {
		if validTag.MatchString(t) {
			validTags = append(validTags, t)
		}
	}
	sort.Strings(validTags)

	// Pick the target tag we want to move to.
	tag := validTags[len(validTags)-1]
	if *rollback {
		if len(validTags) < 2 {
			sklog.Fatal(fmt.Errorf("No version to rollback to."))
		}
		tag = validTags[len(validTags)-2]
	}
	image := fmt.Sprintf("%s/%s/%s:%s", gcr.SERVER, *project, imageName, tag)

	// Loop over all the yaml files and update tags for the given imageName.
	changed := map[string]bool{}
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
				lines[i] = matches[1] + image
			}
		}
		if changed[filename] {
			f, err := ioutil.TempFile(*kubeDir, "pushk")
			if err != nil {
				sklog.Fatalf("Failed to create temp file: %s", err)
			}
			if _, err := f.WriteString(strings.Join(lines, "\n")); err != nil {
				sklog.Fatalf("Failed to write updated file: %s", err)
			}
			if err := f.Close(); err != nil {
				sklog.Fatalf("Failed to close updated file: %s", err)
			}
			if err := os.Remove(filename); err != nil {
				sklog.Fatalf("Failed to remove old config file: %s", err)
			}
			if err := os.Rename(f.Name(), filename); err != nil {
				sklog.Fatalf("Failed to rewrite updated file: %s", err)
			}
		}
	}
	if len(changed) != 0 {
		changedFiles := []string{}
		for k, _ := range changed {
			changedFiles = append(changedFiles, k)
		}
		cmd := fmt.Sprintf("\nkubectl apply --filename=%s\n", strings.Join(changedFiles, ","))
		if *apply {
			files := fmt.Sprintf("--filename=%s\n", strings.Join(changedFiles, ","))
			if err := exec.Run(context.Background(), &exec.Command{
				Name:      "kubectl",
				Args:      []string{"apply", files},
				LogStderr: true,
				LogStdout: true,
			}); err != nil {
				sklog.Errorf("Failed to run %q: %s", err)
			}
		} else {
			fmt.Println(cmd)
		}
	} else {
		fmt.Println("Nothing to do.")
	}
}
