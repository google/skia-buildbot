package sqlautobisectionstore

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/autobisection"
	v1 "go.skia.org/infra/perf/go/autobisection/proto/v1"
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
			Autobisections (job_id, workflow_id, anomaly_group_id, anomaly_id, regression_status)
		VALUES
			($1, $2, $3, $4, $5)
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
	if err := validateAutobisection(b); err != nil {
		return skerr.Wrap(err)
	}

	statement := statements[insertAutobisection]
	if _, err := s.db.Exec(ctx, statement,
		b.JobID,
		b.WorkflowID,
		b.AnomalyGroupID,
		b.AnomalyId,
		b.RegressionStatus,
	); err != nil {
		return fmt.Errorf("error inserting autobisection result into Autobisections table: %w", err)
	}
	return nil
}

func validateAutobisection(b *schema.AutobisectionSchema) error {
	if b.JobID == "" {
		return skerr.Fmt("job_id cannot be empty")
	}
	if b.AnomalyGroupID == "" {
		return skerr.Fmt("anomaly group id cannot be empty")
	}
	if b.AnomalyId == "" {
		return skerr.Fmt("anomaly id cannot be empty")
	}
	if b.WorkflowID == "" {
		return skerr.Fmt("workflow id cannot be empty")
	}
	if !validateRegressionStatus(b.RegressionStatus) {
		return skerr.Fmt("regression status is invalid: %s", b.RegressionStatus)
	}
	return nil
}

func validateRegressionStatus(regressionStatus string) bool {
	v, ok := v1.RegressionStatus_value[regressionStatus]
	// value 0 is "unspecified", but we always provide the status.
	return ok && v > 0
}

// Ensure AutobisectionStore implements autobisection.Store
var _ autobisection.Store = (*AutobisectionStore)(nil)
