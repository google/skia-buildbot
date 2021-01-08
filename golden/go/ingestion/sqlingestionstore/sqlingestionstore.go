// Package sqlingestionstore contains a SQL-backed implementation of IngestionStore, which
// is meant as a quick "yes/no" to the question "Did we already ingest this file?" when polling
// for files missed during Pub/Sub ingestion.
package sqlingestionstore

import (
	"context"
	"crypto/md5"
	"time"

	"go.skia.org/infra/golden/go/ingestion"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/skerr"
)

const (
	cacheSize = 100_000
)

type sqlStore struct {
	db    *pgxpool.Pool
	cache *lru.Cache
}

func New(db *pgxpool.Pool) *sqlStore {
	cache, err := lru.New(cacheSize)
	if err != nil {
		panic(err) // should only cause error if size < 0
	}
	return &sqlStore{db: db, cache: cache}
}

// SetIngested implements the ingestion.IngestionStore interface.
// TODO(kjlubick) When the actual SQL ingestion works, change this to be a no-op (the ingesters
//   themselves will write to this table) and WasIngested to target the SourceFiles table.
func (s *sqlStore) SetIngested(ctx context.Context, fileName, _ string, ts time.Time) error {
	sourceID := md5.Sum([]byte(fileName))
	_, err := s.db.Exec(ctx, `
UPSERT INTO DeprecatedIngestedFiles (source_file_id, source_file, last_ingested)
VALUES ($1, $2, $3)`, sourceID[:], fileName, ts)
	if err != nil {
		return skerr.Wrapf(err, "Marking %s as ingested", fileName)
	}
	return nil
}

// WasIngested implements the ingestion.IngestionStore interface. It has a RAM cache to remember
// already ingested files (since an ingested file cannot become "uningested").
func (s *sqlStore) WasIngested(ctx context.Context, fileName, _ string) (bool, error) {
	if s.cache.Contains(fileName) {
		return true, nil
	}
	sourceID := md5.Sum([]byte(fileName))
	row := s.db.QueryRow(ctx, `SELECT count(*) FROM DeprecatedIngestedFiles where source_file_id = $1`, sourceID[:])
	count := 0
	err := row.Scan(&count)
	if err != nil {
		return false, skerr.Wrapf(err, "Looking for ingested file %s", fileName)
	}
	if count == 0 {
		return false, nil
	}
	s.cache.Add(fileName, true)
	return true, nil
}

// Verify sqlStore implements IngestionStore
var _ ingestion.IngestionStore = (*sqlStore)(nil)
