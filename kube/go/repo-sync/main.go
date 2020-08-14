// repo-sync syncs git repos, with the presumption that the destination repo is
// a Google Cloud source repository.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	period      = flag.Duration("period", 15*time.Minute, "How frequently to sync the repos.")
	source      = flag.String("source", common.REPO_SKIA, "URL of the source repo.")
	destination = flag.String("destination", "skia", "Name of the GCP source repo to sync to.")
	project     = flag.String("project", "skia-public", "The GCE project name of the destination repo.")
	workDir     = flag.String("work_dir", "/tmp", "Directory to place the checkout.")
	promPort    = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

// sync does the pull from the source repo and then pushes to the destination repo.
func sync(ctx context.Context) error {
	// git pull "some external repo"
	sklog.Info("pull")
	cmd := fmt.Sprintf("git pull %s %s", *source, git.DefaultBranch)
	sklog.Infof("%q", cmd)
	out, err := exec.RunSimple(ctx, cmd)
	if err != nil {
		sklog.Errorf("git output: %q", out)
		return fmt.Errorf("Failed to pull: %s", err)
	}
	sklog.Infof("pulled: %q", out)
	// git push
	sklog.Info("push")
	out, err = exec.RunSimple(ctx, "git push")
	if err != nil {
		sklog.Errorf("git output: %q", out)
		return fmt.Errorf("Failed to push: %s", err)
	}
	sklog.Infof("pushed: %q", out)
	return nil
}

func main() {
	common.InitWithMust(
		"repo-sync",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	ctx := context.Background()
	repoDir := filepath.Join(*workDir, *destination)
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		sklog.Info("Cloning repo.")
		// gcloud source repos clone skia
		output := bytes.Buffer{}
		err := exec.Run(ctx, &exec.Command{
			Name:           "gcloud",
			Args:           []string{"source", "repos", "clone", *destination, "--project", *project},
			Dir:            *workDir,
			CombinedOutput: &output,
			Timeout:        10 * time.Minute,
		})
		if err != nil {
			sklog.Errorf("gcloud output: %q", output.String())
			sklog.Fatalf("Failed to initially clone the target repo %q: %s", *destination, err)
		}
	} else {
		sklog.Info("Skipping clone, repo directory already exists.")
	}
	if err := os.Chdir(repoDir); err != nil {
		sklog.Fatalf("Failed to change cwd to the repo location: %s", err)
	}

	if err := sync(ctx); err != nil {
		sklog.Fatalf("Failed to sync: %s", err)
	}
	for range time.Tick(*period) {
		if err := sync(ctx); err != nil {
			sklog.Errorf("Failed to sync: %s", err)
		}
	}
}
