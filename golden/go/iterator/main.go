package main

import (
	"flag"
	"os"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/golden/go/db"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

var (
	nCommits     = flag.Int("n_commits", 50, "Number of recent commits to include in the analysis.")
	gitRepoDir   = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL   = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
	traceservice = flag.String("trace_service", "localhost:9001", "The address of the traceservice endpoint.")
	outputDir    = flag.String("output_dir", "tiles_tests", "Path to the output file for the sample.")
)

func main() {
	// Load the data that make up the state of the system.
	tile, _, _ := load()
	writeTile(tile, *outputDir)
	sklog.Infof("Finished.")
}

func writeTile(tile *tiling.Tile, outputDir string) error {
	outputDir, err := fileutil.EnsureDirExists(outputDir)
	if err != nil {
		sklog.Fatalf("Unable to make sure the directory exists: %s", err)
	}

	// Iterate over the tile and extract the tests.
	tilesByTest := map[string]*tiling.Tile{}
	for traceId, trace := range tile.Traces {
		p := trace.Params()
		testName := p[types.PRIMARY_KEY_FIELD]
		corpus := p[types.CORPUS_FIELD]
		k := testName + "---" + corpus
		targetTile, ok := tilesByTest[k]
		if !ok {
			targetTile = tiling.NewTile()
			targetTile.Commits = tile.Commits
		}
		targetTile.Traces[traceId] = trace
		paramtools.ParamSet(targetTile.ParamSet).AddParams(p)
		tilesByTest[k] = targetTile
	}

	// Iterate over each test and assemble the tile.
	for testId, testTile := range tilesByTest {
		outputFname := filepath.Join(outputDir, testId)

		file, err := os.Create(outputFname)
		if err != nil {
			sklog.Fatalf("Error opening file %s: %s", outputFname, err)
		}

		if err := serialize.SerializeTile(file, testTile); err != nil {
			sklog.Fatalf("Error writing file %s: %s", outputFname, err)
		}
		file.Close()
	}
	return nil
}

// load retrieves the last tile, the expectations and the ignore store.
func load() (*tiling.Tile, *expstorage.Expectations, ignore.IgnoreStore) {
	// Set up flags and the database.
	dbConf := database.ConfigFromFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_ROOT, db.PROD_DB_NAME, db.MigrationSteps())
	common.Init()

	// Open the database
	vdb, err := dbConf.NewVersionedDB()
	if err != nil {
		sklog.Fatal(err)
	}

	if !vdb.IsLatestVersion() {
		sklog.Fatal("Wrong DB version. Please updated to latest version.")
	}

	evt := eventbus.New()
	expStore := expstorage.NewCachingExpectationStore(expstorage.NewSQLExpectationStore(vdb), evt)

	// Check out the repository.
	git, err := gitinfo.CloneOrUpdate(*gitRepoURL, *gitRepoDir, false)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Opening trace DB")
	// Open the tracedb and load the latest tile.
	// Connect to traceDB and create the builders.
	tdb, err := tracedb.NewTraceServiceDBFromAddress(*traceservice, types.GoldenTraceBuilder)
	if err != nil {
		sklog.Fatalf("Failed to connect to tracedb: %s", err)
	}

	sklog.Infof("Creating MasterTileBuilder")
	masterTileBuilder, err := tracedb.NewMasterTileBuilder(tdb, git, *nCommits, evt)
	if err != nil {
		sklog.Fatalf("Failed to build trace/db.DB: %s", err)
	}

	storages := &storage.Storage{
		ExpectationsStore: expStore,
		MasterTileBuilder: masterTileBuilder,
		NCommits:          *nCommits,
		EventBus:          evt,
	}

	sklog.Infof("Creating Ignore Store")
	storages.IgnoreStore = ignore.NewSQLIgnoreStore(vdb, expStore, storages.GetTileStreamNow(time.Minute*20))

	expectations, err := expStore.Get()
	if err != nil {
		sklog.Fatalf("Unable to get expecations: %s", err)
	}

	sklog.Infof("Loading Tile")
	tile := masterTileBuilder.GetTile()
	sklog.Infof("Done loading tile.")

	return tile, expectations, storages.IgnoreStore
}
