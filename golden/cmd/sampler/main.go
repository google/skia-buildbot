package main

import (
	"bytes"
	"context"
	"flag"
	"math/rand"
	"net/url"
	"os"
	"reflect"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
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
	dsNamespace  = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	gitRepoDir   = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL   = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
	nCommits     = flag.Int("n_commits", 50, "Number of recent commits to include in the analysis.")
	nTests       = flag.Int("n_tests", 0, "Set number of tests to pick randomly.")
	outputFile   = flag.String("output_file", "sample.tile", "Path to the output file for the sample.")
	query        = flag.String("query", "", "Query to filter which traces are considered.")
	sampleSize   = flag.Int("sample_size", 0, "Number of random traces to pick. 0 returns the entire tile.")
	traceservice = flag.String("trace_service", "localhost:9001", "The address of the traceservice endpoint.")
)

func main() {
	// Load the data that make up the state of the system.
	tile, expectations, ignoreStore := load(context.Background(), *dsNamespace)
	tile = sampleTile(tile, *sampleSize, *query, *nTests)
	writeSample(*outputFile, tile, expectations, ignoreStore)
	sklog.Infof("Finished.")
}

func sampleTile(tile *tiling.Tile, sampleSize int, queryStr string, nTests int) *tiling.Tile {
	// Filter the traces if there was a query defined.
	if queryStr != "" {
		query, err := url.ParseQuery(queryStr)
		if err != nil {
			sklog.Fatalf("Unable to parse querye '%s'. Got error: %s", queryStr, err)
		}

		newTraces := map[string]tiling.Trace{}
		for traceID, trace := range tile.Traces {
			if tiling.Matches(trace, query) {
				newTraces[traceID] = trace
			}
		}
		tile.Traces = newTraces
	}

	// Fixed number of tests selected.
	if nTests > 0 {
		byTest := map[string][]string{}
		for traceID, trace := range tile.Traces {
			name := trace.Params()[types.PRIMARY_KEY_FIELD]
			byTest[name] = append(byTest[name], traceID)
		}

		newTraces := map[string]tiling.Trace{}
		idx := 0
		for testName, traceIDs := range byTest {
			for _, traceID := range traceIDs {
				newTraces[traceID] = tile.Traces[traceID]
			}
			sklog.Infof("Included test/traces: %s/%d", testName, len(traceIDs))
			idx++
			if idx >= nTests {
				break
			}
		}
		tile.Traces = newTraces
	} else if sampleSize > 0 {
		// Sample a given number of traces.
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

	return tile
}

// writeSample writes sample to disk.
func writeSample(outputFileName string, tile *tiling.Tile, expectations types.Expectations, ignoreStore ignore.IgnoreStore) {
	sample := &serialize.Sample{
		Tile:         tile,
		Expectations: expectations,
	}

	// Get the ignore rules.
	var err error
	if sample.IgnoreRules, err = ignoreStore.List(false); err != nil {
		sklog.Fatalf("Error retrieving ignore rules: %s", err)
	}

	// Write the sample to disk.
	var buf bytes.Buffer
	t := timer.New("Writing sample")
	err = sample.Serialize(&buf)
	if err != nil {
		sklog.Fatalf("Error serializing tile: %s", err)
	}
	t.Stop()

	file, err := os.Create(outputFileName)
	if err != nil {
		sklog.Fatalf("Unable to create file %s:  %s", outputFileName, err)
	}
	outputBuf := buf.Bytes()
	_, err = file.Write(outputBuf)
	if err != nil {
		sklog.Fatalf("Writing file %s. Got error: %s", outputFileName, err)
	}
	util.Close(file)

	// Read the sample from disk and do a deep compare.
	t = timer.New("Reading back tile")
	foundSample, err := serialize.DeserializeSample(bytes.NewBuffer(outputBuf))
	if err != nil {
		sklog.Fatalf("Error deserializing sample: %s", err)
	}
	t.Stop()

	// Compare the traces to make sure.
	for id, trace := range sample.Tile.Traces {
		foundTrace, ok := foundSample.Tile.Traces[id]
		if !ok {
			sklog.Fatalf("Could not find trace with id: %s", id)
		}

		if !reflect.DeepEqual(trace, foundTrace) {
			sklog.Fatalf("Traces do not match")
		}
	}

	// Compare the expectations and ignores
	if !reflect.DeepEqual(sample.Expectations, foundSample.Expectations) {
		sklog.Fatalf("Expectations do not match")
	}

	if !reflect.DeepEqual(sample.IgnoreRules, foundSample.IgnoreRules) {
		sklog.Fatalf("Ignore rules do not match")
	}

	sklog.Infof("File written successfully !")
}

// load retrieves the last tile, the expectations and the ignore store.
func load(ctx context.Context, dsNamespace string) (*tiling.Tile, types.Expectations, ignore.IgnoreStore) {
	// Set up flags and the database.
	dbConf := database.ConfigFromFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_ROOT, db.PROD_DB_NAME, db.MigrationSteps())
	common.Init()

	evt := eventbus.New()
	var expStore expstorage.ExpectationsStore
	var vdb *database.VersionedDB
	var err error

	if dsNamespace != "" {
		expStore, _, err = expstorage.NewCloudExpectationsStore(ds.DS, evt)
		if err != nil {
			sklog.Fatalf("Unable to create cloud expectations store: %s", err)
		}
	} else {
		// Open the database
		vdb, err = dbConf.NewVersionedDB()
		if err != nil {
			sklog.Fatal(err)
		}

		if !vdb.IsLatestVersion() {
			sklog.Fatal("Wrong DB version. Please updated to latest version.")
		}

		expStore = expstorage.NewCachingExpectationStore(expstorage.NewSQLExpectationStore(vdb), evt)
	}

	// Check out the repository.
	git, err := gitinfo.CloneOrUpdate(ctx, *gitRepoURL, *gitRepoDir, false)
	if err != nil {
		sklog.Fatal(err)
	}

	// Open the tracedb and load the latest tile.
	// Connect to traceDB and create the builders.
	tdb, err := tracedb.NewTraceServiceDBFromAddress(*traceservice, types.GoldenTraceBuilder)
	if err != nil {
		sklog.Fatalf("Failed to connect to tracedb: %s", err)
	}

	masterTileBuilder, err := tracedb.NewMasterTileBuilder(ctx, tdb, git, *nCommits, evt, "")
	if err != nil {
		sklog.Fatalf("Failed to build trace/db.DB: %s", err)
	}

	storages := &storage.Storage{
		ExpectationsStore: expStore,
		MasterTileBuilder: masterTileBuilder,
		NCommits:          *nCommits,
		EventBus:          evt,
	}

	if dsNamespace != "" {
		storages.IgnoreStore, err = ignore.NewCloudIgnoreStore(ds.DS, expStore, storages.GetTileStreamNow(time.Minute*20))
		if err != nil {
			sklog.Fatalf("Unable to create cloud ignorestore: %s", err)
		}
	} else {
		storages.IgnoreStore = ignore.NewSQLIgnoreStore(vdb, expStore, storages.GetTileStreamNow(time.Minute*20))
	}

	expectations, err := expStore.Get()
	if err != nil {
		sklog.Fatalf("Unable to get expectations: %s", err)
	}
	return masterTileBuilder.GetTile(), expectations, storages.IgnoreStore
}
