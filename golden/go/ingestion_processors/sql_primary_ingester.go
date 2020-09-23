package ingestion_processors

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Configuration option that identifies a tracestore backed by BigTable.
	sqlGoldIngester = "gold_sql"

	sqlConnectionURL = "SQLConnectionURL"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(sqlGoldIngester, newSQLProcessor)
}

func newSQLProcessor(ctx context.Context, _ vcsinfo.VCS, cfg ingestion.Config, _ *http.Client) (ingestion.Processor, error) {
	// example: "postgresql://root@gold-cockroachdb-public:26234/staging_db?sslmode=disable"
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
		commitNumCache: cnc,
		keyValueCache:  koc,
		traceCounter:   metrics2.GetCounter("gold_traces_ingested"),
		now:            time.Now,
	}, nil
}

type sqlProcessor struct {
	db             *pgxpool.Pool // The standard *pgx.Conn is *not* thread safe.
	mostRecentData *lru.Cache    // maps trace_id => int (commitNumber)
	commitNumCache *lru.Cache    // maps GitHash => int (commitNumber)
	keyValueCache  *lru.Cache    // maps md5hash(map) => bool if it has been stored
	traceCounter   metrics2.Counter

	now func() time.Time
}

// sqlExecutor lets us use either *pgxpool.Pool or a transaction it returns to run our queries.
type sqlExecutor interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
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

	// Know that we know this is a valid commit, let's store the data.
	sourceFileHash, err := createSourceFile(ctx, s.db, fileName, s.now())
	if err != nil {
		return skerr.Wrapf(err, "storing source file metadata for %s", fileName)
	}

	traceValuesToInsert, newCacheEntries, err := s.storeTraceMetadata(ctx, gr, commitID)
	if err != nil {
		return skerr.Wrapf(err, "storing keys and options for %s", fileName)
	}

	defer shared.NewMetricsTimer("store_sql_values").Stop()
	if err := storeValues(ctx, s.db, traceValuesToInsert, commitID, sourceFileHash); err != nil {
		return skerr.Wrapf(err, "storing %d values from %s", len(traceValuesToInsert), fileName)
	}

	_, err = s.db.Exec(ctx, `UPDATE Commits SET has_data = true WHERE commit_id = $1`, commitID)
	if err != nil {
		return skerr.Wrapf(err, "writing commit denseness while ingesting %s", fileName)
	}

	// Now that we are sure the file was ingested, we can update the shared cache of key/value maps
	// so the future ingestions will go faster (since they don't need to store the immutable data
	// in those caches.
	for hashAsString := range newCacheEntries {
		s.keyValueCache.Add(hashAsString, true)
	}
	s.traceCounter.Inc(int64(len(traceValuesToInsert)))
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

type traceIDAndDigest struct {
	traceID string // hex encoded
	digest  types.Digest
}

type valueAtHead struct {
	traceID    []byte
	groupingID []byte
	keysJSON   string
}

func (s *sqlProcessor) storeTraceMetadata(ctx context.Context, gr *jsonio.GoldResults, commitID int) ([]sqlTraceValue, map[string]bool, error) {
	defer shared.NewMetricsTimer("store_trace_metadata").Stop()
	rv := make([]sqlTraceValue, 0, len(gr.Results))

	tracesToCreate := make([]jsonAndHash, 0, len(gr.Results))
	atHeadToCreate := make([]valueAtHead, 0, len(gr.Results))
	tracesToUpdate := make([]traceIDAndDigest, 0, len(gr.Results))
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

		if h := string(keysHash); !newCacheEntries[h] && !s.keyValueCache.Contains(h) {
			tracesToCreate = append(tracesToCreate, jsonAndHash{
				json: keysJSON,
				hash: keysHash,
			})
			atHeadToCreate = append(atHeadToCreate, valueAtHead{
				traceID:    keysHash,
				groupingID: groupingHash,
				keysJSON:   keysJSON,
			})
			newCacheEntries[h] = true
		}
		tracesToUpdate = append(tracesToUpdate, traceIDAndDigest{
			traceID: hex.EncodeToString(keysHash),
			digest:  result.Digest,
		})

		if h := string(optsHash); !newCacheEntries[h] && !s.keyValueCache.Contains(h) {
			optionsToCreate = append(optionsToCreate, jsonAndHash{
				json: optsJSON,
				hash: optsHash,
			})
			newCacheEntries[h] = true
		}

		if h := string(groupingHash); !newCacheEntries[h] && !s.keyValueCache.Contains(h) {
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

	if err := batchCreateTraces(ctx, s.db, tracesToCreate); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	if err := batchCreateValuesAtHead(ctx, s.db, atHeadToCreate); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	if err := updateTraceValuesAtHead(ctx, s.db, tracesToUpdate, commitID); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	if err := batchCreateKeys(ctx, s.db, insertGroupings, groupingsToCreate); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	if err := batchCreateKeys(ctx, s.db, insertOptions, optionsToCreate); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	if err := batchStoreParamset(ctx, s.db, paramset, commitID); err != nil {
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
	// This can be somewhat high because in the steady state case there is not a lot of contention
	// on this table.
	const chunkSize = 500
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

const insertTraces = `INSERT INTO Traces (trace_id, keys) VALUES `

func batchCreateTraces(ctx context.Context, db sqlExecutor, toCreate []jsonAndHash) error {
	if len(toCreate) == 0 {
		return nil
	}
	// This can be somewhat high because in the steady state case there is not a lot of conflict
	// on an individual row (and we bail out if there is already a value here).
	const chunkSize = 200
	err := util.ChunkIter(len(toCreate), chunkSize, func(startIdx int, endIdx int) error {
		batch := toCreate[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := insertTraces
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

const insertValuesAtHead = `INSERT INTO ValuesAtHead
(trace_id, grouping_id, keys, expectation_label, most_recent_commit_id) VALUES
`

func batchCreateValuesAtHead(ctx context.Context, db sqlExecutor, toCreate []valueAtHead) error {
	if len(toCreate) == 0 {
		return nil
	}
	// This can be somewhat high because in the steady state case there is not a lot of conflict
	// on an individual row (and we bail out if there is already a value here).
	const chunkSize = 200
	err := util.ChunkIter(len(toCreate), chunkSize, func(startIdx int, endIdx int) error {
		batch := toCreate[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := insertValuesAtHead
		const valuesPerRow = 5
		vp, err := sql.ValuesPlaceholders(valuesPerRow, len(batch))
		if err != nil {
			return skerr.Wrap(err)
		}
		statement += vp
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, value := range batch {
			arguments = append(arguments, value.traceID, value.groupingID, value.keysJSON, sql.LabelUntriaged, 0)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, then we are done (we'll update
		// the most_recent_commit_id on the subsequent update)
		statement += ` ON CONFLICT DO NOTHING;`

		_, err = db.Exec(ctx, statement, arguments...)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d values at head", len(toCreate))
	}
	return nil
}

// This approach is inspired by https://stackoverflow.com/a/28723617 as a way to perform multiple
// updates based on tuples of data. The JSON that is argument 1 has as the key a trace_id
// (as a hex encoded string) and the latest digest as the corresponding value (aslo as a hex string)
// This could be done perhaps with a temporary table, when those leave experimental support.
// This approach is so that we do not have to make n individual UPDATE/SET calls where n is the
// number of items in the file we are ingesting. This allows us to batch the SQL requests.
const conditionalUpdateValuesAtHead = `
WITH ToUpdate AS (
  SELECT decode(key, 'hex') AS trace_id, decode(value, 'hex') AS new_digest
  FROM json_each_text($1)
)
UPDATE ValuesAtHead
SET
  digest = CASE
WHEN (most_recent_commit_id < $2) THEN
  new_digest
ELSE
  digest
END,
  most_recent_commit_id = CASE
WHEN (most_recent_commit_id < $2) THEN
  $2
ELSE
  most_recent_commit_id
END
FROM ToUpdate
WHERE ValuesAtHead.trace_id = ToUpdate.trace_id
RETURNING NOTHING`

func updateTraceValuesAtHead(ctx context.Context, db sqlExecutor, toUpdate []traceIDAndDigest, commitFK int) error {
	if len(toUpdate) == 0 {
		return nil
	}

	// This value was chosen arbitrarily
	const chunkSize = 100
	err := util.ChunkIter(len(toUpdate), chunkSize, func(startIdx int, endIdx int) error {
		batch := toUpdate[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		traceDigests := make(map[string]types.Digest, len(batch))
		for _, t := range batch {
			traceDigests[t.traceID] = t.digest
		}
		jsonTuples, err := json.Marshal(traceDigests)
		if err != nil {
			return skerr.Wrapf(err, "Converting %d tuples to a JSON blob", len(traceDigests))
		}

		_, err = db.Exec(ctx, conditionalUpdateValuesAtHead, jsonTuples, commitFK)
		return skerr.Wrapf(err, "updating with sql %s", conditionalUpdateValuesAtHead)
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
	expectationsArgs := make([]interface{}, 0, len(values)*valuesPerExpectationsRow)

	duplicateKeys := make(map[string]bool, len(values))

	err := util.ChunkIter(len(values), chunkSize, func(startIdx int, endIdx int) error {
		batch := values[startIdx:endIdx]
		uniqueValues := 0
		// we can re-use the same arguments to avoid extra allocations/GC.
		traceValuesArgs = traceValuesArgs[:0]
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
		if uniqueValues == 0 {
			return nil
		}

		traceValuesStatement := `
UPSERT INTO TraceValues (trace_id, shard, commit_id, grouping_id, digest,
  options_id, source_file_id) VALUES `
		valuePlaceholders, err := sql.ValuesPlaceholders(valuesPerTraceValuesRow, uniqueValues)
		if err != nil {
			return err // should never happen
		}

		_, err = db.Exec(ctx, traceValuesStatement+valuePlaceholders, traceValuesArgs...)
		return skerr.Wrapf(err, "storing values")
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d values", len(values))
	}

	const expChunkSize = 1000 * valuesPerExpectationsRow
	util.ChunkIter(len(expectationsArgs), expChunkSize, func(startIdx int, endIdx int) error {
		argumentBatch := expectationsArgs[startIdx:endIdx]
		if len(argumentBatch) == 0 {
			return nil
		}

		// This lets us index by untriaged status and not have to scan over all TraceValues.
		expectationsStatement := `INSERT INTO Expectations (grouping_id, digest, label) VALUES `
		valuePlaceholders, err := sql.ValuesPlaceholders(valuesPerExpectationsRow, len(argumentBatch)/valuesPerExpectationsRow)
		if err != nil {
			return err // should never happen
		}
		// We only want to store that a given digest is untriaged if we haven't seen it before.
		expectationsStatement = expectationsStatement + valuePlaceholders + `ON CONFLICT DO NOTHING;`
		_, err = db.Exec(ctx, expectationsStatement, argumentBatch...)
		return skerr.Wrapf(err, "storing expectations")
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d expectations", len(values))
	}

	return nil
}

func groupingFor(keys map[string]string) map[string]string {
	return map[string]string{
		types.CorpusField:     keys[types.CorpusField],
		types.PrimaryKeyField: keys[types.PrimaryKeyField],
	}
}
