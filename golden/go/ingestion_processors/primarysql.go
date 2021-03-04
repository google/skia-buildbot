package ingestion_processors

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/golden/go/ingestion"
)

type sqlPrimaryIngester struct {
	db        *pgxpool.Pool
	source    ingestion.Source
	tileWidth int
}

// HandlesFile returns true if the underlying source handles the given file
func (s *sqlPrimaryIngester) HandlesFile(name string) bool {
	return s.source.HandlesFile(name)
}

// Process take the content of the given file and writes it to the various SQL tables required
// by the schema.
func (s *sqlPrimaryIngester) Process(ctx context.Context, filename string) error {
	panic("implement me")
}

// overwriteNowKey is used by tests to make the time deterministic.
const overwriteNowKey = contextKey("overwriteNow")

type contextKey string

// Make sure sqlPrimaryIngester implements the ingestion.Processor interface.
var _ ingestion.Processor = (*sqlPrimaryIngester)(nil)
