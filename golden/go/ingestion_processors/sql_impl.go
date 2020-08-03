package ingestion_processors

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4/pgxpool"

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
	}, nil
}

type sqlProcessor struct {
	db               *pgxpool.Pool // The standard *pgx.Conn is *not* thread safe.
	commitNumCache   *lru.Cache
	keysOptionsCache *lru.Cache
	traceCounter     metrics2.Counter
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
	sourceFileHash, err := s.storeFile(ctx, resultsFile.Name(), time.Now())
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
		keysJSON, keysHash, err := serializeMap(keys)
		if err != nil {
			sklog.Errorf("Invalid keys map or something %s: %s", keys, err)
			continue
		}
		optsJSON, optsHash, err := serializeMap(options)
		if err != nil {
			sklog.Errorf("Invalid options map or something %s: %s", keys, err)
			continue
		}
		grouping := groupingFor(keys)
		groupingJSON, groupingHash, err := serializeMap(grouping)
		if err != nil {
			sklog.Errorf("Invalid grouping or something %s: %s", keys, err)
			continue
		}
		digestBytes, err := digestToBytes(result.Digest)
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

// ON CONFLICT DO NOTHING because this table is full of immutable data
const insertKeys = `INSERT INTO KeyValueMaps (keys_hash, keys) VALUES `
const doNothingOnConflict = `ON CONFLICT DO NOTHING`

func (s *sqlProcessor) batchStoreKeys(ctx context.Context, toStore []jsonAndHash) error {
	if len(toStore) == 0 {
		return nil
	}
	const chunkSize = 100 // This was arbitrarily picked.
	err := util.ChunkIter(len(toStore), chunkSize, func(startIdx int, endIdx int) error {
		batch := toStore[startIdx:endIdx]
		statement := insertKeys

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
		statement += doNothingOnConflict

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

	const chunkSize = 1000
	err := util.ChunkIter(len(values), chunkSize, func(startIdx int, endIdx int) error {
		batch := values[startIdx:endIdx]

		upsertStatement := `
UPSERT INTO TraceValues (trace_hash, shard, commit_number, grouping_hash, digest, 
  options_hash, source_file_hash) VALUES `

		var arguments []interface{}
		argIdx := 1
		for i, value := range batch {
			if i != 0 {
				upsertStatement += ","
			}
			upsertStatement += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5, argIdx+6)
			argIdx += 7

			arguments = append(arguments, value.keysHash)
			arguments = append(arguments, value.keysHash[:1])
			arguments = append(arguments, commitNum)
			arguments = append(arguments, value.groupingHash)
			arguments = append(arguments, value.digestBytes)
			arguments = append(arguments, value.optionsHash)
			arguments = append(arguments, sourceFileHash)
		}
		_, err := s.db.Exec(ctx, upsertStatement, arguments...)
		return err
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d values", len(values))
	}
	return nil
}

// serializeMap returns the given map in JSON and the md5 of that json string.
func serializeMap(m map[string]string) (string, []byte, error) {
	str, err := json.Marshal(m)
	if err != nil {
		return "", nil, err
	}
	h := md5.Sum(str)
	return string(str), h[:], err
}

func digestToBytes(d types.Digest) ([]byte, error) {
	return hex.DecodeString(string(d))
}

func groupingFor(keys map[string]string) map[string]string {
	return map[string]string{
		types.CorpusField:     keys[types.CorpusField],
		types.PrimaryKeyField: keys[types.PrimaryKeyField],
	}
}
