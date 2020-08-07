package ingestion_processors

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"sort"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4/pgxpool"
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
	db, err := pgxpool.Connect(ctx, dbConnectionURL)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
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
	// 10 million traces/options should be sufficient to avoid many unnecessary puts to the TraceIDs
	// and OptionIDs tables
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

	commitNum, err := s.getCLNumber(ctx, gr.GitHash)
	if err != nil {
		return skerr.Wrapf(err, "could not determine branch for %s", gr.GitHash)
	}

	// Know that we know this is a valid commit, let's store the data.
	sourceFileHash, err := s.storeFile(ctx, resultsFile.Name(), s.now())
	if err != nil {
		return skerr.Wrapf(err, "storing source file metadata for %s", resultsFile.Name())
	}

	toStore, err := s.storeKeysOptionsAndFlatten(ctx, gr)
	if err != nil {
		return skerr.Wrapf(err, "storing keys and options for %s", resultsFile.Name())
	}

	defer shared.NewMetricsTimer("store_sql_values").Stop()
	if err := s.storeValues(ctx, toStore, commitNum, sourceFileHash); err != nil {
		return skerr.Wrapf(err, "storing %d values from %s", len(toStore), resultsFile.Name())
	}
	s.traceCounter.Inc(int64(len(toStore)))
	return nil
}

type sqlTraceValue struct {
	digestBytes  []byte
	keysHash     []byte
	optionsHash  []byte
	groupingHash []byte
}

const selectCommitNumberFromGitHash = `SELECT commit_number FROM Commits WHERE git_hash=$1 LIMIT 1`

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

const upsertSourceFile = `UPSERT INTO SourceFiles (source_file_hash, source_file, last_ingested) VALUES ($1, $2, $3)`

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

func (s *sqlProcessor) storeKeysOptionsAndFlatten(ctx context.Context, gr *jsonio.GoldResults) ([]sqlTraceValue, error) {
	defer shared.NewMetricsTimer("store_sql_keys_and_options").Stop()
	rv := make([]sqlTraceValue, 0, len(gr.Results))

	keysToStore := make([]jsonAndHash, 0, len(gr.Results))
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
		optsJSON, optsHash, err := sql.SerializeMap(options)
		if err != nil {
			sklog.Errorf("Invalid options map or something %s: %s", keys, err)
			continue
		}
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
			keysToStore = append(keysToStore, jsonAndHash{
				json: keysJSON,
				hash: keysHash,
			})
		}

		if h := string(optsHash); !s.keysOptionsCache.Contains(h) {
			s.keysOptionsCache.Add(h, true)
			keysToStore = append(keysToStore, jsonAndHash{
				json: optsJSON,
				hash: optsHash,
			})
		}

		if h := string(groupingHash); !s.keysOptionsCache.Contains(h) {
			s.keysOptionsCache.Add(h, true)
			keysToStore = append(keysToStore, jsonAndHash{
				json: groupingJSON,
				hash: groupingHash,
			})
		}

		rv = append(rv, sqlTraceValue{
			digestBytes:  digestBytes,
			keysHash:     keysHash,
			optionsHash:  optsHash,
			groupingHash: groupingHash,
		})
	}

	if err := s.batchStoreKeys(ctx, keysToStore); err != nil {
		// There was an error, clear so we have to store everything again, just to be sure.
		s.keysOptionsCache.Purge()
		return nil, skerr.Wrap(err)
	}

	return rv, nil
}

func (s *sqlProcessor) batchStoreKeys(ctx context.Context, toStore []jsonAndHash) error {
	if len(toStore) == 0 {
		return nil
	}
	const chunkSize = 100 // This was arbitrarily picked.
	err := util.ChunkIter(len(toStore), chunkSize, func(startIdx int, endIdx int) error {
		batch := toStore[startIdx:endIdx]
		statement := `INSERT INTO KeyValueMaps (keys_hash, keys) VALUES `

		var arguments []interface{}
		argumentIdx := 1
		for i, value := range batch {
			if i != 0 {
				statement += ","
			}
			statement += fmt.Sprintf("($%d, $%d)",
				argumentIdx, argumentIdx+1)
			argumentIdx += 2

			arguments = append(arguments, value.hash)
			arguments = append(arguments, value.json)
		}
		// ON CONFLICT DO NOTHING because this table is full of immutable data
		statement += `ON CONFLICT DO NOTHING`

		_, err := s.db.Exec(ctx, statement, arguments...)
		return err
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d JSON entries and hashes", len(toStore))
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
		return values[i].keysHash[0] < values[j].keysHash[0]
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
			if _, ok := duplicateKeys[string(value.keysHash)]; ok {
				continue
			}
			uniqueValues++
			duplicateKeys[string(value.keysHash)] = true
			traceValuesArgs = append(traceValuesArgs, value.keysHash)
			traceValuesArgs = append(traceValuesArgs, sql.ComputeTraceValueShard(value.keysHash))
			traceValuesArgs = append(traceValuesArgs, commitNum)
			traceValuesArgs = append(traceValuesArgs, value.groupingHash)
			traceValuesArgs = append(traceValuesArgs, value.digestBytes)
			traceValuesArgs = append(traceValuesArgs, value.optionsHash)
			traceValuesArgs = append(traceValuesArgs, sourceFileHash)

			expectationsArgs = append(expectationsArgs, value.groupingHash)
			expectationsArgs = append(expectationsArgs, value.digestBytes)
			expectationsArgs = append(expectationsArgs, sql.LabelUntriaged)
		}

		traceValuesStatement := `
UPSERT INTO TraceValues (trace_hash, shard, commit_number, grouping_hash, digest, 
  options_hash, source_file_hash) VALUES `
		valuePlaceholders, err := sql.ValuesPlaceholders(valuesPerTraceValuesRow, uniqueValues)
		if err != nil {
			return err // should never happen
		}

		_, err = s.db.Exec(ctx, traceValuesStatement+valuePlaceholders, traceValuesArgs...)
		if err != nil {
			return skerr.Wrapf(err, "storing values")
		}

		// This lets us index by untriaged status and not have to scan over all TraceValues.
		expectationsStatement := `
INSERT INTO Expectations (grouping_hash, digest, label) VALUES `
		valuePlaceholders, err = sql.ValuesPlaceholders(valuesPerExpectationsRow, uniqueValues)
		if err != nil {
			return err // should never happen
		}
		// We only want to store that a given digest is untriaged if we haven't seen it before.
		expectationsStatement = expectationsStatement + valuePlaceholders + `ON CONFLICT DO NOTHING`
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
