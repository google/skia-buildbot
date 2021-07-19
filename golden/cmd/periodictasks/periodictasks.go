package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/commenter"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/code_review/github_crs"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/ignore/sqlignorestore"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/tracing"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Arbitrary number
	maxSQLConnections = 4
)

// TODO(kjlubick) Add a task to check for abandoned CLs.

type periodicTasksConfig struct {
	config.Common

	// ChangelistDiffPeriod is how often to look at recently updated CLs and tabulate the diffs
	// for the digests produced.
	// The diffs are not calculated in this service, but sent via pubsub to the appropriate workers.
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

	psClient := mustInitPubsubClient(ctx, ptc)

	startUpdateTracesIgnoreStatus(ctx, db, ptc)

	startCommentOnCLs(ctx, db, ptc)

	gatherer := &diffWorkGatherer{
		db:         db,
		windowSize: ptc.WindowSize,
		publisher: &pubsubDiffPublisher{
			client: psClient,
			topic:  ptc.DiffWorkTopic,
		},
		mostRecentCLScan: now.Now(ctx).Add(-5 * 24 * time.Hour),
	}
	startPrimaryBranchDiffWork(ctx, gatherer, ptc)
	startChangelistsDiffWork(ctx, gatherer, ptc)

	sklog.Infof("periodic tasks have been started")
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	sklog.Fatal(http.ListenAndServe(ptc.ReadyPort, nil))
}

func mustInitPubsubClient(ctx context.Context, ptc periodicTasksConfig) *pubsub.Client {
	psc, err := pubsub.NewClient(ctx, ptc.PubsubProjectID)
	if err != nil {
		sklog.Fatalf("initializing pubsub client for project %s: %s", ptc.PubsubProjectID, err)
	}
	return psc
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
	tokenSource, err := auth.NewDefaultTokenSource(ptc.Local, auth.SCOPE_GERRIT)
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

type workPublisher interface {
	PublishWork(ctx context.Context, grouping paramtools.Params, newDigests []types.Digest) error
}

type diffWorkGatherer struct {
	db         *pgxpool.Pool
	publisher  workPublisher
	windowSize int

	mostRecentCLScan time.Time
}

// startPrimaryBranchDiffWork starts the process that periodically creates Pub/Sub messages for
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
// publishes a Pub/Sub notification for each.
func (g *diffWorkGatherer) gatherFromPrimaryBranch(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "gatherFromPrimaryBranch")
	defer span.End()
	const statement = `WITH
RecentCommits AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
),
FirstCommitInWindow AS (
	SELECT commit_id FROM RecentCommits
	ORDER BY commit_id ASC LIMIT 1
),
GroupingsWithRecentData AS (
	SELECT DISTINCT grouping_id FROM ValuesAtHead
	JOIN FirstCommitInWindow ON ValuesAtHead.most_recent_commit_id >= FirstCommitInWindow.commit_id
)
SELECT Groupings.keys FROM Groupings
JOIN GroupingsWithRecentData on Groupings.grouping_id = GroupingsWithRecentData.grouping_id
`
	rows, err := g.db.Query(ctx, statement, g.windowSize)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var grouping paramtools.Params
		if err := rows.Scan(&grouping); err != nil {
			return skerr.Wrap(err)
		}
		if err := g.publisher.PublishWork(ctx, grouping, nil); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// startChangelistsDiffWork starts the process that periodically creates Pub/Sub messages for
//// diff workers to calculate diffs for images produced by CLs.
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

// gatherFromChangelists scans all recently updated CLs and sends a Pub/Sub message for any grouping
// in those CLs that saw images not already on the primary branch.
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
	if len(clIDs) == 0 {
		return nil
	}
	for _, cl := range clIDs {
		sklog.Debugf("Publishing diffs for CL %s", cl)
		if err := g.publishDiffsForCL(ctx, firstTileIDInWindow, cl); err != nil {
			return skerr.Wrap(err)
		}
	}
	g.mostRecentCLScan = updatedTS
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

// publishDiffsForCL finds all data produced by this CL and compares it against the data on the
// primary branch. If any digest is not already on the primary branch (in the sliding window), it
// will be bunched up and sent for that grouping.
func (g *diffWorkGatherer) publishDiffsForCL(ctx context.Context, startingTile schema.TileID, cl string) error {
	ctx, span := trace.StartSpan(ctx, "publishDiffsForCL")
	defer span.End()
	span.AddAttributes(trace.StringAttribute("CL", cl))
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
SELECT Groupings.grouping_id, Groupings.keys, encode(DigestsNotOnPrimaryBranch.digest, 'hex') FROM DigestsNotOnPrimaryBranch
JOIN Groupings ON DigestsNotOnPrimaryBranch.grouping_id = Groupings.grouping_id
ORDER BY 1, 3
`
	rows, err := g.db.Query(ctx, statement, cl, startingTile)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer rows.Close()
	var newDigests []types.Digest
	var currentGroupingID schema.GroupingID
	var currentGrouping paramtools.Params
	for rows.Next() {
		var digest types.Digest
		var groupingID schema.GroupingID
		var grouping paramtools.Params
		if err := rows.Scan(&groupingID, &grouping, &digest); err != nil {
			return skerr.Wrap(err)
		}
		if currentGroupingID == nil {
			currentGroupingID = groupingID
			currentGrouping = grouping
		}
		if bytes.Equal(currentGroupingID, groupingID) {
			newDigests = append(newDigests, digest)
		} else {
			if err := g.publisher.PublishWork(ctx, currentGrouping, newDigests); err != nil {
				return skerr.Wrap(err)
			}
			currentGroupingID = groupingID
			currentGrouping = grouping
			// Reset newDigests to be empty and then start adding the new digests to it.
			newDigests = append(newDigests[:0], digest)
		}
	}
	if currentGroupingID == nil {
		return nil // nothing to report
	}
	// Publish the last set
	if err := g.publisher.PublishWork(ctx, currentGrouping, newDigests); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

type pubsubDiffPublisher struct {
	client *pubsub.Client
	topic  string
}

// PublishWork publishes a WorkerMessage to the configured PubSub topic so that a worker
// (see diffcalculator) can pick it up and calculate the diffs.
func (p *pubsubDiffPublisher) PublishWork(ctx context.Context, grouping paramtools.Params, newDigests []types.Digest) error {
	body, err := json.Marshal(diff.WorkerMessage{
		Version:         diff.WorkerMessageVersion,
		Grouping:        grouping,
		AdditionalLeft:  newDigests,
		AdditionalRight: newDigests, // We want to compare all new digests from CLs with themselves
	})
	if err != nil {
		return skerr.Wrap(err) // should never happen because JSON input is well-formed.
	}
	p.client.Topic(p.topic).Publish(ctx, &pubsub.Message{
		Data: body,
	})
	// Don't block until message is sent to speed up throughput.
	return nil
}
