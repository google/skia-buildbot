// pushk pushes a new version of an app.
//
// Actually just modifies kubernetes yaml files with the correct tag for the
// given image.  Either prints the kubectl command to run to apply the changes,
// or actually runs them if --apply is supplied.
//
// pushk docserver
// pushk --rollback docserver
// pushk --project=skia-public docserver
// pushk --rollback --project=skia-public docserver
// pushk --kube_dir=../kube --rollback --project=skia-public docserver
package main

import (
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
	"go.skia.org/infra/go/gcr"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	project  = flag.String("project", "skia-public", "The GCE project name.")
	kubeDir  = flag.String("kube_dir", "../kube", "The directory with the kubernetes config files.")
	rollback = flag.Bool("rollback", false, "If true go back to the second most recent image, otherwise use most recent image.")
)

var (
	// Groups
	// 0 - the prefix
	// 1 - full image name
	// 2 - tag
	imageRegex = regexp.MustCompile(`^(\s+image:\s+)gcr.io\/skia-public\/configmap-reload:(\S+)\s+$`)
)

func main() {
	common.Init()

	ts := auth.NewGCloudTokenSource(*project)

	imageName := flag.Arg(0)
	filenames, err := filepath.Glob(filepath.Join(*kubeDir, *project, "*.yaml"))
	if err != nil {
		sklog.Fatal(err)
	}
	tags, err := gcr.NewClient(ts, *project, imageName).Tags()
	if err != nil {
		sklog.Fatal(err)
	}
	if len(tags) == 0 {
		sklog.Fatal(fmt.Errorf("Not enough tags returned."))
	}
	// TODO(jcgregorio) Strip names that don't match the form added by build_* scripts.
	sort.Strings(tags)
	tag := tags[len(tags)-1]
	if *rollback {
		if len(tags) < 2 {
			sklog.Fatal(fmt.Errorf("No version to rollback to."))
		}
		tag = tags[len(tags)-2]
	}
	image := fmt.Sprintf("%s/%s/%s:%s", gcr.SERVER, *project, imageName, tag)
	fmt.Println(imageName)
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
			if len(matches) != 3 {
				continue
			}
			if matches[2] != tag {
				changed[filename] = true
				lines[i] = matches[0] + image
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
		fmt.Printf("\n\nkubectl apply --filename=%s", strings.Join(changedFiles, ","))
	}
}
