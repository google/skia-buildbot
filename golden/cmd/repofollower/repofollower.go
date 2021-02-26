// The repofollower executable monitors the repo we are tracking and fills in the GitCommits table.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"go.skia.org/infra/go/vcsinfo"

	"github.com/jackc/pgx/v4"

	"go.skia.org/infra/go/skerr"

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
	// Arbitrary number
	maxSQLConnections = 4

	// The initial commit will be given this commit ID. Subsequent commits will have monotonically
	// increasing integers as IDs.
	initialID = 1_000_000_000

	// If overrideLatestCommit is set on a context, the associated value will be used instead of
	// querying gitiles (which changes over time). This is used by tests.
	overrideLatestCommitKey = contextKey("override_latest_commit")
)

type contextKey string

type repoFollowerConfig struct {
	config.Common

	InitialCommit string `json:"initial_commit"`

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
			err := updateCycle(ctx, db, client, rfc)
			if err != nil {
				sklog.Errorf("Error on this cycle for talking to %s: %s", rfc.GitRepoURL, rfc)
			}
		}
	}
}

func updateCycle(ctx context.Context, db *pgxpool.Pool, client *gitiles.Repo, rfc repoFollowerConfig) error {
	latestHash, err := getLatestCommitFromRepo(ctx, client, rfc)
	if err != nil {
		return skerr.Wrap(err)
	}

	previousHash, previousID, err := getPreviousCommitFromDB(ctx, db)
	if err != nil {
		return skerr.Wrapf(err, "getting recent commits from DB")
	}

	if previousHash == latestHash {
		sklog.Infof("no updates - latest seen commit %s", previousHash)
		return nil
	}
	if previousHash == "" {
		previousHash = rfc.InitialCommit
		previousID = initialID
	}

	sklog.Infof("Getting git history from %s to %s", previousHash, latestHash)
	commits, err := client.LogFirstParent(ctx, previousHash, latestHash)
	if err != nil {
		return skerr.Wrapf(err, "getting backlog of commits from %s..%s", previousHash, latestHash)
	}
	// commits is backwards and LogFirstParent does not respect gitiles.LogReverse()
	sklog.Infof("Got %d commits to store", len(commits))

	if err := storeCommits(ctx, db, previousID, commits); err != nil {
		return skerr.Wrapf(err, "storing %d commits to GitCommits table", len(commits))
	}
	return nil
}

// getLatestCommitFromRepo returns the git hash of the latest git commit known on the configured
// branch. If overrideLatestCommitKey has a value set, that will be used instead.
func getLatestCommitFromRepo(ctx context.Context, client *gitiles.Repo, rfc repoFollowerConfig) (string, error) {
	if hash := ctx.Value(overrideLatestCommitKey); hash != nil {
		return hash.(string), nil
	}
	latestCommit, err := client.Log(ctx, rfc.GitRepoBranch, gitiles.LogLimit(1))
	if err != nil {
		return "", skerr.Wrapf(err, "getting last commit")
	}
	if len(latestCommit) < 1 {
		return "", skerr.Fmt("No commits returned")
	}
	sklog.Debugf("latest commit: %#v", latestCommit[0])
	return latestCommit[0].Hash, nil
}

func getPreviousCommitFromDB(ctx context.Context, db *pgxpool.Pool) (string, int64, error) {
	row := db.QueryRow(ctx, `SELECT git_hash, commit_id FROM GitCommits
ORDER BY commit_id DESC LIMIT 1`)
	hash := ""
	id := ""
	if err := row.Scan(&hash, &id); err != nil {
		if err == pgx.ErrNoRows {
			return "", 0, nil // No data in GitCommits
		}
		return "", 0, skerr.Wrap(err)
	}
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return "", 0, skerr.Wrapf(err, "It is assumed that the commit ids for this type of repo tracking are ints: %q", id)
	}
	return hash, idInt, nil
}

func storeCommits(ctx context.Context, db *pgxpool.Pool, lastCommitID int64, commits []*vcsinfo.LongCommit) error {
	const statement = `UPSERT INTO GitCommits (git_hash, commit_id, commit_time, author_email, subject) VALUES `
	const valuesPerRow = 5
	arguments := make([]interface{}, 0, len(commits)*valuesPerRow)
	commitID := lastCommitID + 1
	for i := range commits {
		// commits is in backwards order. This reverses things
		c := commits[len(commits)-i-1]
		//fmt.Printf("Commit: %v\n", c)
		cid := fmt.Sprintf("%012d", commitID)
		arguments = append(arguments, c.Hash, cid, c.Timestamp, c.Author, c.Subject)
		commitID++
	}
	vp := sql.ValuesPlaceholders(valuesPerRow, len(commits))
	if _, err := db.Exec(ctx, statement+vp, arguments...); err != nil {
		return skerr.Wrap(err)
	}
	return nil

}
