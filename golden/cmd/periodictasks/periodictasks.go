package main

import (
	"context"
	"flag"
	"net/http"

	"cloud.google.com/go/pubsub"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
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
//   - Fix any races with trace rules (re-apply all rules to Traces and ValuesAtHead)
//   - Send tasks to diffworker queue (de-duplicating where possible).

type periodicTasksConfig struct {
	config.Common

	// NullIgnorePeriod is how often we should try to apply the ignore rules to null traces.
	NullIgnorePeriod config.Duration `json:"null_ignore_period"`
	// RepoUpdateSubscription is the subscription to use to listen to the RepoUpdateTopic.
	RepoUpdateSubscription string `json:"repo_update_subscription" optional:"true"`
	// RepoUpdateTopic is the Pub/Sub topic to which repo updates will be pushed. We listen to this
	// so we can determine when a CL lands. This topic should be setup via
	// https://cloud.google.com/source-repositories/docs/configuring-notifications
	RepoUpdateTopic string `json:"repo_update_topic" optional:"true"`
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

	startUpdateTracesWithNullStatus(ctx, db, ptc)

	startListenForLandedChangelists(ctx, db, ptc)

	sklog.Infof("periodic tasks have been started")
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	sklog.Fatal(http.ListenAndServe(ptc.ReadyPort, nil))
}

func startUpdateTracesWithNullStatus(ctx context.Context, db *pgxpool.Pool, ptc periodicTasksConfig) {
	liveness := metrics2.NewLiveness("periodic_tasks", map[string]string{
		"task": "updateTracesWithNullStatus",
	})
	go util.RepeatCtx(ctx, ptc.NullIgnorePeriod.Duration, func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "updateTracesWithNullStatus")
		defer span.End()
		if err := sqlignorestore.UpdateIgnoredTraces(ctx, db); err != nil {
			sklog.Error("Error while updating null traces: %s", err)
			return // return so the liveness is not updated
		}
		liveness.Reset()
		sklog.Infof("Done with updateTracesWithNullStatus")
	})
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

func startListenForLandedChangelists(ctx context.Context, _ *pgxpool.Pool, ptc periodicTasksConfig) {
	if ptc.RepoUpdateTopic == "" || ptc.RepoUpdateSubscription == "" {
		sklog.Info("Skipping listening for repo updates")
		return
	}
	liveness := metrics2.NewLiveness("periodic_tasks", map[string]string{
		"task": "commitLanded",
	})

	psc, err := pubsub.NewClient(ctx, ptc.PubsubProjectID)
	if err != nil {
		sklog.Fatalf("Error initializing pubsub client for project %s: %s", ptc.PubsubProjectID, err)
	}

	// Check that the topic exists. Fail if it does not.
	t := psc.Topic(ptc.RepoUpdateTopic)
	if exists, err := t.Exists(ctx); err != nil {
		sklog.Fatalf("Error checking for existing topic %s: %s", ptc.RepoUpdateTopic, err)
	} else if !exists {
		sklog.Fatalf("Repo update topic %s does not exist in project %s", ptc.RepoUpdateTopic, ptc.PubsubProjectID)
	}

	// Check that the subscription exists. Fail if it does not.
	sub := psc.Subscription(ptc.RepoUpdateSubscription)
	if exists, err := sub.Exists(ctx); err != nil {
		sklog.Fatalf("Error checking for existing subscription %s", ptc.RepoUpdateSubscription)
	} else if !exists {
		sklog.Fatalf("subscription %s does not exist in project %s", ptc.RepoUpdateSubscription, ptc.PubsubProjectID)
	}

	// This process will handle one message at a time. This allows us to more finely control the
	// scaling up as necessary.
	sub.ReceiveSettings.NumGoroutines = 1
	go func() {
		err := sub.Receive(ctx, func(ctx context.Context, message *pubsub.Message) {
			ctx, span := trace.StartSpan(ctx, "periodic_RepoUpdate")
			defer span.End()
			liveness.Reset()
			sklog.Infof("Repo Update: %v", message.Attributes)
			sklog.Infof("JSON? %s", string(message.Data))
			message.Ack()
		})
		sklog.Errorf("Pub/Sub receiver died: %s", err)
	}()
	sklog.Infof("Listening for repo updates on %s-%s", ptc.RepoUpdateTopic, ptc.RepoUpdateSubscription)
}
