package sqlconfigstore

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/trace_visibility/sqlconfigstore/schema"
)

// SQLConfigStore implements persistence for visibility configuration.
type SQLConfigStore struct {
	db pool.Pool
}

// New returns a new SQLConfigStore.
func New(db pool.Pool) *SQLConfigStore {
	return &SQLConfigStore{
		db: db,
	}
}

// GetAll returns all visibility configurations from the database.
func (s *SQLConfigStore) GetAll(ctx context.Context) ([]schema.PublicTraceRulesSchema, error) {
	stmt := `SELECT public_rule_expr FROM PublicTraceRules`
	rows, err := s.db.Query(ctx, stmt)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to query PublicTraceRules")
	}
	defer rows.Close()

	var configs []schema.PublicTraceRulesSchema
	for rows.Next() {
		var cfg schema.PublicTraceRulesSchema
		if err := rows.Scan(&cfg.RuleExpression); err != nil {
			return nil, skerr.Wrapf(err, "Failed to scan PublicTraceRules")
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}

// Set adds or updates a visibility configuration entry.
func (s *SQLConfigStore) Set(ctx context.Context, ruleExpression string) error {
	stmt := `
		INSERT INTO PublicTraceRules (public_rule_expr)
		VALUES ($1)
		ON CONFLICT (public_rule_expr) DO NOTHING
	`
	_, err := s.db.Exec(ctx, stmt, ruleExpression)
	if err != nil {
		return skerr.Wrapf(err, "Failed to insert PublicTraceRules for %q", ruleExpression)
	}
	return nil
}

// Delete removes a visibility configuration entry.
func (s *SQLConfigStore) Delete(ctx context.Context, ruleExpression string) error {
	stmt := `DELETE FROM PublicTraceRules WHERE public_rule_expr = $1`
	_, err := s.db.Exec(ctx, stmt, ruleExpression)
	if err != nil {
		return skerr.Wrapf(err, "Failed to delete PublicTraceRules for %q", ruleExpression)
	}
	return nil
}
