package main

import (
	"bytes"
	"flag"
	"math/rand"
	"os"
	"reflect"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
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
	outputFile   = flag.String("output_file", "sample.tile", "Path to the output file for the sample.")
	sampleSize   = flag.Int("sample_size", 0, "Number of random traces to pick. 0 returns the entire tile.")
)

func main() {
	// Load the data that make up the state of the system.
	tile, expectations, ignoreStore := load()

	glog.Infof("Loaded data. Starting to write sample.")
	writeSample(*outputFile, tile, expectations, ignoreStore, *sampleSize)
	glog.Infof("Finished.")
}

// writeSample writes sample to disk.
func writeSample(outputFileName string, tile *tiling.Tile, expectations *expstorage.Expectations, ignoreStore ignore.IgnoreStore, sampleSize int) {
	sample := &serialize.Sample{
		Tile:         tile,
		Expectations: expectations,
	}

	// Get the ignore rules.
	var err error
	if sample.IgnoreRules, err = ignoreStore.List(false); err != nil {
		glog.Fatalf("Error retrieving ignore rules: %s", err)
	}

	if sampleSize > 0 {
		traceIDs := make([]string, 0, len(tile.Traces))
		for id := range tile.Traces {
			traceIDs = append(traceIDs, id)
		}

		permutation := rand.Perm(len(traceIDs))[:util.MinInt(len(traceIDs), sampleSize)]
		newTraces := make(map[string]tiling.Trace, len(traceIDs))
		for _, idx := range permutation {
			newTraces[traceIDs[idx]] = tile.Traces[traceIDs[idx]]
		}
		tile.Traces = newTraces
	}

	// Write the sample to disk.
	var buf bytes.Buffer
	t := timer.New("Writing sample")
	err = sample.Serialize(&buf)
	if err != nil {
		glog.Fatalf("Error serializing tile: %s", err)
	}
	t.Stop()

	file, err := os.Create(outputFileName)
	if err != nil {
		glog.Fatalf("Unable to create file %s:  %s", outputFileName, err)
	}
	outputBuf := buf.Bytes()
	_, err = file.Write(outputBuf)
	if err != nil {
		glog.Fatalf("Writing file %s. Got error: %s", outputFileName, err)
	}
	util.Close(file)

	// Read the sample from disk and do a deep compare.
	t = timer.New("Reading back tile")
	foundSample, err := serialize.DeserializeSample(bytes.NewBuffer(outputBuf))
	if err != nil {
		glog.Fatalf("Error deserializing sample: %s", err)
	}
	t.Stop()

	// Compare the traces to make sure.
	for id, trace := range sample.Tile.Traces {
		foundTrace, ok := foundSample.Tile.Traces[id]
		if !ok {
			glog.Fatalf("Could not find trace with id: %s", id)
		}

		if !reflect.DeepEqual(trace, foundTrace) {
			glog.Fatalf("Traces do not match")
		}
	}

	// Compare the expectaions and ignores
	if !reflect.DeepEqual(sample.Expectations, foundSample.Expectations) {
		glog.Fatalf("Expectations do not match")
	}

	if !reflect.DeepEqual(sample.IgnoreRules, foundSample.IgnoreRules) {
		glog.Fatalf("Ignore rules do not match")
	}

	glog.Infof("File written successfully !")
}

// load retrieves the last tile, the expectations and the ignore store.
func load() (*tiling.Tile, *expstorage.Expectations, ignore.IgnoreStore) {
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
	return masterTileBuilder.GetTile(), expectations, storages.IgnoreStore
}
