package main

import (
	"context"
	"flag"
	"net/http"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/code_review/commenter"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/ignore/sqlignorestore"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/tracing"
)

const (
	// Arbitrary number
	maxSQLConnections = 4
)

// TODO(kjlubick) other periodic tasks
//   - Send tasks to diffworker queue (de-duplicating where possible).

type periodicTasksConfig struct {
	config.Common

	// A string with placeholders for generating a comment message. See
	// commenter.commentTemplateContext for the exact fields.
	CLCommentTemplate string `json:"cl_comment_template" optional:"true"`

	// CommentOnCLsPeriod, if positive, is how often to check recent CLs and Patchsets for
	// untriaged digests and comment on them if appropriate.
	CommentOnCLsPeriod config.Duration `json:"comment_on_cls_period" optional:"true"`

	// UpdateIgnorePeriod is how often we should try to apply the ignore rules to all traces.
	UpdateIgnorePeriod config.Duration `json:"null_ignore_period"` // TODO(kjlubick) change JSON
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
	var systems []commenter.ReviewSystem

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
