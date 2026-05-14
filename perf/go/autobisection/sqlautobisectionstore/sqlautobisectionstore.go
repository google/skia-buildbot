package sqlautobisectionstore

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/autobisection"
	"go.skia.org/infra/perf/go/autobisection/sqlautobisectionstore/schema"
)

// statement is an SQL statement identifier.
type statement int

const (
	insertAutobisection statement = iota
)

var statements = map[statement]string{
	insertAutobisection: `
		INSERT INTO
			Autobisections (job_id, anomaly_group_id, anomaly_id, is_real_regression)
		VALUES
			($1, $2, $3, $4)
	`,
}

// AutobisectionStore implements the autobisection.Store interface using a SQL database.
type AutobisectionStore struct {
	db pool.Pool
}

// New returns a new AutobisectionStore.
func New(db pool.Pool) (*AutobisectionStore, error) {
	return &AutobisectionStore{
		db: db,
	}, nil
}

// Save implements the autobisection.Store interface.
func (s *AutobisectionStore) Save(ctx context.Context, b *schema.AutobisectionSchema) error {
	if b.JobID == "" {
		return skerr.Fmt("job_id cannot be empty")
	}
	if b.AnomalyGroupID == "" {
		return skerr.Fmt("anomaly group id cannot be empty")
	}
	if b.AnomalyId == "" {
		return skerr.Fmt("anomaly id cannot be empty")
	}

	statement := statements[insertAutobisection]
	if _, err := s.db.Exec(ctx, statement,
		b.JobID,
		b.AnomalyGroupID,
		b.AnomalyId,
		b.IsRealRegression,
	); err != nil {
		return fmt.Errorf("error inserting autobisection result into Autobisections table: %w", err)
	}
	return nil
}

// Ensure AutobisectionStore implements autobisection.Store
var _ autobisection.Store = (*AutobisectionStore)(nil)
