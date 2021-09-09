// This executable builds the Docker images based off the executables in the
// gcr.io/skia-public/infra image. It then issues a PubSub notification to have those apps
// tagged and deployed by docker_pushes_watcher.
// See //docker_pushes_watcher/README.md for more.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/auth"
	docker_pubsub "go.skia.org/infra/go/docker/build/pubsub"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/docker"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	gerritProject = flag.String("gerrit_project", "", "Gerrit project name.")
	gerritUrl     = flag.String("gerrit_url", "", "URL of the Gerrit server.")
	projectId     = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId        = flag.String("task_id", "", "ID of this task.")
	taskName      = flag.String("task_name", "", "Name of the task.")
	workdir       = flag.String("workdir", ".", "Working directory")

	checkoutFlags = checkout.SetupFlags(nil)

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

const (
	leasingImageName = "leasing"
	ctImageName      = "ctfe"
)

var (
	infraCommonEnv = []string{
		"SKIP_BUILD=1",
		"ROOT=/OUT",
	}
)

// hasChangedFilesInDir returns true if any of the specified file paths include the specified dir.
func hasChangedFilesInDir(changedFiles []string, dir string) bool {
	for _, changedFile := range changedFiles {
		if strings.HasPrefix(changedFile, dir) {
			return true
		}
	}
	return false
}

func buildPushCTImage(ctx context.Context, tag, repo, configDir string, changedFiles []string, topic *pubsub.Topic) error {
	if !hasChangedFilesInDir(changedFiles, "ct/") {
		sklog.Infof("Not building and pushing CT image because %s does not include a change to CT.", changedFiles)
		return nil
	}
	tempDir, err := os_steps.TempDir(ctx, "", "")
	if err != nil {
		return err
	}
	image := fmt.Sprintf("gcr.io/skia-public/%s", ctImageName)
	cmd := []string{"/bin/sh", "-c", "cd /home/skia/golib/src/go.skia.org/infra/ct && make release"}
	volumes := []string{fmt.Sprintf("%s:/OUT", tempDir)}
	return docker.BuildPushImageFromInfraImage(ctx, "CT", image, tag, repo, configDir, tempDir, tag, topic, cmd, volumes, infraCommonEnv, nil)
}

func buildPushLeasingImage(ctx context.Context, tag, repo, configDir string, changedFiles []string, topic *pubsub.Topic) error {
	if !hasChangedFilesInDir(changedFiles, "leasing/") {
		sklog.Infof("Not building and pushing leasing image because %s does not include a change to leasing.", changedFiles)
		return nil
	}
	tempDir, err := os_steps.TempDir(ctx, "", "")
	if err != nil {
		return err
	}
	image := fmt.Sprintf("gcr.io/skia-public/%s", leasingImageName)
	cmd := []string{"/bin/sh", "-c", "cd /home/skia/golib/src/go.skia.org/infra/leasing && make release"}
	volumes := []string{fmt.Sprintf("%s:/OUT", tempDir)}
	return docker.BuildPushImageFromInfraImage(ctx, "Leasing", image, tag, repo, configDir, tempDir, tag, topic, cmd, volumes, infraCommonEnv, nil)
}

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	rs, err := checkout.GetRepoState(checkoutFlags)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if *gerritProject == "" {
		td.Fatalf(ctx, "--gerrit_project is required.")
	}
	if *gerritUrl == "" {
		td.Fatalf(ctx, "--gerrit_url is required.")
	}

	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Setup go.
	ctx = golang.WithEnv(ctx, wd)

	// Create token source with scope for cloud registry (storage), pubsub and gerrit.
	ts, err := auth_steps.Init(ctx, *local, auth.ScopeUserinfoEmail, auth.ScopeFullControl, pubsub.ScopePubSub, auth.ScopeGerrit)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Instantiate httpClient for gerrit/gitiles.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Create pubsub client.
	client, err := pubsub.NewClient(ctx, docker_pubsub.TOPIC_PROJECT_ID, option.WithTokenSource(ts))
	if err != nil {
		td.Fatal(ctx, err)
	}
	topic := client.Topic(docker_pubsub.TOPIC)

	// Figure out which tag to use for docker build/push and what the changed files are.
	tag := rs.Revision
	changedFiles := []string{}
	if rs.Issue != "" && rs.Patchset != "" {
		tag = fmt.Sprintf("%s_%s", rs.Issue, rs.Patchset)

		// Call gerrit to get list of changed files.
		g, err := gerrit.NewGerrit(*gerritUrl, httpClient)
		if err != nil {
			td.Fatal(ctx, err)
		}
		issue, err := strconv.ParseInt(rs.Issue, 10, 64)
		if err != nil {
			td.Fatal(ctx, err)
		}
		changedFiles, err = g.GetFileNames(ctx, issue, rs.Patchset)
		if err != nil {
			td.Fatal(ctx, err)
		}
	} else {
		// Call gitiles to get list of changed files.
		g := gitiles.NewRepo(rs.Repo, httpClient)
		diffs, err := g.GetTreeDiffs(ctx, rs.Revision)
		if err != nil {
			td.Fatal(ctx, err)
		}
		for _, d := range diffs {
			changedFiles = append(changedFiles, d.OldPath)
			if d.OldPath != d.NewPath {
				// If paths in diff are different then add them both to list of changed files.
				changedFiles = append(changedFiles, d.NewPath)
			}
		}
	}

	// Create a temporary config dir for Docker.
	configDir, err := ioutil.TempDir("", "")
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer util.RemoveAll(configDir)

	// Login to docker (required to push to docker).
	token, err := ts.Token()
	if err != nil {
		td.Fatal(ctx, err)
	}
	if err := docker.Login(ctx, token.AccessToken, "gcr.io/skia-public/", configDir); err != nil {
		td.Fatal(ctx, err)
	}

	// Build and push all apps of interest below.
	if err := buildPushCTImage(ctx, tag, rs.Repo, configDir, changedFiles, topic); err != nil {
		td.Fatal(ctx, err)
	}
	if err := buildPushLeasingImage(ctx, tag, rs.Repo, configDir, changedFiles, topic); err != nil {
		td.Fatal(ctx, err)
	}
}
