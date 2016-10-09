package main

import (
	"bytes"
	"flag"
	"fmt"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/db"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

var (
	nCommits     = flag.Int("n_commits", 50, "Number of recent commits to include in the analysis.")
	gitRepoDir   = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL   = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
	traceservice = flag.String("trace_service", "localhost:9001", "The address of the traceservice endpoint.")
)

func main() {
	// Load the data that make up the state of the system.
	writeStats(load())
}

type TileStats struct {
	DigestsPerCommit       []int
	UniqueDigestsPerCommit []int
	Digests                int
	UniqueDigests          int
	TotalCells             int
}

func (t *TileStats) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Total Cells  : %d\n", t.TotalCells))
	buf.WriteString(fmt.Sprintf("Total Digests: %d\n", t.Digests))
	buf.WriteString(fmt.Sprintf("Unique Digests: %d\n", t.Digests))
	buf.WriteString("Per Commit Digests: ")
	for _, count := range t.DigestsPerCommit {
		buf.WriteString(fmt.Sprintf("%6d", count))
	}
	buf.WriteString("\n")

	buf.WriteString("Per Commit Unique Digests:")
	for _, count := range t.UniqueDigestsPerCommit {
		buf.WriteString(fmt.Sprintf("%6d", count))
	}
	buf.WriteString("\n")

	return buf.String()
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
		DigestsPerCommit:       digestsPerCommit,
		UniqueDigestsPerCommit: uniquesPerCommit,
		Digests:                totalDigests,
		UniqueDigests:          len(uniqueDigestSet),
		TotalCells:             len(commits) * len(tile.Traces),
	}
}

func writeStats(tilePair *types.TilePair, exps *expstorage.Expectations, ignoreStore ignore.IgnoreStore) {
	stats := getTileStats(tilePair.Tile)
	fmt.Printf("Stats for filtered Tile\n%s", stats)
}

// load retrieves the last tile, the expectations and the ignore store.
func load() (*types.TilePair, *expstorage.Expectations, ignore.IgnoreStore) {
	// Set up flags and the database.
	dbConf := database.ConfigFromFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_ROOT, db.PROD_DB_NAME, db.MigrationSteps())
	common.Init()

	// Open the database
	vdb, err := dbConf.NewVersionedDB()
	if err != nil {
		glog.Fatal(err)
	}

	if !vdb.IsLatestVersion() {
		glog.Fatal("Wrong DB version. Please updated to latest version.")
	}

	evt := eventbus.New(nil)
	expStore := expstorage.NewCachingExpectationStore(expstorage.NewSQLExpectationStore(vdb), evt)

	// Check out the repository.
	git, err := gitinfo.CloneOrUpdate(*gitRepoURL, *gitRepoDir, false)
	if err != nil {
		glog.Fatal(err)
	}

	// Open the tracedb and load the latest tile.
	// Connect to traceDB and create the builders.
	tdb, err := tracedb.NewTraceServiceDBFromAddress(*traceservice, types.GoldenTraceBuilder)
	if err != nil {
		glog.Fatalf("Failed to connect to tracedb: %s", err)
	}

	masterTileBuilder, err := tracedb.NewMasterTileBuilder(tdb, git, *nCommits, evt)
	if err != nil {
		glog.Fatalf("Failed to build trace/db.DB: %s", err)
	}

	storages := &storage.Storage{
		ExpectationsStore: expStore,
		MasterTileBuilder: masterTileBuilder,
		NCommits:          *nCommits,
		EventBus:          evt,
	}

	storages.IgnoreStore = ignore.NewSQLIgnoreStore(vdb, expStore, storages.GetTileStreamNow(time.Minute*20))

	expectations, err := expStore.Get()
	if err != nil {
		glog.Fatalf("Unable to get expecations: %s", err)
	}

	tilePair, err := storages.GetLastTileTrimmed()
	if err != nil {
		glog.Fatalf("Unable to retrieve tile pair. Got error: %s", err)
	}

	return tilePair, expectations, storages.IgnoreStore
}
