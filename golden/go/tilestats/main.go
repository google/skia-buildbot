package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/db"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

var BEGINNING_OF_TIME = time.Date(2014, time.June, 1, 0, 0, 0, 0, time.UTC)

var (
	nCommits     = flag.Int("n_commits", 50, "Number of recent commits to include in the analysis.")
	gitRepoDir   = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL   = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
	traceservice = flag.String("trace_service", "localhost:9001", "The address of the traceservice endpoint.")
)

func main() {
	calcStats()
	// Load the data that make up the state of the system.
	// writeStats(load())
}

type TileStats struct {
	Commits                []*tiling.Commit
	DigestsPerCommit       []int
	UniqueDigestsPerCommit []int
	Digests                int
	UniqueDigests          int
	TotalCells             int
}

func (t *TileStats) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Total Cells               : %d\n", t.TotalCells)
	fmt.Fprintf(&buf, "Total Digests             : %d\n", t.Digests)
	fmt.Fprintf(&buf, "Unique Digests            : %d\n", t.UniqueDigests)
	fmt.Fprintf(&buf, "Per Commit Digests        : ")
	for idx := len(t.Commits) - 1; idx >= 0; idx-- {
		commit := t.Commits[idx]
		fmt.Fprintf(&buf, "%s %s %7d %7d\n", commit.Hash[:10], time.Unix(commit.CommitTime, 0), t.DigestsPerCommit[idx], t.UniqueDigestsPerCommit[idx])
	}
	// intsToString(&buf, t.DigestsPerCommit, 8)
	// fmt.Fprintf(&buf, "Per Commit Unique Digests : ")
	// intsToString(&buf, t.UniqueDigestsPerCommit, 8)
	return buf.String()
}

func intsToString(buf *bytes.Buffer, arr []int, spaces int) {
	fmtStr := fmt.Sprintf("%%%dd ", spaces)
	for _, i := range arr {
		fmt.Fprintf(buf, fmtStr, i)
	}
	buf.WriteString("\n")
}

func getTileStats(tile *tiling.Tile) *TileStats {
	commits := tile.Commits
	uniqueDigestSet := util.StringSet{}
	totalDigests := 0
	digestsPerCommit := make([]int, len(commits))
	perCommitSets := make([]util.StringSet, len(commits))

	for idx := range commits {
		perCommitSets[idx] = util.StringSet{}
	}

	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		for idx, val := range gTrace.Values {
			if val != types.MISSING_DIGEST {
				uniqueDigestSet[val] = true
				totalDigests++
				digestsPerCommit[idx]++
				perCommitSets[idx][val] = true
			}
		}
	}

	uniquesPerCommit := make([]int, len(commits))
	for idx, digestSet := range perCommitSets {
		uniquesPerCommit[idx] = len(digestSet)
	}

	return &TileStats{
		Commits:                tile.Commits,
		DigestsPerCommit:       digestsPerCommit,
		UniqueDigestsPerCommit: uniquesPerCommit,
		Digests:                totalDigests,
		UniqueDigests:          len(uniqueDigestSet),
		TotalCells:             len(commits) * len(tile.Traces),
	}
}

func calcStats() {
	// Set up flags and the database.
	dbConf := database.ConfigFromFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_ROOT, db.PROD_DB_NAME, db.MigrationSteps())
	common.Init()
	sklog.Infof("Init completed.")

	// Open the database
	vdb, err := dbConf.NewVersionedDB()
	if err != nil {
		sklog.Fatal(err)
	}

	if !vdb.IsLatestVersion() {
		sklog.Fatal("Wrong DB version. Please updated to latest version.")
	}
	sklog.Infof("Database initialized.")

	// evt := eventbus.New(nil)
	// expStore := expstorage.NewCachingExpectationStore(expstorage.NewSQLExpectationStore(vdb), evt)
	// sklog.Infof("Expectations loaded.")

	// Check out the repository.
	git, err := gitinfo.CloneOrUpdate(*gitRepoURL, *gitRepoDir, false)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Git repo cloned/updated.")

	// Open the tracedb and load the latest tile.
	// Connect to traceDB and create the builders.
	tdb, err := tracedb.NewTraceServiceDBFromAddress(*traceservice, types.GoldenTraceBuilder)
	if err != nil {
		sklog.Fatalf("Failed to connect to tracedb: %s", err)
	}
	sklog.Infof("Tracedb opened.")

	commitHashes := git.From(BEGINNING_OF_TIME)
	increments := 50
	commitIDs := make([]*tracedb.CommitID, 0, increments)

	for idx := len(commitHashes); idx > 0; idx -= increments {
		hashes := commitHashes[util.MaxInt(0, idx-increments):idx]
		fmt.Printf("ITERATION: %d - %d\n", idx-increments, idx)
		commitIDs = commitIDs[:0]

		for _, hash := range hashes {
			info, err := git.Details(hash, true)
			if err != nil {
				sklog.Fatalf("Error retrieving details: %s", err)
			}
			if !info.Branches["master"] {
				continue
			}
			commitIDs = append(commitIDs, getCommitID(info))
			// fmt.Printf("%s %s %s %s, %v\n", info.Timestamp, info.Hash[:12], info.Author, info.Subject, info.Branches)
		}

		tile, _, err := tdb.TileFromCommits(commitIDs)
		if err != nil {
			sklog.Fatalf("Unable to load tile from commits. Got error: %s", err)
		}
		tileStats := getTileStats(tile)
		fmt.Fprintf(os.Stdout, "======================================================\n")
		fmt.Fprintf(os.Stdout, "%s\n", tileStats.String())
		tile = nil
		runtime.GC()
		if idx < (len(commitHashes) - 48) {
			break
		}
	}
}

func getCommitID(lc *vcsinfo.LongCommit) *tracedb.CommitID {
	branch := util.StringSet(lc.Branches).Keys()[0]
	return &tracedb.CommitID{
		Timestamp: lc.Timestamp.Unix(),
		ID:        lc.Hash,
		Source:    branch,
	}
}

func writeStats(tilePair *types.TilePair, exps *expstorage.Expectations, ignoreStore ignore.IgnoreStore) {
	stats := getTileStats(tilePair.Tile)
	fmt.Printf("Stats for filtered Tile\n%s", stats)

	stats = getTileStats(tilePair.TileWithIgnores)
	fmt.Printf("\n\nStats for unfiltered Tile\n%s", stats)
}

// load retrieves the last tile, the expectations and the ignore store.
func load() (*types.TilePair, *expstorage.Expectations, ignore.IgnoreStore) {
	// Set up flags and the database.
	dbConf := database.ConfigFromFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_ROOT, db.PROD_DB_NAME, db.MigrationSteps())
	common.Init()
	sklog.Infof("Init completed.")

	// Open the database
	vdb, err := dbConf.NewVersionedDB()
	if err != nil {
		sklog.Fatal(err)
	}

	if !vdb.IsLatestVersion() {
		sklog.Fatal("Wrong DB version. Please updated to latest version.")
	}
	sklog.Infof("Database initialized.")

	evt := eventbus.New(nil)
	expStore := expstorage.NewCachingExpectationStore(expstorage.NewSQLExpectationStore(vdb), evt)
	sklog.Infof("Expectations loaded.")

	// Check out the repository.
	git, err := gitinfo.CloneOrUpdate(*gitRepoURL, *gitRepoDir, false)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Git repo cloned/updated.")

	// Open the tracedb and load the latest tile.
	// Connect to traceDB and create the builders.
	tdb, err := tracedb.NewTraceServiceDBFromAddress(*traceservice, types.GoldenTraceBuilder)
	if err != nil {
		sklog.Fatalf("Failed to connect to tracedb: %s", err)
	}
	sklog.Infof("Tracedb opened.")

	masterTileBuilder, err := tracedb.NewMasterTileBuilder(tdb, git, *nCommits, evt)
	if err != nil {
		sklog.Fatalf("Failed to build trace/db.DB: %s", err)
	}
	sklog.Infof("Tilebuilder created.")

	storages := &storage.Storage{
		ExpectationsStore: expStore,
		MasterTileBuilder: masterTileBuilder,
		NCommits:          *nCommits,
		EventBus:          evt,
	}

	storages.IgnoreStore = ignore.NewSQLIgnoreStore(vdb, expStore, storages.GetTileStreamNow(time.Minute*20))
	sklog.Infof("Ignore store created.")

	expectations, err := expStore.Get()
	if err != nil {
		sklog.Fatalf("Unable to get expecations: %s", err)
	}
	sklog.Infof("Expecationstore created.")

	tilePair, err := storages.GetLastTileTrimmed()
	if err != nil {
		sklog.Fatalf("Unable to retrieve tile pair. Got error: %s", err)
	}
	sklog.Infof("Tilepair loaded.")

	return tilePair, expectations, storages.IgnoreStore
}
