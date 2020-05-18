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

	dirtyConfigChecksPeriod = flag.Duration("dirty_config_checks_period", 2*time.Minute, "How often to check for dirty configs/images in K8s.")
	configFile              = flag.String("config_file", "", "The location of the config.json file that describes all the clusters.")
	cluster                 = flag.String("cluster", "skia-public", "The k8s cluster name.")
	local                   = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort                = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	workdir                 = flag.String("workdir", "/tmp/", "Directory to use for scratch work.")

	// // It is assumed that tasks.json is always in this path for Skia repositories.
	// tasksJsonRelPath = filepath.Join("infra", "bots", "tasks.json")
	bbCfgTemplateParsed = template.Must(template.New("buildbucket_config").Parse(bbCfgTemplate))
)

// setupGit acquires necessary credentials to clone the repo.
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

	gUrl := strings.Split(*repoUrl, ".googlesource.com")[0] + "-review.googlesource.com"
	g, err := gerrit.NewGerrit(gUrl, httpClient)
	if err != nil {
		sklog.Fatal(err)
	}

	// This should read commit from Skia master branch.
	repo := gitiles.NewRepo(*repoUrl, httpClient)
	// baseCommitInfo, err := repo.Details(ctx, bbCfgBranch)
	// if err != nil {
	// 	sklog.Fatal(err)
	// }
	// baseCommit := baseCommitInfo.Hash

	// Read tasks.json.
	var tasksBuf bytes.Buffer
	if err := repo.ReadFileAtRef(ctx, specs.TASKS_CFG_FILE, "master", &tasksBuf); err != nil {
		sklog.Fatal(err)
	}
	tasksCfg, err := specs.ParseTasksCfg(tasksBuf.String())
	if err != nil {
	}

	jobs := make([]string, 0, len(tasksCfg.Jobs))
	// jobs := []string{}
	for j := range tasksCfg.Jobs {
		jobs = append(jobs, j)
	}
	sort.Strings(jobs)
	fmt.Println("THESE ARE THE JOBS!")
	fmt.Println(jobs)
	fmt.Println(len(jobs))
	fmt.Println(len(tasksCfg.Jobs))

	// return

	newBBConfig := new(bytes.Buffer)
	if err := bbCfgTemplateParsed.Execute(newBBConfig, struct {
		EmptyBuckets []string
		BucketName   string
		Jobs         []string
	}{
		EmptyBuckets: *emptyBuckets,
		BucketName:   *bucketName,
		Jobs:         jobs,
	}); err != nil {
		sklog.Fatalf("Failed to execute bbcfg template: %s", err)
	}

	// Find what the existing buildbucket.config is.
	var buf bytes.Buffer
	if err := repo.ReadFileAtRef(ctx, bbCfgFileName, bbCfgBranch, &buf); err != nil {
		sklog.Fatal(err)
	}

	fmt.Println("OLD CONTENTS ARE THESE:")
	fmt.Println(buf.String())
	fmt.Println("----")

	fmt.Println("NEW CONTENTS ARE THESE:")
	fmt.Println(newBBConfig.String())
	fmt.Println("----")

	fmt.Println(buf.String() == newBBConfig.String())

	return

	newCfg := buf.String()
	newCfg += "\nTesting\n"

	// Create the Gerrit CL.
	commitMsg := "Update buildbucket.config"
	repoSplit := strings.Split(*repoUrl, "/")
	project := strings.TrimSuffix(repoSplit[len(repoSplit)-1], ".git")
	ci, err := gerrit.CreateAndEditChange(ctx, g, project, bbCfgBranch, commitMsg, bbCfgBranch, func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		if err := g.EditFile(ctx, ci, bbCfgFileName, string(newCfg)); err != nil {
			sklog.Fatal(err)
			// return err
		}
		return nil
	})
	if err != nil {
		sklog.Fatal(err)
		// sklog.Fatal(err)
	}
	sklog.Infof("Uploaded change https://%s-review.googlesource.com/%d", project, ci.Issue)
	if *submit {
		if ci.WorkInProgress {
			if err := g.SetReadyForReview(ctx, ci); err != nil {
				sklog.Fatalf("Failed to set ready for review: %s", err)
			}
		}
		// ADD REVIEWER???
		if err := g.SetReview(ctx, ci, "", gerrit.CONFIG_CHROMIUM.SelfApproveLabels, []string{"rmistry@google.com"} /* TODO(rmistry): Change to nil after verifying things work */); err != nil {
			sklog.Fatalf("Failed to set Code-Review+1: %s", err)
		}
		// if err := g.Submit(ctx, ci); err != nil {
		// 	sklog.Fatalf("Failed to submit CL: %s", err)
		// }
	}

	// Need to create my own.

	// var bbCfg buildbucketpb.BuildbucketCfg
	// if err := proto.UnmarshalText(buf.String(), &bbCfg); err != nil {
	// 	sklog.Fatal(err)
	// }
	// fmt.Println("DONE")
	// fmt.Println(bbCfg.Buckets)

	// go util.RepeatCtx(ctx, *pollingPeriod, func(ctx context.Context) {
	// 	newMetrics, err := performChecks(ctx, clientset, gitiles.NewRepo(k8sYamlRepo, httpClient), oldMetrics)
	// 	if err != nil {
	// 		sklog.Errorf("Error when checking for dirty configs: %s", err)
	// 	}
	// })

	// For now
	// select {}
}
