package ingestion_processors

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/types"

	// Make sure the postgreSQL driver is loaded.
	_ "github.com/lib/pq"
)

func newSQLProcessor(ctx context.Context, _ vcsinfo.VCS, _ ingestion.Config, _ *http.Client) (ingestion.Processor, error) {
	dbConnectionURL := "postgresql://root@localhost:26257/demo_gold_db?sslmode=disable"
	db, err := sql.Open("postgres", dbConnectionURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "opening the database %s", dbConnectionURL)
	}
	if err = db.PingContext(ctx); err != nil {
		return nil, skerr.Wrapf(err, "connecting to database via ping %s", dbConnectionURL)
	}
	cnc, err := lru.New(1000) // ~40 bytes per entry = 40k
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	koc, err := lru.New(10_000_000) // ~20 bytes per entry = 200M
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &sqlProcessor{
		db:               db,
		commitNumCache:   cnc, // maps GitHash => int (commitNumber)
		keysOptionsCache: koc, // maps md5hash(map) => bool if it has been stored
	}, nil
}

type sqlProcessor struct {
	db               *sql.DB
	commitNumCache   *lru.Cache
	keysOptionsCache *lru.Cache
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

	if err := s.storeValues(ctx, toStore, commitNum, sourceFileHash); err != nil {
		return skerr.Wrapf(err, "storing %d values from %s", len(toStore), resultsFile.Name())
	}
	return nil
}

type sqlTraceValue struct {
	digest      []byte
	keysHash    []byte
	optionsHash []byte
}

const selectCommitNumberFromGitHash = `SELECT commit_number FROM Commits WHERE git_hash=$1 LIMIT 1`

func (s *sqlProcessor) getCLNumber(ctx context.Context, hash string) (int, error) {
	if num, ok := s.commitNumCache.Get(hash); ok {
		return num.(int), nil
	}

	var commitNum int32
	row := s.db.QueryRowContext(ctx, selectCommitNumberFromGitHash, hash)
	if err := row.Scan(&commitNum); err != nil {
		return 0, skerr.Wrapf(err, "getting number for %s", hash)
	}
	s.commitNumCache.Add(hash, commitNum)
	return int(commitNum), nil
}

const upsertSourceFile = `UPSERT INTO SourceFiles (source_file_hash, source_file, last_ingested) VALUES ($1, $2, $3)`

func (s *sqlProcessor) storeFile(ctx context.Context, pathToFile string, now time.Time) ([]byte, error) {
	sourceFileHash := md5.Sum([]byte(pathToFile))
	_, err := s.db.ExecContext(ctx, upsertSourceFile, sourceFileHash[:], pathToFile, now)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return sourceFileHash[:], nil
}

func (s *sqlProcessor) storeKeysOptionsAndFlatten(ctx context.Context, gr *jsonio.GoldResults) ([]sqlTraceValue, error) {

	rv := make([]sqlTraceValue, 0, len(gr.Results))
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
		digestBytes, err := digestToBytes(result.Digest)
		if err != nil {
			sklog.Errorf("Invalid digest %s: %s", result.Digest, err)
			continue
		}

		if err := s.maybeStoreToKeys(ctx, keysJSON, keysHash); err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := s.maybeStoreToOptions(ctx, optsJSON, optsHash); err != nil {
			return nil, skerr.Wrap(err)
		}

		rv = append(rv, sqlTraceValue{
			digest:      digestBytes,
			keysHash:    keysHash,
			optionsHash: optsHash,
		})
	}
	return rv, nil
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
