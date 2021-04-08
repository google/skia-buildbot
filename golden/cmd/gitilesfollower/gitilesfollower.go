// The gitilesfollower executable monitors the repo we are tracking using gitiles. It fills in
// the GitCommits table, specifically making a mapping between a GitHash and CommitID. The CommitID
// is based on the "index" of the commit (i.e. how many commits since the initial commit).
//
// This will be used by all clients that have their tests in the same repo as the code under test.
// Clients with more complex repo structures, will need to have an alternate way of linking
// commit_id to git_hash.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/tracing"
)

const (
	// Arbitrary number
	maxSQLConnections = 4

	// The initial commit will be given this commit ID. Subsequent commits will have monotonically
	// increasing integers as IDs. We pick this number instead of zero in case we need to go
	// backwards, we can assign a non-negative integer as an id (which won't break the sort order
	// when turned into a string).
	initialID = 1_000_000_000

	// If overrideLatestCommit is set on a context, the associated value will be used instead of
	// querying gitiles (which changes over time). This is used by tests.
	overrideLatestCommitKey = contextKey("override_latest_commit")
)

type contextKey string // See advice in https://golang.org/pkg/context/#WithValue

type repoFollowerConfig struct {
	config.Common

	// InitialCommit that we will use if there are no existing commits in the DB. It will be counted
	// like a "commit zero", which we actually assign to commit 1 billion in case we need to go back
	// in time, we can sort our commit_ids without resorting to negative numbers.
	InitialCommit string `json:"initial_commit"`

	// ReposToMonitorCLs is a list of all repos that we need to monitor for commits that could
	// correspond to CLs. When those CLs land, we need to merge in the expectations associated
	// with that CL to the primary branch.
	ReposToMonitorCLs []monitorConfig `json:"repos_to_monitor_cls"`

	// PollPeriod is how often we should poll the source of truth.
	PollPeriod config.Duration `json:"poll_period"`
}

// monitorConfig houses the data we need to track a repo and determine which CLs a landed commit
// corresponds to.
type monitorConfig struct {
	// RepoURL is the url that will be polled via gitiles.
	RepoURL string `json:"repo_url"`
	// ExtractionTechnique codifies the methods for linking (via a commit message/body) to a CL.
	ExtractionTechnique extractionTechnique `json:"extraction_technique"`
	// InitialCommit that we will used if there is no monitoring progress stored in the DB.
	// It should be before/at the point where we migrated expectations.
	InitialCommit string `json:"initial_commit"`
	// SystemName is the abbreviation that is given to a given CodeReviewSystem.
	SystemName string `json:"system_name"`
	// Branch is generally the primary branch of the repo that we will poll for the latest commit.
	Branch string `json:"branch"`
	// LegacyUpdaterInUse indicates the status of the CLs should not be changed because the source
	// of truth for expectations is still Firestore, which is controlled by gold_frontend.
	// This should be able to be removed after the SQL migration is complete.
	LegacyUpdaterInUse bool `json:"legacy_updater_in_use"`
}

type extractionTechnique string

const (
	// ReviewedLine corresponds to looking for a Reviewed-on line in the commit message.
	ReviewedLine = extractionTechnique("ReviewedLine")
	// FromSubject corresponds to looking at the title for a CL ID in square brackets.
	FromSubject = extractionTechnique("FromSubject")
)

func main() {
	// Command line flags.
	var (
		commonInstanceConfig = flag.String("common_instance_config", "", "Path to the json5 file containing the configuration that needs to be the same across all services for a given instance.")
		thisConfig           = flag.String("config", "", "Path to the json5 file containing the configuration specific to gitiles follower server.")
		hang                 = flag.Bool("hang", false, "Stop and do nothing after reading the flags. Good for debugging containers.")
	)

	// Parse the options. So we can configure logging.
	flag.Parse()

	if *hang {
		sklog.Info("Hanging")
		select {}
	}

	var rfc repoFollowerConfig
	if err := config.LoadFromJSON5(&rfc, commonInstanceConfig, thisConfig); err != nil {
		sklog.Fatalf("Reading config: %s", err)
	}
	sklog.Infof("Loaded config %#v", rfc)

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(&rfc.PromPort),
	}

	common.InitWithMust("gitilesfollower", logOpts...)
	if err := tracing.Initialize(1); err != nil {
		sklog.Fatalf("Could not set up tracing: %s", err)
	}

	ctx := context.Background()
	db := mustInitSQLDatabase(ctx, rfc)

	ts, err := auth.NewDefaultTokenSource(false, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatalf("Problem setting up default token source: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	gitilesClient := gitiles.NewRepo(rfc.GitRepoURL, client)
	// This starts a goroutine in the background
	if err := pollRepo(ctx, db, gitilesClient, rfc); err != nil {
		sklog.Fatalf("Could not do initial update: %s", err)
	}
	if err := checkForLanded(ctx, db, client, rfc); err != nil {
		sklog.Fatalf("Could not do initial scan of landed CLs: %s", err)
	}
	sklog.Infof("Initial update complete")
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

// pollRepo does an initial updateCycle and starts a goroutine to continue updating according
// to the provided duration for as long as the context remains ok.
func pollRepo(ctx context.Context, db *pgxpool.Pool, client *gitiles.Repo, rfc repoFollowerConfig) error {
	sklog.Infof("Doing initial update")
	err := updateCycle(ctx, db, client, rfc)
	if err != nil {
		return skerr.Wrap(err)
	}
	go func() {
		ct := time.NewTicker(rfc.PollPeriod.Duration)
		defer ct.Stop()
		sklog.Infof("Polling every %s", rfc.PollPeriod.Duration)
		for {
			select {
			case <-ctx.Done():
				sklog.Errorf("Stopping polling due to context error: %s", ctx.Err())
				return
			case <-ct.C:
				err := updateCycle(ctx, db, client, rfc)
				if err != nil {
					sklog.Errorf("Error on this cycle for talking to %s: %s", rfc.GitRepoURL, err)
				}
			}
		}
	}()
	return nil
}

// GitilesLogger is a subset of the gitiles client library that we need. This allows us to mock
// it out during tests.
type GitilesLogger interface {
	Log(ctx context.Context, logExpr string, opts ...gitiles.LogOption) ([]*vcsinfo.LongCommit, error)
	LogFirstParent(ctx context.Context, from, to string, opts ...gitiles.LogOption) ([]*vcsinfo.LongCommit, error)
}

// updateCycle polls the gitiles repo for the latest commit and the database for the previously
// seen commit. If those are different, it polls gitiles for all commits that happened between
// those two points and stores them to the DB.
func updateCycle(ctx context.Context, db *pgxpool.Pool, client GitilesLogger, rfc repoFollowerConfig) error {
	ctx, span := trace.StartSpan(ctx, "gitilesfollower_updateCycle")
	defer span.End()
	latestHash, err := getLatestCommitFromRepo(ctx, client, rfc.GitRepoBranch)
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
	reverse(commits)
	sklog.Infof("Got %d commits to store", len(commits))
	if err := storeCommits(ctx, db, previousID, commits); err != nil {
		return skerr.Wrapf(err, "storing %d commits to GitCommits table", len(commits))
	}
	return nil
}

// reverses the order of the slice.
func reverse(commits []*vcsinfo.LongCommit) {
	total := len(commits)
	for i := 0; i < total/2; i++ {
		commits[i], commits[total-i-1] = commits[total-i-1], commits[i]
	}
}

// getLatestCommitFromRepo returns the git hash of the latest git commit known on the configured
// branch. If overrideLatestCommitKey has a value set, that will be used instead.
func getLatestCommitFromRepo(ctx context.Context, client GitilesLogger, branch string) (string, error) {
	if hash := ctx.Value(overrideLatestCommitKey); hash != nil {
		return hash.(string), nil
	}
	ctx, span := trace.StartSpan(ctx, "gitilesfollower_getLatestCommitFromRepo")
	defer span.End()
	latestCommit, err := client.Log(ctx, branch, gitiles.LogLimit(1))
	if err != nil {
		return "", skerr.Wrapf(err, "getting last commit")
	}
	if len(latestCommit) < 1 {
		return "", skerr.Fmt("No commits returned")
	}
	return latestCommit[0].Hash, nil
}

// getPreviousCommitFromDB returns the git_hash and the commit_id of the most recently stored
// commit. "Most recent" here is defined by the lexicographical order of the commit_id. Of note,
// commit_id is returned as an integer because subsequent ids will be computed by adding to that
// integer value.
//
// This approach takes a lesson from Perf by only querying data from the most recent commit in the
// DB and the latest on the tree to make Gold resilient to merged/changed history.
// (e.g. go/skia-infra-pm-007)
func getPreviousCommitFromDB(ctx context.Context, db *pgxpool.Pool) (string, int64, error) {
	ctx, span := trace.StartSpan(ctx, "gitilesfollower_getPreviousCommitFromDB")
	defer span.End()
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

// storeCommits writes the given commits to the SQL database, assigning them commitIDs in
// monotonically increasing order. The commits slice is expected to be sorted with the oldest
// commit first (the opposite of how gitiles returns it).
func storeCommits(ctx context.Context, db *pgxpool.Pool, lastCommitID int64, commits []*vcsinfo.LongCommit) error {
	ctx, span := trace.StartSpan(ctx, "gitilesfollower_storeCommits")
	defer span.End()
	commitID := lastCommitID + 1
	// batchSize is only really relevant in the initial load. But we need it to avoid going over
	// the 65k limit of placeholder indexes.
	const batchSize = 1000
	const statement = `UPSERT INTO GitCommits (git_hash, commit_id, commit_time, author_email, subject) VALUES `
	const valuesPerRow = 5
	err := util.ChunkIter(len(commits), batchSize, func(startIdx int, endIdx int) error {
		chunk := commits[startIdx:endIdx]
		arguments := make([]interface{}, 0, len(chunk)*valuesPerRow)
		for _, c := range chunk {
			cid := fmt.Sprintf("%012d", commitID)
			arguments = append(arguments, c.Hash, cid, c.Timestamp, c.Author, c.Subject)
			commitID++
		}
		vp := sql.ValuesPlaceholders(valuesPerRow, len(chunk))
		if _, err := db.Exec(ctx, statement+vp, arguments...); err != nil {
			return skerr.Wrap(err)
		}
		return nil
	})
	return skerr.Wrap(err)
}

// checkForLanded will check all recent commits in the ReposToMonitor for any references to CLs
// that have landed. Then, it starts a go routine to continue this periodically.
func checkForLanded(ctx context.Context, db *pgxpool.Pool, client *http.Client, rfc repoFollowerConfig) error {
	if len(rfc.ReposToMonitorCLs) == 0 {
		return skerr.Fmt("No repos to monitor landed CLs")
	}
	sklog.Infof("Doing initial check for landed CLs")
	var gClients []*gitiles.Repo
	for _, repo := range rfc.ReposToMonitorCLs {
		gClients = append(gClients, gitiles.NewRepo(repo.RepoURL, client))
	}
	for i, client := range gClients {
		err := checkForLandedCycle(ctx, db, client, rfc.ReposToMonitorCLs[i])
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	go func() {
		ct := time.NewTicker(rfc.PollPeriod.Duration)
		defer ct.Stop()
		sklog.Infof("Checking for landed CLs every %s", rfc.PollPeriod.Duration)
		for {
			select {
			case <-ctx.Done():
				sklog.Errorf("Stopping landed check due to context error: %s", ctx.Err())
				return
			case <-ct.C:
				for i, client := range gClients {
					err := checkForLandedCycle(ctx, db, client, rfc.ReposToMonitorCLs[i])
					if err != nil {
						sklog.Errorf("Error checking for landed commits with configuration %s", rfc.ReposToMonitorCLs[i], err)
					}
				}
				sklog.Infof("Checked %d repos for landed CLs", len(gClients))
			}
		}
	}()
	return nil
}

// checkForLandedCycle will see if there are any recent commits for the given repo. If there are,
// it will find any corresponding CLs for them and migrate the expectations associated with them
// to the primary branch and mark them as "landed" in the DB.
func checkForLandedCycle(ctx context.Context, db *pgxpool.Pool, client GitilesLogger, m monitorConfig) error {
	ctx, span := trace.StartSpan(ctx, "gitilesfollower_checkForLandedCycle")
	span.AddAttributes(trace.StringAttribute("repo", m.RepoURL))
	defer span.End()
	latestHash, err := getLatestCommitFromRepo(ctx, client, m.Branch)
	if err != nil {
		return skerr.Wrap(err)
	}
	previousHash, err := getPreviouslyLandedCommit(ctx, db, m.RepoURL)
	if err != nil {
		return skerr.Wrapf(err, "getting recently landed commit from DB for repo %s", m.RepoURL)
	}
	if previousHash == latestHash {
		sklog.Infof("no updates - latest seen commit %s", previousHash)
		return nil
	}
	if previousHash == "" {
		previousHash = m.InitialCommit
	}

	sklog.Infof("Getting git history from %s to %s", previousHash, latestHash)
	commits, err := client.LogFirstParent(ctx, previousHash, latestHash)
	if err != nil {
		return skerr.Wrapf(err, "getting backlog of commits from %s..%s", previousHash, latestHash)
	}
	if len(commits) == 0 {
		sklog.Warningf("No commits between %s and %s", previousHash, latestHash)
		return nil
	}
	// commits is backwards and LogFirstParent does not respect gitiles.LogReverse()
	reverse(commits)
	sklog.Infof("Found %d commits to check for a CL", len(commits))
	for _, c := range commits {
		var clID string
		switch m.ExtractionTechnique {
		case ReviewedLine:
			clID = extractReviewedLine(c.Body)
		case FromSubject:
			clID = extractFromSubject(c.Subject)
		}
		if clID == "" {
			sklog.Infof("No CL detected for %#v", c)
			continue
		}
		if err := migrateExpectationsToPrimaryBranch(ctx, db, m.SystemName, clID, c.Timestamp, !m.LegacyUpdaterInUse); err != nil {
			return skerr.Wrapf(err, "migrating cl %s-%s", m.SystemName, clID)
		}
		sklog.Infof("Commit %s landed at %s", c.Hash[:12], c.Timestamp)
	}
	_, err = db.Exec(ctx, `UPSERT INTO TrackingCommits (repo, last_git_hash) VALUES ($1, $2)`, m.RepoURL, latestHash)
	return skerr.Wrap(err)
}

// getPreviouslyLandedCommit returns the git hash of the last commit we checked for a CL in the
// given repo.
func getPreviouslyLandedCommit(ctx context.Context, db *pgxpool.Pool, repoURL string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "getPreviouslyLandedCommit")
	defer span.End()
	row := db.QueryRow(ctx, `SELECT last_git_hash FROM TrackingCommits WHERE repo = $1`, repoURL)
	var rv string
	if err := row.Scan(&rv); err != nil {
		if err == pgx.ErrNoRows {
			return "", nil // No data in TrackingCommits
		}
		return "", skerr.Wrap(err)
	}
	return rv, nil
}

var reviewedLineRegex = regexp.MustCompile(`(^|\n)Reviewed-on: .+/(?P<clID>\S+?)($|\n)`)

// extractReviewedLine looks for a line that starts with Reviewed-on and then parses out the
// CL id from that line (which are the last characters after the last slash).
func extractReviewedLine(clBody string) string {
	match := reviewedLineRegex.FindStringSubmatch(clBody)
	if len(match) > 0 {
		return match[2] // the second group should be our CL ID
	}
	return ""
}

// We assume a PR has the pull request number in the Subject/Title, at the end.
// e.g. "Turn off docs upload temporarily (#44365) (#44413)" refers to PR 44413
var prSuffix = regexp.MustCompile(`.+\(#(?P<id>\d+)\)\s*$`)

// extractFromSubject looks at the subject of a CL and expects to find the associated CL (aka Pull
// Request) appended to the message.
func extractFromSubject(subject string) string {
	if match := prSuffix.FindStringSubmatch(subject); match != nil {
		// match[0] is the whole string, match[1] is the first group
		return match[1]
	}
	return ""
}

// migrateExpectationsToPrimaryBranch finds all the expectations that were added for a given CL
// and condenses them into one record per user who triaged digests on that CL. These records are
// all stored with the same timestamp as the commit that landed with them. The records and their
// corresponding expectations are added to the primary branch. Then the given CL is marked as
// "landed".
func migrateExpectationsToPrimaryBranch(ctx context.Context, db *pgxpool.Pool, crs, clID string, landedTS time.Time, setLanded bool) error {
	ctx, span := trace.StartSpan(ctx, "migrateExpectationsToPrimaryBranch")
	defer span.End()
	qID := sql.Qualify(crs, clID)
	changes, err := getExpectationChangesForCL(ctx, db, qID)
	if err != nil {
		return skerr.Wrap(err)
	}
	err = storeChangesAsRecordDeltasExpectations(ctx, db, changes, landedTS)
	if err != nil {
		return skerr.Wrap(err)
	}
	if setLanded {
		row := db.QueryRow(ctx, `UPDATE Changelists SET status = 'landed' WHERE changelist_id = $1 RETURNING changelist_id`, qID)
		var s string
		if err := row.Scan(&s); err != nil {
			if err == pgx.ErrNoRows {
				return nil
			}
			return skerr.Wrapf(err, "Updating cl %s to be landed", qID)
		}
	}
	return nil
}

type groupingDigest struct {
	grouping schema.MD5Hash
	digest   schema.MD5Hash
}

type finalState struct {
	labelBefore        schema.ExpectationLabel
	labelAfter         schema.ExpectationLabel
	userWhoTriagedLast string
}

// getExpectationChangesForCL gets all the expectations for the given CL and arranges them in
// temporal order. It de-duplicates any entries (e.g. triaging a digest to positive,then to
// negative, then to positive would be condensed to a single "triage to positive" action). Entries
// are "blamed" to the user who last touched the digest+grouping pair.
func getExpectationChangesForCL(ctx context.Context, db *pgxpool.Pool, qualifiedCLID string) (map[groupingDigest]finalState, error) {
	ctx, span := trace.StartSpan(ctx, "getExpectationChangesForCL")
	defer span.End()
	rows, err := db.Query(ctx, `
SELECT user_name, grouping_id, digest, label_before, label_after
FROM ExpectationRecords JOIN ExpectationDeltas
  ON ExpectationRecords.expectation_record_id = ExpectationDeltas.expectation_record_id
WHERE branch_name = $1
ORDER BY triage_time ASC`, qualifiedCLID)
	if err != nil {
		return nil, skerr.Wrapf(err, "Getting deltas and records for CL %s", qualifiedCLID)
	}
	defer rows.Close()
	// By using a map, we can deduplicate rows and return an object that represents the final
	// state of all the triage logic.
	rv := map[groupingDigest]finalState{}
	for rows.Next() {
		var user string
		var grouping schema.GroupingID
		var digest schema.DigestBytes
		var before schema.ExpectationLabel
		var after schema.ExpectationLabel
		if err := rows.Scan(&user, &grouping, &digest, &before, &after); err != nil {
			return nil, skerr.Wrap(err)
		}
		key := groupingDigest{
			grouping: sql.AsMD5Hash(grouping),
			digest:   sql.AsMD5Hash(digest),
		}
		fs, ok := rv[key]
		if !ok {
			// only update the label before on the first time we see a triage for a grouping.
			fs.labelBefore = before
		}
		fs.labelAfter = after
		fs.userWhoTriagedLast = user
		rv[key] = fs
	}
	return rv, nil
}

// storeChangesAsRecordDeltasExpectations takes the given map and turns them into ExpectationDeltas.
// From there, it is able to make a record per user and store the given deltas and expectations
// according to that record.
func storeChangesAsRecordDeltasExpectations(ctx context.Context, db *pgxpool.Pool, changes map[groupingDigest]finalState, ts time.Time) error {
	ctx, span := trace.StartSpan(ctx, "storeChangesAsRecordDeltasExpectations")
	defer span.End()
	if len(changes) == 0 {
		return nil
	}
	// We want to make one triage record for each user who triaged data on this CL. Those records
	// will represent the final state.
	byUser := map[string][]schema.ExpectationDeltaRow{}
	for gd, fs := range changes {
		if fs.labelBefore == fs.labelAfter {
			continue // skip "no-op" triages, where something was triaged in one way, then undone.
		}
		byUser[fs.userWhoTriagedLast] = append(byUser[fs.userWhoTriagedLast], schema.ExpectationDeltaRow{
			GroupingID:  sql.FromMD5Hash(gd.grouping),
			Digest:      sql.FromMD5Hash(gd.digest),
			LabelBefore: fs.labelBefore,
			LabelAfter:  fs.labelAfter,
		})
	}
	for user, deltas := range byUser {
		if len(deltas) == 0 {
			continue
		}
		recordID := uuid.New()
		// Write the record for this user
		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
INSERT INTO ExpectationRecords (expectation_record_id, user_name, triage_time, num_changes)
VALUES ($1, $2, $3, $4)`, recordID, user, ts, len(deltas))
			return err // Don't wrap - crdbpgx might retry
		})
		if err != nil {
			return skerr.Wrapf(err, "storing record")
		}
		if err := bulkWriteDeltas(ctx, db, recordID, deltas); err != nil {
			return skerr.Wrapf(err, "storing deltas")
		}
		if err := bulkWriteExpectations(ctx, db, recordID, deltas); err != nil {
			return skerr.Wrapf(err, "storing expectations")
		}
	}
	return nil
}

// bulkWriteDeltas stores all the deltas using a batched approach. They are all attributed to the
// provided record id.
func bulkWriteDeltas(ctx context.Context, db *pgxpool.Pool, recordID uuid.UUID, deltas []schema.ExpectationDeltaRow) error {
	ctx, span := trace.StartSpan(ctx, "bulkWriteDeltas")
	defer span.End()
	const chunkSize = 200 // Arbitrarily picked
	err := util.ChunkIter(len(deltas), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := deltas[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO ExpectationDeltas (expectation_record_id, grouping_id, digest,
label_before, label_after) VALUES `
		const valuesPerRow = 5
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, recordID, row.GroupingID, row.Digest, row.LabelBefore, row.LabelAfter)
		}
		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d expectation delta rows", len(deltas))
	}
	return nil
}

// bulkWriteExpectations stores all the expectations using a batched approach. They are all
// attributed to the provided record id.
func bulkWriteExpectations(ctx context.Context, db *pgxpool.Pool, recordID uuid.UUID, deltas []schema.ExpectationDeltaRow) error {
	ctx, span := trace.StartSpan(ctx, "bulkWriteExpectations")
	defer span.End()
	const chunkSize = 200 // Arbitrarily picked
	err := util.ChunkIter(len(deltas), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := deltas[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `UPSERT INTO Expectations (grouping_id, digest, label, expectation_record_id) VALUES `
		const valuesPerRow = 4
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, row.GroupingID, row.Digest, row.LabelAfter, recordID)
		}
		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d expectation rows", len(deltas))
	}
	return nil
}
