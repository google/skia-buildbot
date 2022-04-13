// Codereview Watcher checks for open cherrypicks in source repos+branches and
// adds reminder comments about considering cherrypicks into target
// repos+branches.
package main

import (
	"bytes"
	"context"
	"flag"
	"io/fs"
	"sync"
	"text/template"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/oauth2/google"

	"go.skia.org/infra/cherrypick-watcher/go/config"
	"go.skia.org/infra/cherrypick-watcher/go/db"
	cp_gerrit "go.skia.org/infra/cherrypick-watcher/go/gerrit"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	local        = flag.Bool("local", false, "Set to true for local testing.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	fsNamespace  = flag.String("fs_namespace", "cherrypick-watcher-staging", "Typically the instance id. e.g. 'cherrypick-watcher'")
	fsProjectID  = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
	pollInterval = flag.Duration("poll_interval", 2*time.Minute, "How often the server will poll Gerrit for new open cherrypicks.")
	configFile   = flag.String("config_file", "test.json", "Path to the config file, such as prod.json or test.json as found in cherrypick-watcher/go/config")

	// Mutex used to make sure only one poller runs at a time.
	pollerMtx sync.Mutex

	// In-memory cache of the cherrypicks we have already processed to avoid
	// hitting the DB everytime.
	processedCache map[string]interface{}
)

const (
	gerritCommentTxt = `Friendly reminder:
Should this cherrypick into {{.SourceBranch}} also be cherrypicked into {{.TargetBranch}}?
{{.CustomMessage}}Thanks!`
)

type gerritCommentVars struct {
	SourceBranch  string
	TargetBranch  string
	CustomMessage string
}

var (
	gerritCommentTmpl = template.Must(template.New("gerritComment").Parse(gerritCommentTxt))
)

// startPoller does the following:
// 1) Loop through all supported branch dependencies in the config file.
// 2) For each config:
//   * Query gerrit for open cherrypicks in the source repo+branch.
//   * For each open cherrypick:
//     * Check in-memory cache to see if we already processed this change. If
//       in cache, continue with the next cherrypick in step 2 above.
//     * Check DB to see if we already processed this change. If in DB, then
//       add to the cache and continue with the next cherrypick in step 2 above.
//     * If cherrypick is not in cache or DB:
//       * Check to see if the change is a cherrypick and if the cherrypick
//         already exists in the target repo+branch.
//       * If change is not a cherrypick or cherrypick does not exist in the
//         target repo+branch:
//         * Add a reminder comment to the cherrypick.
//       * Mark the cherrypick as processed in the in-memory cache and in the DB.
func startPoller(ctx context.Context, gerritClient *gerrit.Gerrit, dbClient *db.FirestoreDB, branchDeps []*config.SupportedBranchDep) {
	liveness := metrics2.NewLiveness("cherrypick_watcher")
	util.RepeatCtx(ctx, *pollInterval, func(ctx context.Context) {
		pollerMtx.Lock()
		defer pollerMtx.Unlock()
		sklog.Infof("--------- new round of polling --------------")

		// Ignore the passed-in context; this allows us to continue running even if the
		// context is canceled due to transient errors.
		ctx = context.Background()

		// Loop through all supported branch dependencies in the config file.
		for _, bd := range branchDeps {
			sklog.Infof("Starting processing changes in source repo %s and branch %s for target repo %s and branch %s", bd.SourceRepo, bd.SourceBranch, bd.TargetRepo, bd.TargetBranch)

			// Query gerrit for open cherrypicks in the source repo+branch.
			sourceCherrypicks, err := cp_gerrit.FindAllOpenCherrypicks(ctx, gerritClient, bd.SourceRepo, bd.SourceBranch)
			if err != nil {
				sklog.Errorf("Could not query gerrit for repo %s and branch %s: %s", bd.SourceRepo, bd.SourceBranch, err)
				continue
			}

			for _, c := range sourceCherrypicks {
				sklog.Infof("--Processing %d from %s %s--", c.Issue, bd.SourceRepo, bd.SourceBranch)

				// Get unique key for this change and the branch dependency.
				key := db.GetKey(bd.SourceRepo, bd.SourceBranch, bd.TargetRepo, bd.TargetBranch, c.Issue)

				// Check in-memory cache to see if we already processed this change.
				if _, ok := processedCache[key]; ok {
					sklog.Infof("Found change %d in cache. We have already processed it before. Continuing.", c.Issue)
					continue
				}
				sklog.Infof("Did not find change %d in cache.", c.Issue)

				// Check DB to see if we already processed this change.
				data, err := dbClient.GetFromDB(ctx, key)
				if err != nil {
					sklog.Errorf("Could not access DB for %d: %s", c.Issue, err)
					continue
				}
				if data != nil {
					// Add to the cache so that we do not have to hit the DB again.
					processedCache[key] = true
					sklog.Infof("Found change %d in DB. Added it to the cache. We have already processed the change before. Continuing.", c.Issue)
					continue
				}
				sklog.Infof("Did not find change %d in DB.", c.Issue)

				if c.CherrypickOfChange == 0 {
					// Change was not created via the Gerrit UI.
					sklog.Infof("Change %d is not a cherrypick created from the Gerrit UI.", c.Issue)
				} else {
					// Check to see if the cherrypick already exists in the target repo+branch.
					cherrypickInTargetBranch, err := cp_gerrit.IsCherrypickIn(ctx, gerritClient, bd.TargetRepo, bd.TargetBranch, c.CherrypickOfChange)
					if err != nil {
						sklog.Errorf("Error checking for cherrypick %d in %s %s", c.CherrypickOfChange, bd.TargetRepo, bd.TargetBranch)
						continue
					}
					if cherrypickInTargetBranch {
						// The cherrypick was already created in the target repo+branch.
						sklog.Infof("Cherrypick %d already exists in %s %s. Not going to add reminder comment to %d.", c.CherrypickOfChange, bd.TargetRepo, bd.TargetBranch, c.Issue)
						// Mark the cherrypick as processed in the in-memory cache and in the DB.
						processedCache[key] = true
						if err := dbClient.PutInDB(ctx, key, c.Issue); err != nil {
							sklog.Errorf("Could not mark %d as processed in DB: %s", c.Issue, err)
							continue
						}
						sklog.Infof("Marked the change %d as processed in the cache and the DB", c.Issue)
						continue
					}
					sklog.Infof("Cherrypick %d does not exists in the target %s %s.", c.CherrypickOfChange, bd.TargetRepo, bd.TargetBranch)
				}

				// Add a reminder comment to the cherrypick.
				vars := gerritCommentVars{
					SourceBranch:  bd.SourceBranch,
					TargetBranch:  bd.TargetBranch,
					CustomMessage: bd.CustomMessage,
				}
				var gerritCommentBytes bytes.Buffer
				if err := gerritCommentTmpl.Execute(&gerritCommentBytes, vars); err != nil {
					sklog.Errorf("Failed to execute prComment template: %s", err)
					continue
				}
				if err := cp_gerrit.AddReminderComment(ctx, gerritClient, c, gerritCommentBytes.String()); err != nil {
					sklog.Errorf("Could not add a comment to %d: %s", c.Issue, err)
					continue
				}
				sklog.Infof("Successfully added reminder comment to %d.", c.Issue)

				// Mark the cherrypick as processed in the in-memory cache and in the DB.
				processedCache[key] = true
				if err := dbClient.PutInDB(ctx, key, c.Issue); err != nil {
					sklog.Errorf("Could not mark %d as processed in DB: %s", c.Issue, err)
					continue
				}
				sklog.Infof("Marked the change %d as processed in the cache and the DB", c.Issue)
			}
		}

		liveness.Reset()
		sklog.Info("---------------------------------------------")
	})
}

func main() {
	common.InitWithMust("cherrypick-watcher", common.PrometheusOpt(promPort), common.MetricsLoggingOpt())

	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Instantiate DB.
	dbClient, err := db.New(ctx, ts, *fsNamespace, *fsProjectID)
	if err != nil {
		sklog.Fatalf("Could not init DB: %s", err)
	}

	// Instantiate gerrit client.
	gerritClient, err := gerrit.NewGerrit(gerrit.GerritSkiaURL, httpClient)
	if err != nil {
		sklog.Fatalf("Failed to create Gerrit client: %s", err)
	}

	// Read the config.
	cfgContents, err := fs.ReadFile(config.Configs, *configFile)
	if err != nil {
		sklog.Fatalf("Could not read the config file %s: %s", *configFile, err)
	}
	supportedBranchDeps, err := config.ParseCfg(cfgContents)
	if err != nil {
		sklog.Fatalf("Failed to read the config file at %s: %s", *configFile, err)
	}

	// Initialize the cache.
	processedCache = map[string]interface{}{}

	startPoller(ctx, gerritClient, dbClient, supportedBranchDeps)
}
