package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	gstorage "cloud.google.com/go/storage"
	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/commenter"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/code_review/github_crs"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/ignore/sqlignorestore"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tracing"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/perf/go/ingest/format"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// Arbitrary number
	maxSQLConnections = 4

	clScanRange = 5 * 24 * time.Hour
)

// TODO(kjlubick) Add a task to check for abandoned CLs.

type periodicTasksConfig struct {
	config.Common

	// ChangelistDiffPeriod is how often to look at recently updated CLs and tabulate the diffs
	// for the digests produced.
	// The diffs are not calculated in this service, but the tasks are generated here and
	// processed in the diffcalculator process.
	ChangelistDiffPeriod config.Duration `json:"changelist_diff_period"`

	// CLCommentTemplate is a string with placeholders for generating a comment message. See
	// commenter.commentTemplateContext for the exact fields.
	CLCommentTemplate string `json:"cl_comment_template" optional:"true"`

	// CommentOnCLsPeriod, if positive, is how often to check recent CLs and Patchsets for
	// untriaged digests and comment on them if appropriate.
	CommentOnCLsPeriod config.Duration `json:"comment_on_cls_period" optional:"true"`

	// PerfSummaries configures summary data (e.g. triage status, ignore count) that is fed into
	// a GCS bucket which an instance of Perf can ingest from.
	PerfSummaries *perfSummariesConfig `json:"perf_summaries" optional:"true"`

	// PrimaryBranchDiffPeriod is how often to look at the most recent window of commits and
	// tabulate diffs between all groupings based on the digests produced on the primary branch.
	// The diffs are not calculated in this service, but sent via Pub/Sub to the appropriate workers.
	PrimaryBranchDiffPeriod config.Duration `json:"primary_branch_diff_period"`

	// UpdateIgnorePeriod is how often we should try to apply the ignore rules to all traces.
	UpdateIgnorePeriod config.Duration `json:"update_traces_ignore_period"` // TODO(kjlubick) change JSON
}

type perfSummariesConfig struct {
	AgeOutCommits      int             `json:"age_out_commits"`
	CorporaToSummarize []string        `json:"corpora_to_summarize"`
	GCSBucket          string          `json:"perf_gcs_bucket"`
	KeysToSummarize    []string        `json:"keys_to_summarize"`
	Period             config.Duration `json:"period"`
	ValuesToIgnore     []string        `json:"values_to_ignore"`
}

func main() {
	// Command line flags.
	var (
		commonInstanceConfig = flag.String("common_instance_config", "", "Path to the json5 file containing the configuration that needs to be the same across all services for a given instance.")
		thisConfig           = flag.String("config", "", "Path to the json5 file containing the configuration specific to the periodic tasks server.")
		hang                 = flag.Bool("hang", false, "Stop and do nothing after reading the flags. Good for debugging containers.")
	)

	// Parse the options. So we can configure logging.
	flag.Parse()

	if *hang {
		sklog.Info("Hanging")
		select {}
	}

	var ptc periodicTasksConfig
	if err := config.LoadFromJSON5(&ptc, commonInstanceConfig, thisConfig); err != nil {
		sklog.Fatalf("Reading config: %s", err)
	}
	sklog.Infof("Loaded config %#v", ptc)

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(&ptc.PromPort),
	}

	tp := 0.01
	if ptc.TracingProportion > 0 {
		tp = ptc.TracingProportion
	}
	common.InitWithMust("periodictasks", logOpts...)
	if err := tracing.Initialize(tp, ptc.SQLDatabaseName); err != nil {
		sklog.Fatalf("Could not set up tracing: %s", err)
	}

	ctx := context.Background()
	db := mustInitSQLDatabase(ctx, ptc)

	startUpdateTracesIgnoreStatus(ctx, db, ptc)

	startCommentOnCLs(ctx, db, ptc)

	gatherer := &diffWorkGatherer{
		db:               db,
		windowSize:       ptc.WindowSize,
		mostRecentCLScan: now.Now(ctx).Add(-clScanRange),
	}
	startPrimaryBranchDiffWork(ctx, gatherer, ptc)
	startChangelistsDiffWork(ctx, gatherer, ptc)
	startDiffWorkMetrics(ctx, db)
	startBackupStatusCheck(ctx, db, ptc)
	startKnownDigestsSync(ctx, db, ptc)
	if ptc.PerfSummaries != nil {
		startPerfSummarization(ctx, db, ptc.PerfSummaries)
	}

	sklog.Infof("periodic tasks have been started")
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	sklog.Fatal(http.ListenAndServe(ptc.ReadyPort, nil))
}

func mustInitSQLDatabase(ctx context.Context, ptc periodicTasksConfig) *pgxpool.Pool {
	if ptc.SQLDatabaseName == "" {
		sklog.Fatalf("Must have SQL Database Information")
	}
	url := sql.GetConnectionURL(ptc.SQLConnection, ptc.SQLDatabaseName)
	conf, err := pgxpool.ParseConfig(url)
	if err != nil {
		sklog.Fatalf("error getting postgres config %s: %s", url, err)
	}

	conf.MaxConns = maxSQLConnections
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	sklog.Infof("Connected to SQL database %s", ptc.SQLDatabaseName)
	return db
}

func startUpdateTracesIgnoreStatus(ctx context.Context, db *pgxpool.Pool, ptc periodicTasksConfig) {
	liveness := metrics2.NewLiveness("periodic_tasks", map[string]string{
		"task": "updateTracesIgnoreStatus",
	})
	go util.RepeatCtx(ctx, ptc.UpdateIgnorePeriod.Duration, func(ctx context.Context) {
		sklog.Infof("Updating traces and values at head with ignore status")
		ctx, span := trace.StartSpan(ctx, "periodic_updateTracesIgnoreStatus")
		defer span.End()
		if err := sqlignorestore.UpdateIgnoredTraces(ctx, db); err != nil {
			sklog.Errorf("Error while updating traces ignore status: %s", err)
			return // return so the liveness is not updated
		}
		liveness.Reset()
		sklog.Infof("Done with updateTracesIgnoreStatus")
	})
}

func startCommentOnCLs(ctx context.Context, db *pgxpool.Pool, ptc periodicTasksConfig) {
	if ptc.CommentOnCLsPeriod.Duration <= 0 {
		sklog.Infof("Not commenting on CLs because duration was zero.")
		return
	}
	systems := mustInitializeSystems(ctx, ptc)
	cmntr, err := commenter.New(db, systems, ptc.CLCommentTemplate, ptc.SiteURL, ptc.WindowSize)
	if err != nil {
		sklog.Fatalf("Could not initialize commenting: %s", err)
	}
	liveness := metrics2.NewLiveness("periodic_tasks", map[string]string{
		"task": "commentOnCLs",
	})
	go util.RepeatCtx(ctx, ptc.CommentOnCLsPeriod.Duration, func(ctx context.Context) {
		sklog.Infof("Checking CLs for untriaged results and commenting if necessary")
		ctx, span := trace.StartSpan(ctx, "periodic_commentOnCLsWithUntriagedDigests")
		defer span.End()
		if err := cmntr.CommentOnChangelistsWithUntriagedDigests(ctx); err != nil {
			sklog.Errorf("Error while commenting on CLs: %s", err)
			return // return so the liveness is not updated
		}
		liveness.Reset()
		sklog.Infof("Done checking on CLs to comment")
	})
}

// mustInitializeSystems creates code_review.Clients and returns them wrapped as a ReviewSystem.
// It panics if any part of configuration fails.
func mustInitializeSystems(ctx context.Context, ptc periodicTasksConfig) []commenter.ReviewSystem {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeGerrit)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}
	gerritHTTPClient := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	rv := make([]commenter.ReviewSystem, 0, len(ptc.CodeReviewSystems))
	for _, cfg := range ptc.CodeReviewSystems {
		var crs code_review.Client
		if cfg.Flavor == "gerrit" {
			if cfg.GerritURL == "" {
				sklog.Fatal("You must specify gerrit_url")
			}
			gerritClient, err := gerrit.NewGerrit(cfg.GerritURL, gerritHTTPClient)
			if err != nil {
				sklog.Fatalf("Could not create gerrit client for %s", cfg.GerritURL)
			}
			crs = gerrit_crs.New(gerritClient)
		} else if cfg.Flavor == "github" {
			if cfg.GitHubRepo == "" || cfg.GitHubCredPath == "" {
				sklog.Fatal("You must specify github_repo and github_cred_path")
			}
			gBody, err := ioutil.ReadFile(cfg.GitHubCredPath)
			if err != nil {
				sklog.Fatalf("Couldn't find githubToken in %s: %s", cfg.GitHubCredPath, err)
			}
			gToken := strings.TrimSpace(string(gBody))
			githubTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken})
			c := httputils.DefaultClientConfig().With2xxOnly().WithTokenSource(githubTS).Client()
			crs = github_crs.New(c, cfg.GitHubRepo)
		} else {
			sklog.Fatalf("CRS flavor %s not supported.", cfg.Flavor)
		}
		rv = append(rv, commenter.ReviewSystem{
			ID:     cfg.ID,
			Client: crs,
		})
	}
	return rv
}

type diffWorkGatherer struct {
	db         *pgxpool.Pool
	windowSize int

	mostRecentCLScan time.Time
}

// startPrimaryBranchDiffWork starts the process that periodically creates rows in the SQL DB for
// diff workers to calculate diffs for images on the primary branch.
func startPrimaryBranchDiffWork(ctx context.Context, gatherer *diffWorkGatherer, ptc periodicTasksConfig) {
	liveness := metrics2.NewLiveness("periodic_tasks", map[string]string{
		"task": "calculatePrimaryBranchDiffWork",
	})
	go util.RepeatCtx(ctx, ptc.PrimaryBranchDiffPeriod.Duration, func(ctx context.Context) {
		sklog.Infof("Calculating diffs for images seen recently")
		ctx, span := trace.StartSpan(ctx, "periodic_PrimaryBranchDiffWork")
		defer span.End()
		if err := gatherer.gatherFromPrimaryBranch(ctx); err != nil {
			sklog.Errorf("Error while gathering diff work on primary branch: %s", err)
			return // return so the liveness is not updated
		}
		liveness.Reset()
		sklog.Infof("Done with sending diffs from primary branch")
	})
}

// gatherFromPrimaryBranch finds all groupings that have recent data on the primary branch and
// makes sure a row exists in the SQL DB for each of them.
func (g *diffWorkGatherer) gatherFromPrimaryBranch(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "gatherFromPrimaryBranch")
	defer span.End()
	// Doing the get/join/insert all in 1 transaction did not work when there are many groupings
	// and many diffcalculator processes - too much contention.
	groupingsInWindow, err := g.getDistinctGroupingsInWindow(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	alreadyProcessedGroupings, err := g.getGroupingsBeingProcessed(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	apg := map[string]bool{}
	for _, g := range alreadyProcessedGroupings {
		apg[string(g)] = true
	}
	// TODO(kjlubick) periodically remove groupings that are not at HEAD anymore.
	var newGroupings []schema.GroupingID
	for _, g := range groupingsInWindow {
		if !apg[string(g)] {
			newGroupings = append(newGroupings, g)
		}
	}
	sklog.Infof("There are currently %d groupings in the window and %d groupings being processed for diffs. This cycle, there were %d new groupings detected.",
		len(groupingsInWindow), len(alreadyProcessedGroupings), len(newGroupings))

	if len(newGroupings) == 0 {
		return nil
	}

	return skerr.Wrap(g.addNewGroupingsForProcessing(ctx, newGroupings))
}

// getDistinctGroupingsInWindow returns the distinct grouping ids seen within the current window.
func (g *diffWorkGatherer) getDistinctGroupingsInWindow(ctx context.Context) ([]schema.GroupingID, error) {
	ctx, span := trace.StartSpan(ctx, "getDistinctGroupingsInWindow")
	defer span.End()

	const statement = `WITH
RecentCommits AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
),
FirstCommitInWindow AS (
	SELECT commit_id FROM RecentCommits
	ORDER BY commit_id ASC LIMIT 1
)
SELECT DISTINCT grouping_id FROM ValuesAtHead
JOIN FirstCommitInWindow ON ValuesAtHead.most_recent_commit_id >= FirstCommitInWindow.commit_id`

	rows, err := g.db.Query(ctx, statement, g.windowSize)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []schema.GroupingID
	for rows.Next() {
		var id schema.GroupingID
		if err := rows.Scan(&id); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, id)
	}
	return rv, nil
}

// getGroupingsBeingProcessed returns all groupings that we are currently computing diffs for.
func (g *diffWorkGatherer) getGroupingsBeingProcessed(ctx context.Context) ([]schema.GroupingID, error) {
	ctx, span := trace.StartSpan(ctx, "getGroupingsBeingProcessed")
	defer span.End()

	const statement = `SELECT DISTINCT grouping_id FROM PrimaryBranchDiffCalculationWork
AS OF SYSTEM TIME '-0.1s'`

	rows, err := g.db.Query(ctx, statement)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []schema.GroupingID
	for rows.Next() {
		var id schema.GroupingID
		if err := rows.Scan(&id); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, id)
	}
	return rv, nil
}

// addNewGroupingsForProcessing updates the PrimaryBranchDiffCalculationWork table with the newly
// provided groupings, such that we will start computing diffs for them. This table is potentially
// under a lot of contention. We try to write some sentinel values, but if there are already values
// there, we will bail out.
func (g *diffWorkGatherer) addNewGroupingsForProcessing(ctx context.Context, groupings []schema.GroupingID) error {
	ctx, span := trace.StartSpan(ctx, "addNewGroupingsForProcessing")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("num_groupings", int64(len(groupings))))
	statement := `INSERT INTO PrimaryBranchDiffCalculationWork (grouping_id, last_calculated_ts, calculation_lease_ends) VALUES`
	const valuesPerRow = 3
	vp := sql.ValuesPlaceholders(valuesPerRow, len(groupings))
	statement = statement + vp + ` ON CONFLICT DO NOTHING`
	args := make([]interface{}, 0, valuesPerRow*len(groupings))
	// This time will make sure we compute diffs for this soon.
	beginningOfTime := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, g := range groupings {
		args = append(args, g, beginningOfTime, beginningOfTime)
	}

	err := crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := g.db.Exec(ctx, statement, args...)
		return err // may be retried
	})
	return skerr.Wrap(err)
}

// startChangelistsDiffWork starts the process that periodically creates rows in the SQL DB for
///diff workers to calculate diffs for images produced by CLs.
func startChangelistsDiffWork(ctx context.Context, gatherer *diffWorkGatherer, ptc periodicTasksConfig) {
	liveness := metrics2.NewLiveness("periodic_tasks", map[string]string{
		"task": "calculateChangelistsDiffWork",
	})
	go util.RepeatCtx(ctx, ptc.PrimaryBranchDiffPeriod.Duration, func(ctx context.Context) {
		sklog.Infof("Calculating diffs for images produced on CLs")
		ctx, span := trace.StartSpan(ctx, "periodic_ChangelistsDiffWork")
		defer span.End()
		if err := gatherer.gatherFromChangelists(ctx); err != nil {
			sklog.Errorf("Error while gathering diff work on CLs: %s", err)
			return // return so the liveness is not updated
		}
		liveness.Reset()
		sklog.Infof("Done with sending diffs from CLs")
	})
}

// gatherFromChangelists scans all recently updated CLs and creates a row in the SQL DB for each
// grouping and CL that saw images not already on the primary branch.
func (g *diffWorkGatherer) gatherFromChangelists(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "gatherFromChangelists")
	defer span.End()
	firstTileIDInWindow, err := g.getFirstTileIDInWindow(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	updatedTS := now.Now(ctx)
	// Check all changelists updated since last cycle (or 5 days, initially).
	clIDs, err := g.getRecentlyUpdatedChangelists(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	span.AddAttributes(trace.Int64Attribute("num_cls", int64(len(clIDs))))
	if len(clIDs) != 0 {
		for _, cl := range clIDs {
			sklog.Debugf("Creating diff work for CL %s", cl)
			if err := g.createDiffRowsForCL(ctx, firstTileIDInWindow, cl); err != nil {
				return skerr.Wrap(err)
			}
		}
	}
	g.mostRecentCLScan = updatedTS
	if err := g.deleteOldCLDiffWork(ctx); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// getFirstTileIDInWindow returns the first tile in the current sliding window of commits.
func (g *diffWorkGatherer) getFirstTileIDInWindow(ctx context.Context) (schema.TileID, error) {
	ctx, span := trace.StartSpan(ctx, "getFirstTileIDInWindow")
	defer span.End()
	const statement = `WITH
RecentCommits AS (
	SELECT tile_id, commit_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
)
SELECT tile_id FROM RecentCommits ORDER BY commit_id ASC LIMIT 1
`
	row := g.db.QueryRow(ctx, statement, g.windowSize)
	var id schema.TileID
	if err := row.Scan(&id); err != nil {
		return -1, skerr.Wrap(err)
	}
	return id, nil
}

// getRecentlyUpdatedChangelists returns the qualified IDs of all CLs that were updated after
// the most recent CL scan.
func (g *diffWorkGatherer) getRecentlyUpdatedChangelists(ctx context.Context) ([]string, error) {
	ctx, span := trace.StartSpan(ctx, "getRecentlyUpdatedChangelists")
	defer span.End()
	const statement = `
SELECT changelist_id FROM Changelists WHERE last_ingested_data >= $1
`
	rows, err := g.db.Query(ctx, statement, g.mostRecentCLScan)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, skerr.Wrap(err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// createDiffRowsForCL finds all data produced by this CL and compares it against the data on the
// primary branch. If any digest is not already on the primary branch (in the sliding window), it
// will be gathered up into one row in the SQL DB for diff calculation
func (g *diffWorkGatherer) createDiffRowsForCL(ctx context.Context, startingTile schema.TileID, cl string) error {
	ctx, span := trace.StartSpan(ctx, "createDiffRowsForCL")
	defer span.End()
	span.AddAttributes(trace.StringAttribute("CL", cl))
	row := g.db.QueryRow(ctx, `SELECT last_ingested_data FROM ChangeLists WHERE changelist_id = $1`, cl)
	var updatedTS time.Time
	if err := row.Scan(&updatedTS); err != nil {
		return skerr.Wrap(err)
	}
	updatedTS = updatedTS.UTC()

	const statement = `WITH
	DataForCL AS (
		SELECT DISTINCT grouping_id, digest FROM SecondaryBranchValues
		WHERE branch_name = $1
	),
	DigestsNotOnPrimaryBranch AS (
		-- We do a left join and check for null to pull only those digests that are not already
		-- on the primary branch. Those new digests we have to include in our diff calculations.
		SELECT DISTINCT DataForCL.grouping_id, DataForCL.digest FROM DataForCL
		LEFT JOIN TiledTraceDigests ON DataForCL.grouping_id = TiledTraceDigests.grouping_id
			AND DataForCL.digest = TiledTraceDigests.digest
			AND TiledTraceDigests.tile_id >= $2
		WHERE TiledTraceDigests.digest IS NULL
	)
	SELECT Groupings.grouping_id, encode(DigestsNotOnPrimaryBranch.digest, 'hex') FROM DigestsNotOnPrimaryBranch
	JOIN Groupings ON DigestsNotOnPrimaryBranch.grouping_id = Groupings.grouping_id
	ORDER BY 1, 2
	`
	rows, err := g.db.Query(ctx, statement, cl, startingTile)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer rows.Close()
	var newDigests []types.Digest
	var currentGroupingID schema.GroupingID
	var workRows []schema.SecondaryBranchDiffCalculationRow
	for rows.Next() {
		var digest types.Digest
		var groupingID schema.GroupingID
		if err := rows.Scan(&groupingID, &digest); err != nil {
			return skerr.Wrap(err)
		}
		if currentGroupingID == nil {
			currentGroupingID = groupingID
		}
		if bytes.Equal(currentGroupingID, groupingID) {
			newDigests = append(newDigests, digest)
		} else {
			workRows = append(workRows, schema.SecondaryBranchDiffCalculationRow{
				BranchName:          cl,
				GroupingID:          currentGroupingID,
				LastUpdated:         updatedTS,
				DigestsNotOnPrimary: newDigests,
			})
			currentGroupingID = groupingID
			// Reset newDigests to be empty and then start adding the new digests to it.
			newDigests = []types.Digest{digest}
		}
	}
	rows.Close()
	if currentGroupingID == nil {
		return nil // nothing to report
	}
	workRows = append(workRows, schema.SecondaryBranchDiffCalculationRow{
		BranchName:          cl,
		GroupingID:          currentGroupingID,
		LastUpdated:         updatedTS,
		DigestsNotOnPrimary: newDigests,
	})

	insertStatement := `INSERT INTO SecondaryBranchDiffCalculationWork
(branch_name, grouping_id, last_updated_ts, digests, last_calculated_ts, calculation_lease_ends) VALUES `
	const valuesPerRow = 6
	vp := sql.ValuesPlaceholders(valuesPerRow, len(workRows))
	insertStatement += vp
	insertStatement += ` ON CONFLICT (branch_name, grouping_id)
DO UPDATE SET (last_updated_ts, digests) = (excluded.last_updated_ts, excluded.digests);`

	arguments := make([]interface{}, 0, valuesPerRow*len(workRows))
	epoch := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, wr := range workRows {
		arguments = append(arguments, wr.BranchName, wr.GroupingID, wr.LastUpdated, wr.DigestsNotOnPrimary,
			epoch, epoch)
	}
	err = crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := g.db.Exec(ctx, insertStatement, arguments...)
		return err // may be retried
	})
	return skerr.Wrap(err)
}

// deleteOldCLDiffWork deletes rows in the SQL DB that are "too old", that is, they belong to CLs
// that are several days old, beyond the clScanRange. This helps keep the number of rows down
// in that Table.
func (g *diffWorkGatherer) deleteOldCLDiffWork(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "createDiffRowsForCL")
	defer span.End()

	const statement = `DELETE FROM SecondaryBranchDiffCalculationWork WHERE last_updated_ts < $1`
	err := crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		cutoff := now.Now(ctx).Add(-clScanRange)
		_, err := g.db.Exec(ctx, statement, cutoff)
		return err // may be retried
	})
	return skerr.Wrap(err)
}

// startDiffWorkMetrics continuously reports how much uncalculated work there is
func startDiffWorkMetrics(ctx context.Context, db *pgxpool.Pool) {
	go func() {
		const queueSize = "diffcalculator_workqueuesize"
		const queueFreshness = "diffcalculator_workqueuefreshness"

		for range time.Tick(time.Minute) {
			if err := ctx.Err(); err != nil {
				return
			}
			// These time values come from diffcalculator
			const primarySizeStatement = `SELECT COUNT(*) FROM PrimaryBranchDiffCalculationWork
WHERE (now() - last_calculated_ts) > '1m' AND (now() - calculation_lease_ends) > '10m'`
			row := db.QueryRow(ctx, primarySizeStatement)
			var primarySize int64
			if err := row.Scan(&primarySize); err != nil {
				sklog.Warningf("Could not compute queue size for primary branch: %s", err)
				primarySize = -1
			}
			metrics2.GetInt64Metric(queueSize, map[string]string{"branch": "primary"}).Update(primarySize)

			const secondaryBranchStatement = `SELECT COUNT(*) FROM SecondaryBranchDiffCalculationWork
WHERE last_updated_ts > last_calculated_ts AND (now() - calculation_lease_ends) > '10m'`
			row = db.QueryRow(ctx, secondaryBranchStatement)
			var secondarySize int64
			if err := row.Scan(&secondarySize); err != nil {
				sklog.Warningf("Could not compute queue size for secondary branch: %s", err)
				secondarySize = -1
			}
			metrics2.GetInt64Metric(queueSize, map[string]string{"branch": "secondary"}).Update(secondarySize)

			const primaryFreshnessStatement = `SELECT AVG(now() - last_calculated_ts), MAX(now() - last_calculated_ts)
FROM PrimaryBranchDiffCalculationWork`
			row = db.QueryRow(ctx, primaryFreshnessStatement)
			var primaryAvgFreshness time.Duration
			var primaryMaxFreshness time.Duration
			if err := row.Scan(&primaryAvgFreshness, &primaryMaxFreshness); err != nil {
				sklog.Warningf("Could not compute diffwork freshness for primary branch: %s", err)
				primaryAvgFreshness = -1
				primaryMaxFreshness = -1
			}
			metrics2.GetInt64Metric(queueFreshness, map[string]string{"branch": "primary", "stat": "avg"}).
				Update(int64(primaryAvgFreshness / time.Second))
			metrics2.GetInt64Metric(queueFreshness, map[string]string{"branch": "primary", "stat": "max"}).
				Update(int64(primaryMaxFreshness / time.Second))

		}
	}()
}

// startBackupStatusCheck repeatedly scans the Backup Schedules to see their status. If there are
// not 3 schedules, each with a success, this raises an error. The schedules are created via
// //golden/cmd/sqlinit. If the tables change, those will need to be re-created.
// https://www.cockroachlabs.com/docs/stable/create-schedule-for-backup.html
func startBackupStatusCheck(ctx context.Context, db *pgxpool.Pool, ptc periodicTasksConfig) {
	go func() {
		const backupError = "periodictasks_backup_error"
		backupMetric := metrics2.GetInt64Metric(backupError, map[string]string{"database": ptc.SQLDatabaseName})
		backupMetric.Update(0)

		for range time.Tick(time.Hour) {
			if err := ctx.Err(); err != nil {
				return
			}
			statement := `SELECT id, label, state FROM [SHOW SCHEDULES] WHERE label LIKE '` +
				ptc.SQLDatabaseName + `\_%'`
			rows, err := db.Query(ctx, statement)
			if err != nil {
				sklog.Errorf("Could not check backup schedules: %s", err)
				backupMetric.Update(1)
				return
			}

			hadFailure := false
			totalBackups := 0
			for rows.Next() {
				var id int64
				var label string
				var state pgtype.Text
				if err := rows.Scan(&id, &label, &state); err != nil {
					sklog.Errorf("Could not scan backup results: %s", err)
					backupMetric.Update(1)
					return
				}
				totalBackups++
				// Example errors:
				// reschedule: failed to create job for schedule 623934079889145857: err=executing schedule 623934079889145857: failed to resolve targets specified in the BACKUP stmt: table "crostastdev.commits" does not exist
				// reschedule: failed to create job for schedule 623934084168056833: err=executing schedule 623934084168056833: Get https://storage.googleapis.com/skia-gold-sql-backups/crostastdev/monthly/2021/09/07-042100.00/BACKUP_MANIFEST: oauth2: cannot fetch token: 400 Bad Request
				if strings.Contains(state.String, "reschedule") ||
					strings.Contains(state.String, "failed") ||
					strings.Contains(state.String, "err") {
					hadFailure = true
					sklog.Errorf("Backup Error for %s (%d) - %s", label, id, state.String)
				}
			}
			rows.Close()
			if totalBackups != 3 {
				sklog.Errorf("Expected to see 3 backup schedules (daily, weekly, monthly), but instead saw %d", hadFailure)
				hadFailure = true
			}
			if hadFailure {
				backupMetric.Update(1)
			} else {
				backupMetric.Update(0)
				sklog.Infof("All backups are performing as expected")
			}
		}
	}()
}

// startKnownDigestsSync regularly syncs all the known digests to the KnownHashesGCSPath, which
// can be used by clients (we know it is used by Skia) to make tests not have to decoded and output
// images that match the given hash. This optimization becomes important when tests are putting out
// many many images.
func startKnownDigestsSync(ctx context.Context, db *pgxpool.Pool, ptc periodicTasksConfig) {
	liveness := metrics2.NewLiveness("periodic_tasks", map[string]string{
		"task": "syncKnownDigests",
	})

	storageClient, err := storage.NewGCSClient(ctx, nil, storage.GCSClientOptions{
		Bucket:             ptc.GCSBucket,
		KnownHashesGCSPath: ptc.KnownHashesGCSPath,
	})
	if err != nil {
		sklog.Errorf("Could not start syncing known digests: %s", err)
		return
	}

	go util.RepeatCtx(ctx, 20*time.Minute, func(ctx context.Context) {
		sklog.Infof("Syncing all recently seen digests to %s", ptc.KnownHashesGCSPath)
		ctx, span := trace.StartSpan(ctx, "periodic_SyncKnownDigests")
		defer span.End()

		// We grab digests from twice our window length to be overly thorough to avoid excess
		// uploads from clients who use this.
		digests, err := getAllRecentDigests(ctx, db, ptc.WindowSize*2)
		if err != nil {
			sklog.Errorf("Error getting recent digests: %s", err)
			return
		}

		if err := storageClient.WriteKnownDigests(ctx, digests); err != nil {
			sklog.Errorf("Error writing recent digests: %s", err)
			return
		}
		liveness.Reset()
		sklog.Infof("Done syncing recently seen digests")
	})
}

// getAllRecentDigests returns all the digests seen on the primary branch in the provided window
// of commits. If needed, this could combine the digests with the unique digests seen from recent
// Tryjob results.
func getAllRecentDigests(ctx context.Context, db *pgxpool.Pool, numCommits int) ([]types.Digest, error) {
	ctx, span := trace.StartSpan(ctx, "getAllRecentDigests")
	defer span.End()

	const statement = `
WITH
RecentCommits AS (
	SELECT tile_id, commit_id FROM CommitsWithData
	AS OF SYSTEM TIME '-0.1s'
	ORDER BY commit_id DESC LIMIT $1
),
OldestTileInWindow AS (
	SELECT MIN(tile_id) as tile_id FROM RecentCommits
	AS OF SYSTEM TIME '-0.1s'
)
SELECT DISTINCT encode(digest, 'hex') FROM TiledTraceDigests
JOIN OldestTileInWindow ON TiledTraceDigests.tile_id >= OldestTileInWindow.tile_id
AS OF SYSTEM TIME '-0.1s'
ORDER BY 1
`

	rows, err := db.Query(ctx, statement, numCommits)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()

	var rv []types.Digest
	for rows.Next() {
		var d types.Digest
		if err := rows.Scan(&d); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, d)
	}
	return rv, nil
}

// startPerfSummarization starts the process that will summarize gold traces and upload them to
// Perf. It assumes the config is non-nil, and will panic if the minimally set data is not done so.
// It starts a go routine that will immediately being summarizing and then repeat the process at
// the configured time period.
func startPerfSummarization(ctx context.Context, db *pgxpool.Pool, sCfg *perfSummariesConfig) {
	sklog.Infof("Perf summary config %+v", *sCfg)
	if sCfg.AgeOutCommits <= 0 {
		panic("Must have a positive, non-zero age_out_commits")
	}
	if len(sCfg.KeysToSummarize) == 0 {
		panic("Must specify at least one key")
	}
	if len(sCfg.CorporaToSummarize) == 0 {
		panic("Must specify at least one corpus")
	}
	liveness := metrics2.NewLiveness("periodic_tasks", map[string]string{
		"task": "PerfSummarization",
	})

	sc, err := gstorage.NewClient(ctx)
	if err != nil {
		panic("Could not make google storage client " + err.Error())
	}
	storageClient := gcsclient.New(sc, sCfg.GCSBucket)

	go util.RepeatCtx(ctx, sCfg.Period.Duration, func(ctx context.Context) {
		sklog.Infof("Tabulating all perf data for keys %s and corpora %s", sCfg.KeysToSummarize, sCfg.CorporaToSummarize)

		if err := summarizeTraces(ctx, db, sCfg, storageClient); err != nil {
			sklog.Errorf("Could not summarize traces using config %+v: %s", sCfg, err)
			return
		}
		liveness.Reset()
		sklog.Infof("Done tabulating all perf data")
	})
}

// summarizeTraces loops through all tuples of keys that match the given configuration (and do not
// include any ignored values), counting how many traces are triaged to one of the three states and
// how many are ignored. This data is uploaded to Perf's GCS bucket in a streaming fashion, that is
// each tuple's data is uploaded on its own, not as one big blob. The entire process could take
// a while, as the summarization may involve full table scans.
func summarizeTraces(ctx context.Context, db *pgxpool.Pool, cfg *perfSummariesConfig, client gcs.GCSClient) error {
	oldestCommitID, latestCommitID, err := getWindowCommitBounds(ctx, db, cfg.AgeOutCommits)
	if err != nil {
		return skerr.Wrap(err)
	}

	tuples, err := getTuplesOfKeysToQuery(ctx, db, cfg.KeysToSummarize, cfg.ValuesToIgnore, cfg.CorporaToSummarize)
	if err != nil {
		return skerr.Wrap(err)
	}

	idToGitHash := map[schema.CommitID]string{}

	for _, tuple := range tuples {
		perfData, err := getTriageStatus(ctx, db, tuple, oldestCommitID)
		if err != nil {
			return skerr.Wrap(err)
		}
		perfData.IgnoredTraces, err = getIgnoredCount(ctx, db, tuple, oldestCommitID)
		if err != nil {
			return skerr.Wrap(err)
		}
		// If the traces had no actual data (e.g. are too old or just on CLs), there is nothing to
		// upload to Perf.
		if perfData.NegativeTraces == 0 && perfData.UntriagedTraces == 0 && perfData.PositiveTraces == 0 && perfData.IgnoredTraces == 0 {
			sklog.Infof("No data for tuple %v; consider ignoring it", tuple)
			continue
		}
		if perfData.CommitID == "" {
			// It could happen that all traces for this set of keys is ignored. If that is the case,
			// we will pretend this is happening at the latest commit
			perfData.CommitID = latestCommitID
		}
		// We need to turn commitIDs into GitHashes, since perf only speaks the latter.
		hash, ok := idToGitHash[perfData.CommitID]
		if ok {
			perfData.GitHash = hash
		} else {
			hash, err := getCommitHashForID(ctx, db, perfData.CommitID)
			if err != nil {
				return skerr.Wrap(err)
			}
			idToGitHash[perfData.CommitID] = hash
			perfData.GitHash = hash
		}
		if err := uploadDataToPerf(ctx, tuple, perfData, client); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// getWindowCommitBounds finds the current oldest and newest commit that make up windowSize commits.
func getWindowCommitBounds(ctx context.Context, db *pgxpool.Pool, windowSize int) (schema.CommitID, schema.CommitID, error) {
	ctx, span := trace.StartSpan(ctx, "getWindowCommitBounds")
	defer span.End()
	const statement = `
WITH
RecentCommits AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
)
SELECT MIN(commit_id), MAX(commit_id) FROM RecentCommits
`
	row := db.QueryRow(ctx, statement, windowSize)
	var oldestCommitID schema.CommitID
	var newestCommitID schema.CommitID
	if err := row.Scan(&oldestCommitID, &newestCommitID); err != nil {
		return "", "", skerr.Wrap(err)
	}
	return oldestCommitID, newestCommitID, nil
}

type pair struct {
	Key   string
	Value string
}

type summaryTuple struct {
	Corpus    string
	KeyValues []pair
}

// getTuplesOfKeysToQuery finds all combinations of the non-null values associated with the provided
// keys for the provided corpora that exist in the Traces table (that is, have seen at least one
// data point at some point in Gold's history). Any trace which has a value in the ignoreValues
// slice will be dropped (e.g. Ignoring very old hardware that we haven't tested on in a long time
// could help speed up this process by skipping those models/gpus).
func getTuplesOfKeysToQuery(ctx context.Context, db *pgxpool.Pool, keys, ignoreValues, corpora []string) ([]summaryTuple, error) {
	ctx, span := trace.StartSpan(ctx, "getTuplesOfKeysToQuery")
	defer span.End()
	// Build a statement like:
	// SELECT DISTINCT keys->>'source_type',keys->>'os',keys->>'model'
	// FROM Traces ORDER BY 1, 2, 3, 4
	statement := `SELECT DISTINCT keys->>'source_type'`
	for _, key := range keys {
		// These keys are provided by the maintainer of Gold, not arbitrary input. Thus, we do not
		// need to pass them in as a prepared statement (which is a bit trickier to get right).
		statement += fmt.Sprintf(",keys->>'%s'", sql.Sanitize(key))
	}
	statement += " FROM TRACES WHERE keys->>'source_type' IN "
	statement += sql.ValuesPlaceholders(len(corpora), 1)
	statement += " ORDER BY 1"
	for i := range keys {
		statement += fmt.Sprintf(",%d", i+2)
	}
	var args []interface{}
	for _, corpus := range corpora {
		args = append(args, corpus)
	}

	rows, err := db.Query(ctx, statement, args...)
	if err != nil {
		return nil, skerr.Wrapf(err, "Using statement:\n%s", statement)
	}
	defer rows.Close()
	var rv []summaryTuple
nextRow:
	for rows.Next() {
		var corpus string
		// We have a variable number of columns being returned. Thus, we need to make a slice of
		// that length, using pgtype.Text because the columns can be null ...
		values := make([]pgtype.Text, len(keys))
		// ... and then put pointers to those types into an args slice...
		args := []interface{}{&corpus}
		for i := range values {
			args = append(args, &values[i])
		}
		// ... so we can pass that in as variadic arguments to Scan.
		if err := rows.Scan(args...); err != nil {
			return nil, skerr.Wrap(err)
		}

		tuple := summaryTuple{Corpus: corpus}
		for i := range keys {
			v := values[i]
			if v.Status == pgtype.Null {
				continue nextRow
			}
			// This is easiest to handle here and not in the SQL statement; Otherwise the SQL
			// statement gets more unruly than it already is.
			if util.In(v.String, ignoreValues) {
				continue nextRow
			}
			tuple.KeyValues = append(tuple.KeyValues, pair{Key: keys[i], Value: v.String})
		}
		rv = append(rv, tuple)
	}
	return rv, nil
}

// summaryData is the data that will be uploaded to perf. Each integer represents a count of traces
// that produced positive, negative, or untriaged digests. Ignored traces do not have their digests
// included in the triage count (and are typically untriaged anyway), so those are a separate count.
// If multiple traces produce the same output digest, they will be counted independently, since we
// are counting "traces that produced positive digests" not "number of positive digests produced".
type summaryData struct {
	PositiveTraces  int
	NegativeTraces  int
	UntriagedTraces int
	IgnoredTraces   int

	CommitID schema.CommitID
	GitHash  string
}

// getTriageStatus takes a given tuple of keys and corpus and returns how many traces are triaged
// positive, negative, or not at all. This data is at head unless the "latest" commit for that
// trace is older than oldestCommitID. If two traces produce the same digest, those will be counted
// individually as 2, not combined as 1. It also returns the newest commitID that any of the traces
// which match the tuple produced data, simplifying data collection by assuming all data matching
// this tuple was produced at the same commit.
func getTriageStatus(ctx context.Context, db *pgxpool.Pool, tuple summaryTuple, oldestCommitID schema.CommitID) (summaryData, error) {
	ctx, span := trace.StartSpan(ctx, "getTriageStatus")
	defer span.End()
	statement := "WITH\n" + joinedTracesStatement(tuple)
	statement += `
),
TracesGroupingDigests AS (
    SELECT JoinedTraces.trace_id, grouping_id, digest, most_recent_commit_id
    FROM JoinedTraces
    JOIN ValuesAtHead on JoinedTraces.trace_id = ValuesAtHead.trace_id
    WHERE most_recent_commit_id >= $1 and matches_any_ignore_rule = False and corpus = $2
)
SELECT label, COUNT(*), MAX(most_recent_commit_id) FROM TracesGroupingDigests
JOIN
Expectations ON TracesGroupingDigests.grouping_id = Expectations.grouping_id AND
                TracesGroupingDigests.digest = Expectations.digest
GROUP BY label`
	rows, err := db.Query(ctx, statement, oldestCommitID, tuple.Corpus)
	if err != nil {
		return summaryData{}, skerr.Wrapf(err, "Error running statement:\n%s", statement)
	}
	defer rows.Close()
	var rv summaryData
	for rows.Next() {
		var label schema.ExpectationLabel
		var count int
		var mostRecentCommitID schema.CommitID
		if err := rows.Scan(&label, &count, &mostRecentCommitID); err != nil {
			return summaryData{}, skerr.Wrap(err)
		}
		switch label {
		case schema.LabelPositive:
			rv.PositiveTraces = count
		case schema.LabelNegative:
			rv.NegativeTraces = count
		case schema.LabelUntriaged:
			rv.UntriagedTraces = count
		}
		rv.CommitID = mostRecentCommitID
	}
	return rv, nil
}

// joinedTracesStatement creates a subquery named JoinedTraces that has all the trace ids matching
// all the key-value pairs and the corpus from the passed in tuple. Experimental testing showed that
// this approach with many INTERSECTs was much faster than using the JSON @> syntax, probably due to
// better use of the keys index. The statement is left open should any callers want to add an
// additional clause.
func joinedTracesStatement(tuple summaryTuple) string {
	statement := "JoinedTraces AS ("
	for _, kv := range tuple.KeyValues {
		statement += fmt.Sprintf("\nSELECT trace_id FROM Traces WHERE keys -> '%s' = '%q'",
			sql.Sanitize(kv.Key), sql.Sanitize(kv.Value))
		statement += "\n\tINTERSECT"
	}
	statement += fmt.Sprintf("\nSELECT trace_id FROM Traces WHERE keys -> '%s' = '%q'",
		types.CorpusField, sql.Sanitize(tuple.Corpus))
	return statement
}

// getIgnoredCount counts how many traces produced data more recently than oldestCommitID and match
// one or more ignore rules.
func getIgnoredCount(ctx context.Context, db *pgxpool.Pool, tuple summaryTuple, oldestCommitID schema.CommitID) (int, error) {
	ctx, span := trace.StartSpan(ctx, "getIgnoredCount")
	defer span.End()
	statement := "WITH\n" + joinedTracesStatement(tuple)
	statement += `
	INTERSECT
	SELECT trace_id FROM Traces where matches_any_ignore_rule = true
)
SELECT count(*) FROM JoinedTraces
JOIN ValuesAtHead ON JoinedTraces.trace_id = ValuesAtHead.trace_id
WHERE most_recent_commit_id > $1`

	row := db.QueryRow(ctx, statement, oldestCommitID)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, skerr.Wrap(err)
	}
	return count, nil
}

// getCommitHashForID looks up the githash associated with a commit id.
func getCommitHashForID(ctx context.Context, db *pgxpool.Pool, id schema.CommitID) (string, error) {
	ctx, span := trace.StartSpan(ctx, "getCommitHashForID")
	defer span.End()
	row := db.QueryRow(ctx, `SELECT git_hash FROM GitCommits WHERE commit_id = $1`, id)
	var gitHash string
	if err := row.Scan(&gitHash); err != nil {
		return "", skerr.Wrap(err)
	}
	return gitHash, nil
}

// uploadDataToPerf creates a JSON object in the format expected by Perf that contains all the
// tabulated summaryData and uploads it to the Perf GCS bucket.
func uploadDataToPerf(ctx context.Context, tuple summaryTuple, data summaryData, client gcs.GCSClient) error {
	valueStr := tuple.Corpus
	key := map[string]string{types.CorpusField: tuple.Corpus}
	for _, p := range tuple.KeyValues {
		valueStr += "-" + p.Value
		key[p.Key] = p.Value
	}
	n := now.Now(ctx)
	// We want to make the data easy to find but unlikely to have name collisions. Thus we use
	// the UnixNano of the current time as the filename and build the folder name based on the
	// time and the values of the data.
	perfPath := fmt.Sprintf("gold-summary-v1/%d/%d/%d/%d/%s/%d.json",
		n.Year(), n.Month(), n.Day(), n.Hour(), valueStr, n.UnixNano())
	opts := gcs.FileWriteOptions{
		ContentType: "application/json",
	}

	f := format.Format{
		Version: 1,
		GitHash: data.GitHash,
		Key:     key,
		Results: []format.Result{{
			Key:         map[string]string{"count": "gold_triaged_positive", "unit": "traces"},
			Measurement: float32(data.PositiveTraces),
		}, {
			Key:         map[string]string{"count": "gold_triaged_negative", "unit": "traces"},
			Measurement: float32(data.NegativeTraces),
		}, {
			Key:         map[string]string{"count": "gold_untriaged", "unit": "traces"},
			Measurement: float32(data.UntriagedTraces),
		}, {
			Key:         map[string]string{"count": "gold_ignored", "unit": "traces"},
			Measurement: float32(data.IgnoredTraces),
		}},
	}
	jsonBytes, err := json.MarshalIndent(f, "", "\t")
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := client.SetFileContents(ctx, perfPath, opts, jsonBytes); err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("Uploaded summary to perf %s", perfPath)
	return nil
}
