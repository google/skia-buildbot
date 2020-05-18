// bb_cfg_updater is an application that updates a repo's buildbucket.config file.
// k8s_checker is an application that checks for the following and alerts if necessary:
// * Dirty images checked into K8s config files.
// * Dirty configs running in K8s.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	// "github.com/golang/protobuf/proto"
	// buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	// The format of this file is that of a gerrit config (not a proto).
	// The buildbucket extension parses the config like this: https://chromium.googlesource.com/infra/gerrit-plugins/buildbucket/+/refs/heads/master/src/main/java/com/googlesource/chromium/plugins/buildbucket/GetConfig.java
	bbCfgFileName = "buildbucket.config"
	// Branch buildbucket.config lies in. Hopefully this will change one day, see b/38258213.
	bbCfgBranch = "refs/meta/config"
	// // The name of the file that contains list of tasks to populate buildbucket.config file.
	// tasksJsonFileName = "tasks.json"

	bbCfgTemplate = `
{{- range .EmptyBuckets}}[bucket "{{.}}"]{{end}}
[bucket "{{.BucketName}}"]
{{- range .Jobs}}
	builder = {{.}}
{{- end}}
`
)

var (
	// Flags.
	repoUrl = flag.String("repo_url", common.REPO_SKIA, "Repo that needs buildbucket.config updated from it's tasks.json file.")
	// gitilesPathToBBConfig = flag.String("gitiles_bb_config", "", "Gititles path to buildbucket.config. Eg: https://skia.googlesource.com/skia/+/refs/meta/config/buildbucket.config")
	bucketName    = flag.String("bucket_name", "", "Name of the bucket to update in buildbucket.config. Eg: luci.skia.skia.primary")
	emptyBuckets  = common.NewMultiStringFlag("empty_bucket", nil, "Empty buckets to specify in buildbucket.config. Eg: luci.chromium.try. See skbug.com/9639 for why these buckets are empty.")
	pollingPeriod = flag.Duration("polling_period", 1*time.Minute, "How often to poll tasks.json.")
	submit        = flag.Bool("submit", false, "If set, automatically submit the Gerrit change to update buildbucket.config")
	local         = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort      = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")

	bbCfgTemplateParsed = template.Must(template.New("buildbucket_config").Parse(bbCfgTemplate))
)

// Maybe not needed. What about for gerrit.
func setupGit() error {
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
	if err != nil {
		return err
	}
	if !*local {
		if _, err := gitauth.New(ts, filepath.Join(os.TempDir(), "gitcookies"), true, ""); err != nil {
			return skerr.Wrapf(err, "Failed to create git cookie updater")
		}
	}
	return nil
}

// getBuildbucketCfgFromJobs reads tasks.json from the specified repository and returns
// contents of what the new buildbucket.config file should be.
func getBuildbucketCfgFromJobs(ctx context.Context, repo *gitiles.Repo) (string, error) {
	// Read tasks.json from the specified repository and create a sorted slice of jobs.
	var tasksBuf bytes.Buffer
	if err := repo.ReadFileAtRef(ctx, specs.TASKS_CFG_FILE, "master", &tasksBuf); err != nil {
		return "", fmt.Errorf("Could not read %s: %s", specs.TASKS_CFG_FILE, err)
	}
	tasksCfg, err := specs.ParseTasksCfg(tasksBuf.String())
	if err != nil {
		return "", fmt.Errorf("Could not parse %s: %s", specs.TASKS_CFG_FILE, err)
	}

	jobs := make([]string, 0, len(tasksCfg.Jobs))
	for j := range tasksCfg.Jobs {
		jobs = append(jobs, j)
	}
	sort.Strings(jobs)

	// Use jobs to create content of buildbucket.config.
	bbCfg := new(bytes.Buffer)
	if err := bbCfgTemplateParsed.Execute(bbCfg, struct {
		EmptyBuckets []string
		BucketName   string
		Jobs         []string
	}{
		EmptyBuckets: *emptyBuckets,
		BucketName:   *bucketName,
		Jobs:         jobs,
	}); err != nil {
		return "", fmt.Errorf("Failed to execute bbCfg template: %s", err)
	}
	return bbCfg.String(), nil
}

// getCurrentBuildbucketCfg returns the current contents of buildbucket.config for the
// specified repository.
func getCurrentBuildbucketCfg(ctx context.Context, repo *gitiles.Repo) (string, error) {
	var buf bytes.Buffer
	if err := repo.ReadFileAtRef(ctx, bbCfgFileName, bbCfgBranch, &buf); err != nil {
		return "", fmt.Errorf("Could not read %s: %s", bbCfgFileName, err)
	}
	return buf.String(), nil
}

func updateBuildbucketCfg(ctx context.Context, g *gerrit.Gerrit, repo *gitiles.Repo, cfgContents string) error {
	// Create the Gerrit CL.
	commitMsg := "Update buildbucket.config"
	repoSplit := strings.Split(*repoUrl, "/")
	project := strings.TrimSuffix(repoSplit[len(repoSplit)-1], ".git")
	baseCommitInfo, err := repo.Details(ctx, bbCfgBranch)
	if err != nil {
		return fmt.Errorf("Could not get details of %s: %s", bbCfgBranch, err)
	}
	baseCommit := baseCommitInfo.Hash
	ci, err := gerrit.CreateAndEditChange(ctx, g, project, bbCfgBranch, commitMsg, baseCommit, func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		if err := g.EditFile(ctx, ci, bbCfgFileName, cfgContents); err != nil {
			return fmt.Errorf("Could not edit %s: %s", bbCfgFileName, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("Could not create Gerrit change: %s", err)
	}
	sklog.Infof("Uploaded change https://%s-review.googlesource.com/%d", project, ci.Issue)

	if *submit {
		if ci.WorkInProgress {
			if err := g.SetReadyForReview(ctx, ci); err != nil {
				return fmt.Errorf("Failed to set ready for review: %s", err)
			}
		}
		if err := g.SetReview(ctx, ci, "", gerrit.CONFIG_CHROMIUM.SelfApproveLabels, []string{"rmistry@google.com"} /* TODO(rmistry): Change to nil after verifying things work */); err != nil {
			return fmt.Errorf("Failed to set approve CL: %s", err)
		}
		// if err := g.Submit(ctx, ci); err != nil {
		// 	return fmt.Errorf("Failed to submit CL: %s", err)
		// }
	}

	return nil
}

func main() {
	common.InitWithMust("bb_cfg_updater", common.PrometheusOpt(promPort))
	defer sklog.Flush()
	ctx := context.Background()

	if err := setupGit(); err != nil {
		sklog.Fatalf("Failed to setup git: %s", err)
	}

	// OAuth2.0 TokenSource.
	ts, err := auth.NewDefaultTokenSource(false, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatal(err)
	}
	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Instantiate Gerrit.
	gUrl := strings.Split(*repoUrl, ".googlesource.com")[0] + "-review.googlesource.com"
	g, err := gerrit.NewGerrit(gUrl, httpClient)
	if err != nil {
		sklog.Fatal(err)
	}

	// Instantiate gitiles using the specified repo URL.
	repo := gitiles.NewRepo(*repoUrl, httpClient)

	go util.RepeatCtx(ctx, *pollingPeriod, func(ctx context.Context) {
		existingCfg, err := getCurrentBuildbucketCfg(ctx, repo)
		if err != nil {
			sklog.Errorf("Could not get contents of buildbucket.config from %s: %s", repo, err)
		}

		newCfg, err := getBuildbucketCfgFromJobs(ctx, repo)
		if err != nil {
			sklog.Errorf("Could not get list of jobs from %s: %s", repo, err)
		}

		// Only update buildbucket.config if the config is different.
		newCfg += "testing"
		if newCfg != existingCfg {
			fmt.Println("NEED TO UPDATE!!!!")
			fmt.Println("NEW NEW NEW")
			fmt.Println(newCfg)
			fmt.Println("OLD OLD OLD")
			fmt.Println(existingCfg)
			if err := updateBuildbucketCfg(ctx, g, repo, newCfg); err != nil {
				sklog.Errorf("Could not update buildbucket.config: %s", err)
			}
		} else {
			fmt.Println("NO NEED TO UPDATE!!")
		}
	})

	select {}
}
