package ingestion_processors

import (
	"context"
	"crypto/md5"
	"net/http"
	"sort"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/sql"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Configuration option that identifies a tracestore backed by BigTable.
	sqlGoldIngester = "gold_sql"

	sqlConnectionURL = "SQLConnectionURL"

	intentionallyReturnError = "testing_only_fail_ingestion"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(sqlGoldIngester, newSQLProcessor)
}

func newSQLProcessor(ctx context.Context, _ vcsinfo.VCS, cfg ingestion.Config, _ *http.Client) (ingestion.Processor, error) {
	// example: "postgresql://root@gold-cockroachdb-public:26234/demo_gold_db?sslmode=disable"
	dbConnectionURL := cfg.ExtraParams[sqlConnectionURL]
	sqlConfig, err := pgxpool.ParseConfig(dbConnectionURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "Getting configuration for the database %s", dbConnectionURL)
	}

	sqlConfig.MaxConns = ingestion.NConcurrentProcessors
	db, err := pgxpool.ConnectConfig(ctx, sqlConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "connecting to the database with config %#v", cfg)
	}

	c, err := db.Acquire(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "acquiring connection to %s", dbConnectionURL)
	}
	defer c.Release()
	if err = c.Conn().Ping(ctx); err != nil {
		return nil, skerr.Wrapf(err, "connecting to database via ping %s", dbConnectionURL)
	}
	cnc, err := lru.New(1000) // ~40 bytes per entry = 40k
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// 10 million traces/options should be sufficient to avoid many unnecessary puts to the Traces,
	// Groupings, and Options tables
	koc, err := lru.New(10_000_000) // ~20 bytes per entry = 200M
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &sqlProcessor{
		db:             db,
		commitNumCache: cnc, // maps GitHash => int (commitNumber)
		keyValueCache:  koc, // maps md5hash(map) => bool if it has been stored
		traceCounter:   metrics2.GetCounter("gold_traces_ingested"),
		now:            time.Now,
	}, nil
}

type sqlProcessor struct {
	db             *pgxpool.Pool // The standard *pgx.Conn is *not* thread safe.
	commitNumCache *lru.Cache
	keyValueCache  *lru.Cache
	traceCounter   metrics2.Counter

	now func() time.Time
}

// sqlExecutor lets us use either *pgxpool.Pool or a transaction it returns to run our queries.
type sqlExecutor interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// readOnlyCache ensures that we don't write to our cache until we are sure a given file's data
// has been ingested.
type readOnlyCache interface {
	Contains(key interface{}) bool
}

func (s *sqlProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	defer metrics2.FuncTimer().Stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	fileName := resultsFile.Name()
	gr, err := processGoldResults(ctx, resultsFile)
	if err != nil {
		return skerr.Wrapf(err, "could not process results file %s", fileName)
	}

	if len(gr.Results) == 0 {
		sklog.Infof("ignoring file %s because it has no results", fileName)
		return ingestion.IgnoreResultsFileErr
	}

	commitID, err := s.getCLNumber(ctx, gr.GitHash)
	if err != nil {
		return skerr.Wrapf(err, "could not determine branch for %s", gr.GitHash)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Starting transaction while ingesting %s", fileName)
	}
	// Rollback is safe to call even if the tx is already closed, so if
	// the tx commits successfully, this is a no-op
	defer tx.Rollback(ctx)

	// Know that we know this is a valid commit, let's store the data.
	sourceFileHash, err := createSourceFile(ctx, tx, fileName, s.now())
	if err != nil {
		return skerr.Wrapf(err, "storing source file metadata for %s", fileName)
	}

	toStore, newCacheEntries, err := storeKeysOptionsAndFlatten(ctx, tx, s.keyValueCache, gr, commitID)
	if err != nil {
		return skerr.Wrapf(err, "storing keys and options for %s", fileName)
	}

	defer shared.NewMetricsTimer("store_sql_values").Stop()
	if err := storeValues(ctx, tx, toStore, commitID, sourceFileHash); err != nil {
		return skerr.Wrapf(err, "storing %d values from %s", len(toStore), fileName)
	}

	if v := ctx.Value(intentionallyReturnError); v != nil {
		return skerr.Fmt("context said to fail (for testing) so we fail while ingesting %s", fileName)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Committing transaction while ingesting %s", fileName)
	}
	// Now that we are sure the file was ingested, we can update the shared cache of key/value maps
	// so the future ingestions will go faster (since they don't need to store the immutable data
	// in those caches.
	for hashAsString := range newCacheEntries {
		s.keyValueCache.Add(hashAsString, true)
	}
	s.traceCounter.Inc(int64(len(toStore)))
	return nil
}

type sqlTraceValue struct {
	traceID     []byte
	digestBytes []byte
	optionsID   []byte
	groupingID  []byte
}

const selectCommitNumberFromGitHash = `SELECT commit_id FROM Commits WHERE git_hash=$1 LIMIT 1`

func (s *sqlProcessor) getCLNumber(ctx context.Context, hash string) (int, error) {
	if num, ok := s.commitNumCache.Get(hash); ok {
		return num.(int), nil
	}

	var commitNum int32
	row := s.db.QueryRow(ctx, selectCommitNumberFromGitHash, hash)
	if err := row.Scan(&commitNum); err != nil {
		return 0, skerr.Wrapf(err, "getting number for %s", hash)
	}
	s.commitNumCache.Add(hash, int(commitNum))
	return int(commitNum), nil
}

const upsertSourceFile = `UPSERT INTO SourceFiles (source_file_id, source_file, last_ingested) VALUES ($1, $2, $3)`

func createSourceFile(ctx context.Context, db sqlExecutor, pathToFile string, now time.Time) ([]byte, error) {
	sourceFileHash := md5.Sum([]byte(pathToFile))
	_, err := db.Exec(ctx, upsertSourceFile, sourceFileHash[:], pathToFile, now)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return sourceFileHash[:], nil
}

type jsonAndHash struct {
	json string
	hash []byte
}

type tracePK []byte

func storeKeysOptionsAndFlatten(ctx context.Context, db sqlExecutor, keyValueCache readOnlyCache, gr *jsonio.GoldResults, commitID int) ([]sqlTraceValue, map[string]bool, error) {
	defer shared.NewMetricsTimer("store_sql_keys_and_options").Stop()
	rv := make([]sqlTraceValue, 0, len(gr.Results))

	tracesToCreate := make([]jsonAndHash, 0, len(gr.Results))
	tracesToUpdate := make([]tracePK, 0, len(gr.Results))
	optionsToCreate := make([]jsonAndHash, 0, len(gr.Results))
	groupingsToCreate := make([]jsonAndHash, 0, len(gr.Results))

	newCacheEntries := map[string]bool{}

	paramset := paramtools.ParamSet{}
	for _, result := range gr.Results {
		keys, options := paramsAndOptions(gr, result)
		if err := shouldIngest(keys, options); err != nil {
			sklog.Infof("Not ingesting a result: %s", err)
			continue
		}
		keysJSON, keysHash, err := sql.SerializeMap(keys)
		if err != nil {
			sklog.Errorf("Invalid keys map or something %s: %s", keys, err)
			continue
		}
		paramset.AddParams(keys)
		optsJSON, optsHash, err := sql.SerializeMap(options)
		if err != nil {
			sklog.Errorf("Invalid options map or something %s: %s", keys, err)
			continue
		}
		paramset.AddParams(options)
		grouping := groupingFor(keys)
		groupingJSON, groupingHash, err := sql.SerializeMap(grouping)
		if err != nil {
			sklog.Errorf("Invalid grouping or something %s: %s", keys, err)
			continue
		}
		digestBytes, err := sql.DigestToBytes(result.Digest)
		if err != nil {
			sklog.Errorf("Invalid digest %s: %s", result.Digest, err)
			continue
		}

		if h := string(keysHash); !newCacheEntries[h] && !keyValueCache.Contains(h) {
			tracesToCreate = append(tracesToCreate, jsonAndHash{
				json: keysJSON,
				hash: keysHash,
			})
			newCacheEntries[h] = true
		}
		tracesToUpdate = append(tracesToUpdate, keysHash)

		if h := string(optsHash); !newCacheEntries[h] && !keyValueCache.Contains(h) {
			optionsToCreate = append(optionsToCreate, jsonAndHash{
				json: optsJSON,
				hash: optsHash,
			})
			newCacheEntries[h] = true
		}

		if h := string(groupingHash); !newCacheEntries[h] && !keyValueCache.Contains(h) {
			groupingsToCreate = append(groupingsToCreate, jsonAndHash{
				json: groupingJSON,
				hash: groupingHash,
			})
			newCacheEntries[h] = true
		}

		rv = append(rv, sqlTraceValue{
			digestBytes: digestBytes,
			traceID:     keysHash,
			optionsID:   optsHash,
			groupingID:  groupingHash,
		})
	}

	if err := batchCreateTraces(ctx, db, tracesToCreate, commitID); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	if err := batchUpdateTraces(ctx, db, tracesToUpdate, commitID); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	if err := batchCreateKeys(ctx, db, insertGroupings, groupingsToCreate); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	if err := batchCreateKeys(ctx, db, insertOptions, optionsToCreate); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	if err := batchStoreParamset(ctx, db, paramset, commitID); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	return rv, newCacheEntries, nil
}

const insertGroupings = `INSERT INTO Groupings (grouping_id, keys) VALUES `
const insertOptions = `INSERT INTO Options (options_id, keys) VALUES `

func batchCreateKeys(ctx context.Context, db sqlExecutor, insert string, toCreate []jsonAndHash) error {
	if len(toCreate) == 0 {
		return nil
	}
	const chunkSize = 100 // This was arbitrarily picked.
	err := util.ChunkIter(len(toCreate), chunkSize, func(startIdx int, endIdx int) error {
		batch := toCreate[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := insert
		const valuesPerRow = 2
		vp, err := sql.ValuesPlaceholders(valuesPerRow, len(batch))
		if err != nil {
			return skerr.Wrap(err)
		}
		statement += vp
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, value := range batch {
			arguments = append(arguments, value.hash, value.json)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, then the data we are writing
		// is immutable.
		statement += ` ON CONFLICT DO NOTHING;`

		_, err = db.Exec(ctx, statement, arguments...)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d JSON entries and hashes with insert %s", len(toCreate), insert)
	}
	return nil
}

const insertTraces = `INSERT INTO Traces (trace_id, keys, most_recent_commit_id) VALUES `

func batchCreateTraces(ctx context.Context, db sqlExecutor, toCreate []jsonAndHash, commitFK int) error {
	if len(toCreate) == 0 {
		return nil
	}
	const chunkSize = 100 // This was arbitrarily picked.
	err := util.ChunkIter(len(toCreate), chunkSize, func(startIdx int, endIdx int) error {
		batch := toCreate[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := insertTraces
		const valuesPerRow = 3
		vp, err := sql.ValuesPlaceholders(valuesPerRow, len(batch))
		if err != nil {
			return skerr.Wrap(err)
		}
		statement += vp
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, value := range batch {
			arguments = append(arguments, value.hash, value.json, commitFK)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, then we are done (we'll update
		// the most_recent_commit_id on the subsequent update)
		statement += ` ON CONFLICT DO NOTHING;`

		_, err = db.Exec(ctx, statement, arguments...)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d traces", len(toCreate))
	}
	return nil
}

const conditionalUpdateMostRecentCommit = `
UPDATE Traces 
SET most_recent_commit_id = CASE
WHEN (most_recent_commit_id < $1) THEN 
  $1
ELSE 
  most_recent_commit_id
END
WHERE trace_id IN `

func batchUpdateTraces(ctx context.Context, db sqlExecutor, toUpdate []tracePK, commitFK int) error {
	if len(toUpdate) == 0 {
		return nil
	}
	const chunkSize = 100 // This was arbitrarily picked.
	err := util.ChunkIter(len(toUpdate), chunkSize, func(startIdx int, endIdx int) error {
		batch := toUpdate[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := conditionalUpdateMostRecentCommit
		// Start at 2 because $1 is the commitFK
		ip, err := sql.InPlaceholders(2, len(batch))
		if err != nil {
			return skerr.Wrap(err)
		}
		statement += ip
		// Add 1 because we have the commitFK and then N arguments in our query
		arguments := make([]interface{}, 0, len(batch)+1)
		arguments = append(arguments, commitFK)
		for _, trace := range batch {
			arguments = append(arguments, trace)
		}

		_, err = db.Exec(ctx, statement, arguments...)
		return skerr.Wrapf(err, "updating with sql %s", statement)
	})
	if err != nil {
		return skerr.Wrapf(err, "updating %d traces", len(toUpdate))
	}
	return nil
}

func batchStoreParamset(ctx context.Context, db sqlExecutor, paramset paramtools.ParamSet, commitFK int) error {
	if len(paramset) == 0 {
		return nil
	}

	type keyValue struct {
		key   string
		value string
	}

	var toStore []keyValue
	for key, values := range paramset {
		for _, v := range values {
			toStore = append(toStore, keyValue{key: key, value: v})
		}
	}

	const chunkSize = 200 // This was arbitrarily picked.
	err := util.ChunkIter(len(toStore), chunkSize, func(startIdx int, endIdx int) error {
		batch := toStore[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO PrimaryBranchParams (key, value, commit_id) VALUES `
		const valuesPerRow = 3
		vp, err := sql.ValuesPlaceholders(valuesPerRow, len(batch))
		if err != nil {
			return skerr.Wrap(err)
		}
		statement += vp
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, kv := range batch {
			arguments = append(arguments, kv.key, kv.value, commitFK)
		}
		// ON CONFLICT DO NOTHING because the rows are immutable once written.
		statement += ` ON CONFLICT DO NOTHING;`

		_, err = db.Exec(ctx, statement, arguments...)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d PrimaryBranchParams", len(toStore))
	}
	return nil
}

func storeValues(ctx context.Context, db sqlExecutor, values []sqlTraceValue, commitNum int, sourceFileHash []byte) error {
	if len(values) == 0 {
		return nil
	}

	// This size was arbitrarily picked, but note that you can't have more than 65536 placeholders
	// in a single query. Turning this up to 9000 is not a good idea, it can overload the cluster.
	const chunkSize = 100
	const valuesPerTraceValuesRow = 7
	const valuesPerExpectationsRow = 3
	traceValuesArgs := make([]interface{}, 0, chunkSize*valuesPerTraceValuesRow)
	expectationsArgs := make([]interface{}, 0, chunkSize*valuesPerExpectationsRow)

	duplicateKeys := make(map[string]bool, len(values))

	// This sorting will makes it so that we are are focusing on a few shards at a time as we send
	// them to the SQL database. Experimentally, this has been shown to be 1) fast to sort (~1 ms)
	// and 2) reduce the 99th percentile for responding to queries. Sorting exactly by shard had
	// a drastic reduction in performance; The reason for this is not super clear, but it probably
	// lead to too much focusing on a single shard/range, especially when multiple goroutines are
	// sending queries at once.
	sort.Slice(values, func(i, j int) bool {
		return values[i].traceID[0] < values[j].traceID[0]
	})

	err := util.ChunkIter(len(values), chunkSize, func(startIdx int, endIdx int) error {
		batch := values[startIdx:endIdx]
		uniqueValues := 0
		// we can re-use the same arguments to avoid extra allocations/GC.
		traceValuesArgs = traceValuesArgs[:0]
		expectationsArgs = expectationsArgs[:0]
		for _, value := range batch {
			// Occasionally, duplicate traces are in Skia's data (which doesn't use goldctl).
			// We strip those out to avoid a SQL error.
			if _, ok := duplicateKeys[string(value.traceID)]; ok {
				continue
			}
			uniqueValues++
			duplicateKeys[string(value.traceID)] = true
			traceValuesArgs = append(traceValuesArgs, value.traceID)
			traceValuesArgs = append(traceValuesArgs, sql.ComputeTraceValueShard(value.traceID))
			traceValuesArgs = append(traceValuesArgs, commitNum)
			traceValuesArgs = append(traceValuesArgs, value.groupingID)
			traceValuesArgs = append(traceValuesArgs, value.digestBytes)
			traceValuesArgs = append(traceValuesArgs, value.optionsID)
			traceValuesArgs = append(traceValuesArgs, sourceFileHash)

			expectationsArgs = append(expectationsArgs, value.groupingID)
			expectationsArgs = append(expectationsArgs, value.digestBytes)
			expectationsArgs = append(expectationsArgs, sql.LabelUntriaged)
		}

		traceValuesStatement := `
UPSERT INTO TraceValues (trace_id, shard, commit_id, grouping_id, digest, 
  options_id, source_file_id) VALUES `
		valuePlaceholders, err := sql.ValuesPlaceholders(valuesPerTraceValuesRow, uniqueValues)
		if err != nil {
			return err // should never happen
		}

		_, err = db.Exec(ctx, traceValuesStatement+valuePlaceholders, traceValuesArgs...)
		if err != nil {
			return skerr.Wrapf(err, "storing values")
		}

		// This lets us index by untriaged status and not have to scan over all TraceValues.
		expectationsStatement := `INSERT INTO Expectations (grouping_id, digest, label) VALUES `
		valuePlaceholders, err = sql.ValuesPlaceholders(valuesPerExpectationsRow, uniqueValues)
		if err != nil {
			return err // should never happen
		}
		// We only want to store that a given digest is untriaged if we haven't seen it before.
		expectationsStatement = expectationsStatement + valuePlaceholders + `ON CONFLICT DO NOTHING;`
		_, err = db.Exec(ctx, expectationsStatement, expectationsArgs...)
		return skerr.Wrapf(err, "storing expectations")
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d values and expectations", len(values))
	}
	return nil
}

func groupingFor(keys map[string]string) map[string]string {
	return map[string]string{
		types.CorpusField:     keys[types.CorpusField],
		types.PrimaryKeyField: keys[types.PrimaryKeyField],
	}
}
