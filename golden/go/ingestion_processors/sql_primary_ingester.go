package ingestion_processors

import (
	"context"
	"crypto/md5"
	"net/http"
	"sort"
	"time"

	lru "github.com/hashicorp/golang-lru"
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
		db:               db,
		commitNumCache:   cnc, // maps GitHash => int (commitNumber)
		keysOptionsCache: koc, // maps md5hash(map) => bool if it has been stored
		traceCounter:     metrics2.GetCounter("gold_traces_ingested"),
		now:              time.Now,
	}, nil
}

type sqlProcessor struct {
	db               *pgxpool.Pool // The standard *pgx.Conn is *not* thread safe.
	commitNumCache   *lru.Cache
	keysOptionsCache *lru.Cache
	traceCounter     metrics2.Counter

	now func() time.Time
}

func (s *sqlProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	defer metrics2.FuncTimer().Stop()
	gr, err := processGoldResults(ctx, resultsFile)
	if err != nil {
		return skerr.Wrapf(err, "could not process results file %s", resultsFile.Name())
	}

	if len(gr.Results) == 0 {
		sklog.Infof("ignoring file %s because it has no results", resultsFile.Name())
		return ingestion.IgnoreResultsFileErr
	}

	commitID, err := s.getCLNumber(ctx, gr.GitHash)
	if err != nil {
		return skerr.Wrapf(err, "could not determine branch for %s", gr.GitHash)
	}

	// Know that we know this is a valid commit, let's store the data.
	sourceFileHash, err := s.storeFile(ctx, resultsFile.Name(), s.now())
	if err != nil {
		return skerr.Wrapf(err, "storing source file metadata for %s", resultsFile.Name())
	}

	toStore, err := s.storeKeysOptionsAndFlatten(ctx, gr, commitID)
	if err != nil {
		return skerr.Wrapf(err, "storing keys and options for %s", resultsFile.Name())
	}

	defer shared.NewMetricsTimer("store_sql_values").Stop()
	if err := s.storeValues(ctx, toStore, commitID, sourceFileHash); err != nil {
		return skerr.Wrapf(err, "storing %d values from %s", len(toStore), resultsFile.Name())
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

func (s *sqlProcessor) storeFile(ctx context.Context, pathToFile string, now time.Time) ([]byte, error) {
	sourceFileHash := md5.Sum([]byte(pathToFile))
	_, err := s.db.Exec(ctx, upsertSourceFile, sourceFileHash[:], pathToFile, now)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return sourceFileHash[:], nil
}

type jsonAndHash struct {
	json string
	hash []byte
}

func (s *sqlProcessor) storeKeysOptionsAndFlatten(ctx context.Context, gr *jsonio.GoldResults, commitID int) ([]sqlTraceValue, error) {
	defer shared.NewMetricsTimer("store_sql_keys_and_options").Stop()
	rv := make([]sqlTraceValue, 0, len(gr.Results))

	tracesToStore := make([]jsonAndHash, 0, len(gr.Results))
	optionsToStore := make([]jsonAndHash, 0, len(gr.Results))
	groupingsToStore := make([]jsonAndHash, 0, len(gr.Results))
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

		if h := string(keysHash); !s.keysOptionsCache.Contains(h) {
			s.keysOptionsCache.Add(h, true)
			tracesToStore = append(tracesToStore, jsonAndHash{
				json: keysJSON,
				hash: keysHash,
			})
		}

		if h := string(optsHash); !s.keysOptionsCache.Contains(h) {
			s.keysOptionsCache.Add(h, true)
			optionsToStore = append(optionsToStore, jsonAndHash{
				json: optsJSON,
				hash: optsHash,
			})
		}

		if h := string(groupingHash); !s.keysOptionsCache.Contains(h) {
			s.keysOptionsCache.Add(h, true)
			groupingsToStore = append(groupingsToStore, jsonAndHash{
				json: groupingJSON,
				hash: groupingHash,
			})
		}

		rv = append(rv, sqlTraceValue{
			digestBytes: digestBytes,
			traceID:     keysHash,
			optionsID:   optsHash,
			groupingID:  groupingHash,
		})
	}

	if err := s.batchStoreKeys(ctx, insertTraces, tracesToStore); err != nil {
		// There was an error, clear so we have to store everything again, just to be sure.
		s.keysOptionsCache.Purge()
		return nil, skerr.Wrap(err)
	}

	if err := s.batchStoreKeys(ctx, insertGroupings, groupingsToStore); err != nil {
		// There was an error, clear so we have to store everything again, just to be sure.
		s.keysOptionsCache.Purge()
		return nil, skerr.Wrap(err)
	}

	if err := s.batchStoreKeys(ctx, insertOptions, optionsToStore); err != nil {
		// There was an error, clear so we have to store everything again, just to be sure.
		s.keysOptionsCache.Purge()
		return nil, skerr.Wrap(err)
	}

	if err := s.batchStoreParamset(ctx, paramset, commitID); err != nil {
		return nil, skerr.Wrap(err)
	}

	return rv, nil
}

const insertTraces = `INSERT INTO Traces (trace_id, keys) VALUES `
const insertGroupings = `INSERT INTO Groupings (grouping_id, keys) VALUES `
const insertOptions = `INSERT INTO Options (options_id, keys) VALUES `

func (s *sqlProcessor) batchStoreKeys(ctx context.Context, insert string, toStore []jsonAndHash) error {
	if len(toStore) == 0 {
		return nil
	}
	const chunkSize = 100 // This was arbitrarily picked.
	err := util.ChunkIter(len(toStore), chunkSize, func(startIdx int, endIdx int) error {
		batch := toStore[startIdx:endIdx]
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

		_, err = s.db.Exec(ctx, statement, arguments...)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d JSON entries and hashes with insert %s", len(toStore), insert)
	}
	return nil
}

func (s *sqlProcessor) batchStoreParamset(ctx context.Context, paramset paramtools.ParamSet, commitID int) error {
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
			arguments = append(arguments, kv.key, kv.value, commitID)
		}
		// ON CONFLICT DO NOTHING because the rows are immutable once written.
		statement += ` ON CONFLICT DO NOTHING;`

		_, err = s.db.Exec(ctx, statement, arguments...)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d PrimaryBranchParams", len(toStore))
	}
	return nil
}

func (s *sqlProcessor) storeValues(ctx context.Context, values []sqlTraceValue, commitNum int, sourceFileHash []byte) error {
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

		_, err = s.db.Exec(ctx, traceValuesStatement+valuePlaceholders, traceValuesArgs...)
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
		_, err = s.db.Exec(ctx, expectationsStatement, expectationsArgs...)
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
