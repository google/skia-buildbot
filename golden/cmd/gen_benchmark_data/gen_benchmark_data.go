package main

import (
	"context"
	"crypto/md5"
	"flag"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/timer"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

const (
	commitOffset      = 40000
	numTests          = 3
	maxSQLConnections = 8
)

func main() {
	rand.Seed(time.Now().UnixNano())
	local := flag.Bool("local", true, "Spin up a local instance of cockroachdb. If false, will connect to local port 26257.")
	port := flag.String("port", "26257", "Port on localhost to connect to. Only set if --local=false")
	profileFile := flag.String("profile_file", "", "File to write profile data to.")

	flag.Parse()
	if *local {
		storageDir, err := sqltest.StartLocalCockroachDB()
		if err != nil {
			sklog.Fatalf("Could not start local cockroachdb: %s", err)
		}
		sklog.Infof("Check out localhost:8080 and %s for storage", storageDir)
	} else {
		sklog.Infof("Not using local db. Make sure you ran kubectl port-forward gold-cockroachdb-0 26234:26234")
	}

	if err := createBenchmarkDBAndTables(*port); err != nil {
		sklog.Fatalf("Could not initialize db/tables: %s", err)
	}

	ctx := context.Background()
	conf, err := pgxpool.ParseConfig("postgresql://root@localhost:" + *port + "/benchmark_db?sslmode=disable")
	if err != nil {
		sklog.Fatalf("error getting postgress config: %s", err)
	}

	conf.MaxConns = maxSQLConnections
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	defer db.Close()

	if *profileFile != "" {
		f, err := os.Create(*profileFile)
		if err != nil {
			sklog.Fatalf("could not create CPU profile: ", err)
		}
		defer f.Close()
		runtime.SetCPUProfileRate(100)
		if err := pprof.StartCPUProfile(f); err != nil {
			sklog.Fatalf("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	defer timer.New("filling data").Stop()
	if err := fillData(ctx, db, commitOffset); err != nil {
		sklog.Fatalf("Could not fill with data: %s", err)
	}

	sklog.Infof("Done.")
}

func createBenchmarkDBAndTables(port string) error {
	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
		`--execute=DROP DATABASE IF EXISTS benchmark_db;`).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "dropping previous database: %s", out)
	}

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
		`--execute=CREATE DATABASE IF NOT EXISTS benchmark_db;`).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating database: %s", out)
	}

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
		"--database=benchmark_db", // Connect to benchmark_db that we just made
		"--execute="+sql.CockroachDBSchema,
	).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating tables: %s", out)
	}
	sklog.Infof("benchmark DB setup")
	return nil
}

func fillData(ctx context.Context, db *pgxpool.Pool, commitOffset int) error {
	eg, errCtx := errgroup.WithContext(ctx)

	for i := 0; i < numTests; i++ {
		fakeFile := "gs://skia-gold-benchmark/dm-json-v1/2020/10/31/13/dm" + strconv.Itoa(rand.Int()) + ".json"
		tracePairs := generateTracesForTest(randomTestSettings{
			Corpus:               "gm",
			TestName:             "blend_modes_" + strconv.Itoa(rand.Int()),
			NumCommits:           1000,
			MinAdditionalKeys:    8,
			MaxAdditionalKeys:    11,
			MaxAdditionalOptions: 4,
			MinTraceDensity:      0.2,
			MaxTraceDensity:      0.99,
			NumTraces:            50,
			MinTraceFlakiness:    2,
			MaxTraceFlakiness:    20,
			TraceDigestOverlap:   0.20,
			GlobalDigestOverlap:  0.05,
		})
		if err := storePrimaryBranchTraceData(errCtx, db, eg, tracePairs, fakeFile, commitOffset); err != nil {
			return skerr.Wrap(err)
		}
	}

	if err := eg.Wait(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func storePrimaryBranchTraceData(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, pairs []tiling.TracePair, sourceFile string, commitOffset int) error {
	sfh := md5.Sum([]byte(sourceFile))
	sourceFileHash := sfh[:] // convert array to slice

	tracesToStore, optsToStore, groupingsToStore, paramset := storeToTraceValues(ctx, db, eg, pairs, sourceFileHash, commitOffset)

	storeToKeyValueTable(ctx, db, eg, insertTraces, tracesToStore)
	storeToKeyValueTable(ctx, db, eg, insertOptions, optsToStore)
	storeToKeyValueTable(ctx, db, eg, insertGroupings, groupingsToStore)
	storeToPrimaryBranchParams(ctx, db, eg, paramset, commitOffset, commitOffset+len(pairs[0].Trace.Digests))

	sklog.Infof("TODO values at head, source files, expectations")

	return nil
}

func storeToTraceValues(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, pairs []tiling.TracePair, sourceFileHash []byte, commitOffset int) (map[string]string, map[string]string, map[string]string, paramtools.ParamSet) {
	// Actual max is 65535. Having a really large batch size has not been problematic yet, although
	// it can be if there is a lot of DB contention.
	const maxPlaceholders = 65000
	numCommits := pairs[0].Trace.Len()
	const traceValuesStatement = `INSERT INTO TraceValues (trace_id, shard, commit_id, grouping_id, digest, options_id, source_file_id) VALUES `
	const valuesPerRow = 7

	arguments := make([]interface{}, 0, maxPlaceholders)
	writeCurrentValues := func() {
		if len(arguments) == 0 {
			return
		}
		argCopy := make([]interface{}, len(arguments))
		copy(argCopy, arguments)
		arguments = arguments[:0]
		eg.Go(func() error {
			sklog.Infof("Inserting %d rows to TraceValues", len(argCopy)/valuesPerRow)
			statement := traceValuesStatement + sql.ValuesPlaceholders(valuesPerRow, len(argCopy)/valuesPerRow)
			statement += " RETURNING NOTHING"
			_, err := db.Exec(ctx, statement, argCopy...)
			if err != nil {
				return skerr.Wrapf(err, "inserting TraceValues")
			}
			return nil
		})
	}

	keysToStore := map[string]string{}      // maps string(traceHash) => keysJSON
	optsToStore := map[string]string{}      // maps string(optsHash) => optsJSON
	groupingsToStore := map[string]string{} // maps string(groupingHash) => groupingJSON
	paramset := paramtools.ParamSet{}

	for _, tp := range pairs {
		// Check to see if we have enough space
		if len(arguments)+numCommits*valuesPerRow > maxPlaceholders {
			writeCurrentValues()
		}
		keysJSON, traceHash := sql.SerializeMap(tp.Trace.Keys())
		keysToStore[string(traceHash)] = keysJSON
		optsJSON, optsHash := sql.SerializeMap(tp.Trace.Options())
		optsToStore[string(optsHash)] = optsJSON
		grouping := groupingFor(tp.Trace.Keys())
		groupingJSON, groupingHash := sql.SerializeMap(grouping)
		groupingsToStore[string(groupingHash)] = groupingJSON
		paramset.AddParams(tp.Trace.KeysAndOptions())
		// Add arguments for all digests in this trace.
		for i := range tp.Trace.Digests {
			digest := tp.Trace.Digests[i]
			if digest == tiling.MissingDigest {
				continue
			}
			digestBytes, err := sql.DigestToBytes(digest)
			if err != nil {
				panic(err)
			}
			arguments = append(arguments, traceHash, sql.ComputeTraceValueShard(traceHash), commitOffset+i,
				groupingHash, digestBytes, optsHash, sourceFileHash)
		}
	}
	writeCurrentValues()
	return keysToStore, optsToStore, groupingsToStore, paramset
}

const insertTraces = `INSERT INTO Traces (trace_id, keys) VALUES `
const insertGroupings = `INSERT INTO Groupings (grouping_id, keys) VALUES `
const insertOptions = `INSERT INTO Options (options_id, keys) VALUES `

func storeToKeyValueTable(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, insert string, toCreate map[string]string) {
	if len(toCreate) == 0 {
		return
	}
	const maxPlaceholders = 65000
	const valuesPerRow = 2

	arguments := make([]interface{}, 0, maxPlaceholders)
	writeCurrentValues := func() {
		if len(arguments) == 0 {
			return
		}
		argCopy := make([]interface{}, len(arguments))
		copy(argCopy, arguments)
		arguments = arguments[:0]
		eg.Go(func() error {
			sklog.Infof("Inserting %d keys", len(argCopy)/valuesPerRow)
			statement := insert + sql.ValuesPlaceholders(valuesPerRow, len(argCopy)/valuesPerRow)
			// ON CONFLICT DO NOTHING because if the rows already exist, then the data we are writing
			// is immutable.
			statement += ` ON CONFLICT DO NOTHING;`
			_, err := db.Exec(ctx, statement, argCopy...)
			if err != nil {
				return skerr.Wrapf(err, "inserting to table %s", insert)
			}
			return nil
		})
	}

	for id, json := range toCreate {
		if len(arguments) > maxPlaceholders {
			writeCurrentValues()
		}
		arguments = append(arguments, []byte(id), json)
	}
	writeCurrentValues()
}

// storeToPrimaryBranchParams writes the given params as if they were seen on every commit in
// the given range. This is potentially more data than would actually be in production, especially
// for sparse repos.
func storeToPrimaryBranchParams(ctx context.Context, db *pgxpool.Pool, eg *errgroup.Group, paramset paramtools.ParamSet, startCommit, endCommit int) {
	const maxPlaceholders = 1000
	const primaryBranchParamsStatement = `INSERT INTO PrimaryBranchParams (key, value, commit_id) VALUES `
	const valuesPerRow = 3

	spew.Dump(paramset)

	arguments := make([]interface{}, 0, maxPlaceholders)
	writeCurrentValues := func() {
		if len(arguments) == 0 {
			return
		}
		argCopy := make([]interface{}, len(arguments))
		copy(argCopy, arguments)
		arguments = arguments[:0]
		eg.Go(func() error {
			statement := primaryBranchParamsStatement + sql.ValuesPlaceholders(valuesPerRow, len(argCopy)/valuesPerRow)
			// ON CONFLICT DO NOTHING because the rows are immutable once written.
			statement += ` ON CONFLICT DO NOTHING;`
			_, err := db.Exec(ctx, statement, argCopy...)
			if err != nil {
				return skerr.Wrapf(err, "inserting TraceValues")
			}
			return nil
		})
	}

	for commit := startCommit; commit < endCommit; commit++ {
		for key, values := range paramset {
			if len(arguments)+valuesPerRow*len(values) > maxPlaceholders {
				writeCurrentValues()
			}
			for _, value := range values {
				arguments = append(arguments, key, value, commit)
			}
		}
	}
	writeCurrentValues()

}

func groupingFor(keys map[string]string) map[string]string {
	return map[string]string{
		types.CorpusField:     keys[types.CorpusField],
		types.PrimaryKeyField: keys[types.PrimaryKeyField],
	}
}
