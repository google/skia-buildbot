// Package sqlignorestore contains a SQL implementation of ignore.Store.
package sqlignorestore

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/paramtools"
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

// Create implements the ignore.Store interface. It will mark all traces that match the rule as
// "ignored".
func (s *StoreImpl) Create(ctx context.Context, rule ignore.Rule) error {
	v, err := url.ParseQuery(rule.Query)
	if err != nil {
		return skerr.Wrapf(err, "invalid ignore query %q", rule.Query)
	}
	err = crdbpgx.ExecuteTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
INSERT INTO IgnoreRules (creator_email, updated_email, expires, note, query)
VALUES ($1, $2, $3, $4, $5)`, rule.CreatedBy, rule.CreatedBy, rule.Expires, rule.Note, v)
		return err // Don't wrap - crdbpgx might retry
	})
	if err != nil {
		return skerr.Wrapf(err, "creating ignore rule %#v", rule)
	}
	// We could be updating a lot of traces and values at head here. If done as one big transaction,
	// that could take a while to land if we are ingesting a lot of new data at the time. As such,
	// we do it in three independent transactions
	if err := markTracesAsIgnored(ctx, s.db, v); err != nil {
		return skerr.Wrap(err)
	}
	if err := markValuesAtHeadAsIgnored(ctx, s.db, v); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// markTracesAsIgnored will update all Traces matching the given paramset as ignored.
func markTracesAsIgnored(ctx context.Context, db *pgxpool.Pool, ps map[string][]string) error {
	condition, arguments := ConvertIgnoreRules([]paramtools.ParamSet{ps})
	statement := `UPDATE Traces SET matches_any_ignore_rule = TRUE WHERE `
	statement += condition
	statement += ` RETURNING NOTHING`
	err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, arguments...)
		return err // Don't wrap - crdbpgx might retry
	})
	return skerr.Wrap(err)
}

// markValuesAtHeadAsIgnored will update all ValuesAtHead matching the given paramset as ignored.
func markValuesAtHeadAsIgnored(ctx context.Context, db *pgxpool.Pool, ps map[string][]string) error {
	condition, arguments := ConvertIgnoreRules([]paramtools.ParamSet{ps})
	statement := `UPDATE ValuesAtHead SET matches_any_ignore_rule = TRUE WHERE `
	statement += condition
	statement += ` RETURNING NOTHING`
	err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, arguments...)
		return err // Don't wrap - crdbpgx might retry
	})
	return skerr.Wrap(err)
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

// Update implements the ignore.Store interface. If the rule paramset changes, it will mark the
// traces that match the old params as "ignored" or not depending on how the unchanged n-1 rules
// plus the new rule affect them. It will then update all traces that match the new rule as
// "ignored".
func (s *StoreImpl) Update(ctx context.Context, rule ignore.Rule) error {
	newParamSet, err := url.ParseQuery(rule.Query)
	if err != nil {
		return skerr.Wrapf(err, "invalid ignore query %q", rule.Query)
	}
	existingRulePS, err := s.getRuleParamSet(ctx, rule.ID)
	if err != nil {
		return skerr.Wrapf(err, "getting existing rule with id %s", rule.ID)
	}
	err = crdbpgx.ExecuteTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err = tx.Exec(ctx, `
UPDATE IgnoreRules SET (updated_email, expires, note, query) = ($1, $2, $3, $4)
WHERE ignore_rule_id = $5`, rule.UpdatedBy, rule.Expires, rule.Note, newParamSet, rule.ID)
		return err // Don't wrap - crdbpgx might retry
	})
	if err != nil {
		return skerr.Wrapf(err, "updating rule with id %s to %#v %#v", rule.ID, rule, newParamSet)
	}
	if existingRulePS.Equal(newParamSet) {
		// We don't need to update Traces or ValuesAtHead because the query was unchanged.
		return nil
	}
	// We could be updating a lot of traces and values at head here. If done as one big transaction,
	// that could take a while to land if we are ingesting a lot of new data at the time. As such,
	// we update in separate transactions.
	combinedRules, err := s.getOtherRules(ctx, rule.ID)
	if err != nil {
		return skerr.Wrapf(err, "getting other rules when updating %s", rule.ID)
	}
	// Apply those old rules to the traces that match the old paramset
	if err := conditionallyMarkTracesAsIgnored(ctx, s.db, existingRulePS, combinedRules); err != nil {
		return skerr.Wrap(err)
	}
	if err := conditionallyMarkValuesAtHeadAsIgnored(ctx, s.db, existingRulePS, combinedRules); err != nil {
		return skerr.Wrap(err)
	}
	// Apply the result of the new rules.
	if err := markTracesAsIgnored(ctx, s.db, newParamSet); err != nil {
		return skerr.Wrap(err)
	}
	if err := markValuesAtHeadAsIgnored(ctx, s.db, newParamSet); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// conditionallyMarkTracesAsIgnored applies the slice of rules to all traces that match the
// provided PatchSet.
func conditionallyMarkTracesAsIgnored(ctx context.Context, db *pgxpool.Pool, ps paramtools.ParamSet, rules []paramtools.ParamSet) error {
	matches, matchArgs := ConvertIgnoreRules(rules)
	condition, conArgs := convertIgnoreRules([]paramtools.ParamSet{ps}, len(matchArgs)+1)
	statement := `UPDATE Traces SET matches_any_ignore_rule = `
	statement += matches
	statement += ` WHERE `
	statement += condition
	statement += ` RETURNING NOTHING`
	matchArgs = append(matchArgs, conArgs...)
	err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, matchArgs...)
		return err // Don't wrap - crdbpgx might retry
	})
	if err != nil {
		return skerr.Wrapf(err, "updating traces to match %d rules", len(rules))
	}
	return nil
}

// conditionallyMarkValuesAtHeadAsIgnored applies the slice of rules to all ValuesAtHead that
// match the provided PatchSet.
func conditionallyMarkValuesAtHeadAsIgnored(ctx context.Context, db *pgxpool.Pool, ps paramtools.ParamSet, rules []paramtools.ParamSet) error {
	matches, matchArgs := ConvertIgnoreRules(rules)
	condition, conArgs := convertIgnoreRules([]paramtools.ParamSet{ps}, len(matchArgs)+1)
	statement := `UPDATE ValuesAtHead SET matches_any_ignore_rule = `
	statement += matches
	statement += ` WHERE `
	statement += condition
	statement += ` RETURNING NOTHING`
	matchArgs = append(matchArgs, conArgs...)
	err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, matchArgs...)
		return err // Don't wrap - crdbpgx might retry
	})
	if err != nil {
		return skerr.Wrapf(err, "updating traces to match %d rules", len(rules))
	}
	return nil
}

// Delete implements the ignore.Store interface. It will mark the traces that match the params of
// the deleted rule as "ignored" or not depending on how the unchanged n-1 rules affect them.
func (s *StoreImpl) Delete(ctx context.Context, id string) error {
	existingRulePS, err := s.getRuleParamSet(ctx, id)
	if err != nil {
		return skerr.Wrapf(err, "getting existing rule with id %s", id)
	}
	err = crdbpgx.ExecuteTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err = tx.Exec(ctx, `
DELETE FROM IgnoreRules WHERE ignore_rule_id = $1`, id)
		return err // Don't wrap - crdbpgx might retry
	})
	// We could be updating a lot of traces and values at head here. If done as one big transaction,
	// that could take a while to land if we are ingesting a lot of new data at the time. As such,
	// we update in separate transactions.
	remainingRules, err := s.getOtherRules(ctx, id)
	if err != nil {
		return skerr.Wrapf(err, "getting other rules when deleting %s", id)
	}
	// Apply those old rules to the traces that match the old paramset
	if err := conditionallyMarkTracesAsIgnored(ctx, s.db, existingRulePS, remainingRules); err != nil {
		return skerr.Wrap(err)
	}
	if err := conditionallyMarkValuesAtHeadAsIgnored(ctx, s.db, existingRulePS, remainingRules); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// getRuleParamSet returns the ParamSet for a given rule.
func (s *StoreImpl) getRuleParamSet(ctx context.Context, id string) (paramtools.ParamSet, error) {
	var ps paramtools.ParamSet
	row := s.db.QueryRow(ctx, `SELECT query FROM IgnoreRules where ignore_rule_id = $1`, id)
	if err := row.Scan(&ps); err != nil {
		return ps, skerr.Wrap(err)
	}
	return ps, nil
}

// getOtherRules returns a slice of params that has all rules not matching the given id.
func (s *StoreImpl) getOtherRules(ctx context.Context, id string) ([]paramtools.ParamSet, error) {
	var rules []paramtools.ParamSet
	rows, err := s.db.Query(ctx, `SELECT query FROM IgnoreRules where ignore_rule_id != $1`, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var ps paramtools.ParamSet
		if err := rows.Scan(&ps); err != nil {
			return nil, skerr.Wrap(err)
		}
		rules = append(rules, ps)
	}
	return rules, nil
}

// Make sure Store fulfills the ignore.Store interface
var _ ignore.Store = (*StoreImpl)(nil)

// ConvertIgnoreRules turns a Paramset into a SQL clause that would match rows using a column
// named "keys". It is currently implemented with AND/OR clauses, but could potentially be done
// with UNION/INTERSECT depending on performance needs.
func ConvertIgnoreRules(rules []paramtools.ParamSet) (string, []interface{}) {
	return convertIgnoreRules(rules, 1)
}

// convertIgnoreRules takes a parameter that configures where the numbered params start.
// 1 is the lowest legal value. 2^16 is the biggest.
func convertIgnoreRules(rules []paramtools.ParamSet, startIndex int) (string, []interface{}) {
	if len(rules) == 0 {
		return "false", nil
	}
	conditions := make([]string, 0, len(rules))
	var arguments []interface{}
	argIdx := startIndex

	for _, rule := range rules {
		rule.Normalize()
		keys := make([]string, 0, len(rule))
		for key := range rule {
			keys = append(keys, key)
		}
		sort.Strings(keys) // sort the keys for determinism

		andParts := make([]string, 0, len(rules))
		for _, key := range keys {
			values := rule[key]
			// We need the COALESCE because if a trace has one key, but not another, it will
			// return NULL. We don't want this NULL to propagate (FALSE OR NULL == NULL), so
			// we coalesce it to false (since if a trace lacks a key, it cannot match the key:value
			// pair).
			subCondition := fmt.Sprintf("COALESCE(keys ->> $%d::STRING IN (", argIdx)
			argIdx++
			arguments = append(arguments, key)
			for i, value := range values {
				if i != 0 {
					subCondition += ", "
				}
				subCondition += fmt.Sprintf("$%d", argIdx)
				argIdx++
				arguments = append(arguments, value)
			}
			subCondition += "), FALSE)"
			andParts = append(andParts, subCondition)
		}
		condition := "(" + strings.Join(andParts, " AND ") + ")"
		conditions = append(conditions, condition)
	}
	combined := "(" + strings.Join(conditions, "\nOR ") + ")"
	return combined, arguments
}
