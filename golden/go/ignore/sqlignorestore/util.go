package sqlignorestore

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

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

// UpdateIgnoredTraces applies all the given ignore rules to any Traces and ValuesAtHead which
// have matches_any_ignore_rule = NULL (typically newly ingested traces).
func UpdateIgnoredTraces(ctx context.Context, db *pgxpool.Pool) error {
	ctx, span := trace.StartSpan(ctx, "UpdateIgnoredTraces")
	defer span.End()
	ignoreRules, err := getAllIgnoreRules(ctx, db)
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("SELECTED %d rules", len(ignoreRules))
	span.AddAttributes(trace.Int64Attribute("num_rules", int64(len(ignoreRules))))
	if err := updateTraces(ctx, db, ignoreRules); err != nil {
		return skerr.Wrap(err)
	}
	if err := updateValuesAtHead(ctx, db, ignoreRules); err != nil {
		return skerr.Wrap(err)
	}
	return skerr.Wrap(err)
}

// updateTraces applies ignore rules to all traces in batches of 1000.
func updateTraces(ctx context.Context, db *pgxpool.Pool, rules []paramtools.ParamSet) error {
	ctx, span := trace.StartSpan(ctx, "updateTraces")
	defer span.End()
	condition, arguments := ConvertIgnoreRules(rules)
	statement := `UPDATE Traces SET matches_any_ignore_rule = `
	statement += condition
	// We return the literal value 1 if a non-zero number of rows were updated. Otherwise the
	// error ErrNoRows will be returned.
	statement += `WHERE matches_any_ignore_rule is NULL LIMIT 1000 RETURNING 1;`
	hasMore := true
	for hasMore {
		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			row := tx.QueryRow(ctx, statement, arguments...)
			var count int
			if err := row.Scan(&count); err != nil {
				if err == pgx.ErrNoRows {
					hasMore = false
					return nil
				}
				return err
			}
			sklog.Debugf("updated some traces")
			return nil
		})
		if err != nil {
			return skerr.Wrapf(err, "Updating primary traces with statement: %s", statement)
		}
	}
	return nil
}

// updateValuesAtHead applies ignore rules to all ValuesAtHead in batches of 1000.
func updateValuesAtHead(ctx context.Context, db *pgxpool.Pool, rules []paramtools.ParamSet) error {
	ctx, span := trace.StartSpan(ctx, "updateValuesAtHead")
	defer span.End()
	condition, arguments := ConvertIgnoreRules(rules)
	statement := `UPDATE ValuesAtHead SET matches_any_ignore_rule = `
	statement += condition
	// We return the literal value 1 if a non-zero number of rows were updated. Otherwise the
	// error ErrNoRows will be returned.
	statement += `WHERE matches_any_ignore_rule is NULL LIMIT 1000 RETURNING 1;`
	hasMore := true
	for hasMore {
		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			row := tx.QueryRow(ctx, statement, arguments...)
			var count int
			if err := row.Scan(&count); err != nil {
				if err == pgx.ErrNoRows {
					hasMore = false
					return nil
				}
				return err
			}
			sklog.Debugf("updated some values at head")
			return nil
		})
		if err != nil {
			return skerr.Wrapf(err, "Updating values at head with statement: %s", statement)
		}
	}
	return nil
}

func getAllIgnoreRules(ctx context.Context, db *pgxpool.Pool) ([]paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "getAllIgnoreRules")
	defer span.End()
	var ignoreRules []paramtools.ParamSet
	err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, "SELECT query FROM IgnoreRules")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			rule := paramtools.ParamSet{}
			err := rows.Scan(&rule)
			if err != nil {
				return skerr.Wrap(err)
			}
			ignoreRules = append(ignoreRules, rule)
		}
		return nil
	})
	return ignoreRules, skerr.Wrap(err)
}
