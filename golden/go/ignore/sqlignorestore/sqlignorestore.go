// Package sqlignorestore contains a SQL implementation of ignore.Store.
package sqlignorestore

import (
	"context"
	"net/url"

	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/sql/schema"
)

type StoreImpl struct {
	db *pgxpool.Pool
}

// New returns a SQL based implementation of ignore.Store.
func New(db *pgxpool.Pool) *StoreImpl {
	return &StoreImpl{db: db}
}

// Create implements the ignore.Store interface.
func (s *StoreImpl) Create(ctx context.Context, rule ignore.Rule) error {
	v, err := url.ParseQuery(rule.Query)
	if err != nil {
		return skerr.Wrapf(err, "invalid ignore query %q", rule.Query)
	}
	_, err = s.db.Exec(ctx, `
INSERT INTO IgnoreRules (creator_email, updated_email, expires, note, query)
VALUES ($1, $2, $3, $4, $5)`, rule.CreatedBy, rule.CreatedBy, rule.Expires, rule.Note, v)
	if err != nil {
		return skerr.Wrap(err)
	}
	// TODO(kjlubick) update the Traces Table.
	return nil
}

// List implements the ignore.Store interface.
func (s *StoreImpl) List(ctx context.Context) ([]ignore.Rule, error) {
	var rv []ignore.Rule
	rows, err := s.db.Query(ctx, `SELECT * FROM IgnoreRules ORDER BY expires ASC`)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var r schema.IgnoreRuleRow
		err := rows.Scan(&r.IgnoreRuleID, &r.CreatorEmail, &r.UpdatedEmail, &r.Expires, &r.Note, &r.Query)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, ignore.Rule{
			ID:        r.IgnoreRuleID.String(),
			CreatedBy: r.CreatorEmail,
			UpdatedBy: r.UpdatedEmail,
			Expires:   r.Expires.UTC(),
			Query:     url.Values(r.Query).Encode(),
			Note:      r.Note,
		})
	}
	return rv, nil
}

// Update implements the ignore.Store interface.
func (s *StoreImpl) Update(ctx context.Context, rule ignore.Rule) error {
	v, err := url.ParseQuery(rule.Query)
	if err != nil {
		return skerr.Wrapf(err, "invalid ignore query %q", rule.Query)
	}
	_, err = s.db.Exec(ctx, `
UPDATE IgnoreRules SET (updated_email, expires, note, query) = ($1, $2, $3, $4)
WHERE ignore_rule_id = $5`, rule.UpdatedBy, rule.Expires, rule.Note, v, rule.ID)
	if err != nil {
		return skerr.Wrap(err)
	}
	// TODO(kjlubick) update the Traces Table.
	return nil
}

// Delete implements the ignore.Store interface.
func (s *StoreImpl) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `
DELETE FROM IgnoreRules WHERE ignore_rule_id = $1`, id)
	if err != nil {
		return skerr.Wrap(err)
	}
	// TODO(kjlubick) update the Traces Table.
	return nil
}

// Make sure Store fulfills the ignore.Store interface
var _ ignore.Store = (*StoreImpl)(nil)
