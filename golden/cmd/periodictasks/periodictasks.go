package main

import (
	"bytes"
	"context"
	"flag"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
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
	"go.skia.org/infra/golden/go/tracing"
	"go.skia.org/infra/golden/go/types"
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

	// PrimaryBranchDiffPeriod is how often to look at the most recent window of commits and
	// tabulate diffs between all groupings based on the digests produced on the primary branch.
	// The diffs are not calculated in this service, but sent via Pub/Sub to the appropriate workers.
	PrimaryBranchDiffPeriod config.Duration `json:"primary_branch_diff_period"`

	// UpdateIgnorePeriod is how often we should try to apply the ignore rules to all traces.
	UpdateIgnorePeriod config.Duration `json:"update_traces_ignore_period"` // TODO(kjlubick) change JSON
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
	if err := tracing.Initialize(tp); err != nil {
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
	systems := mustInitializeSystems(ptc)
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
func mustInitializeSystems(ptc periodicTasksConfig) []commenter.ReviewSystem {
	tokenSource, err := auth.NewDefaultTokenSource(ptc.Local, auth.ScopeGerrit)
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
