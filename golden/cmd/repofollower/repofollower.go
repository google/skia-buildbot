// The repofollower executable monitors the repo we are tracking and fills in the GitCommits table.
package main

import (
	"context"
	"flag"
	"math/rand"
	"net/http"
	"time"

	"go.skia.org/infra/go/gitiles"

	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/tracing"
)

const (
	// Abitrary number
	maxSQLConnections = 4
)

type repoFollowerConfig struct {
	config.Common

	CommitZero string `json:"commit_zero"`

	// PollPeriod is how often we should poll the source of truth.
	PollPeriod config.Duration `json:"poll_period"`

	// Metrics service address (e.g., ':10110')
	PromPort string `json:"prom_port"`

	// The port to provide a web handler for /healthz
	ReadyPort string `json:"ready_port"`
}

func main() {
	// Command line flags.
	var (
		commonInstanceConfig = flag.String("common_instance_config", "", "Path to the json5 file containing the configuration that needs to be the same across all services for a given instance.")
		thisConfig           = flag.String("config", "", "Path to the json5 file containing the configuration specific to baseline server.")
		hang                 = flag.Bool("hang", false, "Stop and do nothing after reading the flags. Good for debugging containers.")
	)

	// Parse the options. So we can configure logging.
	flag.Parse()

	if *hang {
		sklog.Info("Hanging")
		select {}
	}
	rand.Seed(time.Now().UnixNano())

	var rfc repoFollowerConfig
	if err := config.LoadFromJSON5(&rfc, commonInstanceConfig, thisConfig); err != nil {
		sklog.Fatalf("Reading config: %s", err)
	}
	sklog.Infof("Loaded config %#v", rfc)

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(&rfc.PromPort),
	}

	common.InitWithMust("repofollower", logOpts...)
	if err := tracing.Initialize(1); err != nil {
		sklog.Fatalf("Could not set up tracing: %s", err)
	}

	ctx := context.Background()

	db := mustInitSQLDatabase(ctx, rfc)
	// TODO(kjlubick) authenticated gitiles client
	gitilesClient := gitiles.NewRepo(rfc.GitRepoURL, httputils.NewTimeoutClient())
	go pollAndFill(ctx, db, gitilesClient, rfc)

	// Wait at least 5 seconds for polling to start before signaling all is well.
	time.Sleep(5 * time.Second)
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	sklog.Fatal(http.ListenAndServe(rfc.ReadyPort, nil))
}

func mustInitSQLDatabase(ctx context.Context, fcc repoFollowerConfig) *pgxpool.Pool {
	if fcc.SQLDatabaseName == "" {
		sklog.Fatalf("Must have SQL Database Information")
	}
	url := sql.GetConnectionURL(fcc.SQLConnection, fcc.SQLDatabaseName)
	conf, err := pgxpool.ParseConfig(url)
	if err != nil {
		sklog.Fatalf("error getting postgres config %s: %s", url, err)
	}

	conf.MaxConns = maxSQLConnections
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	sklog.Infof("Connected to SQL database %s", fcc.SQLDatabaseName)
	return db
}

func pollAndFill(ctx context.Context, db *pgxpool.Pool, client *gitiles.Repo, rfc repoFollowerConfig) {
	ct := time.Tick(rfc.PollPeriod.Duration)
	for {
		select {
		case <-ctx.Done():
			sklog.Errorf("Stopping polling due to context error: %s", ctx.Err())
			return
		case <-ct:
			updateCycle(ctx, db, client)
		}
	}
}

func updateCycle(ctx context.Context, db *pgxpool.Pool, client *gitiles.Repo) {
	panic("not implemented")
}
