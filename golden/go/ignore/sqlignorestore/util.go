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
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/sql/schema"
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
	err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		ignoreRules, err := getAllIgnoreRules(ctx, tx)
		if err != nil {
			return err
		}
		sklog.Infof("SELECTED %d rules", len(ignoreRules))
		span.AddAttributes(trace.Int64Attribute("num_rules", int64(len(ignoreRules))))
		if err := updateTracesAndValuesAtHead(ctx, tx, ignoreRules); err != nil {
			return err
		}
		// Any remaining traces with a null matches_any_ignore_rule don't match any of the rules.
		if err := updateNullTraces(ctx, tx); err != nil {
			return err
		}
		return nil
	})
	return skerr.Wrap(err)
}

// updateTraces applies ignore rules to all traces in batches of 1000.
func updateTracesAndValuesAtHead(ctx context.Context, tx pgx.Tx, rules []paramtools.ParamSet) error {
	ctx, span := trace.StartSpan(ctx, "updateTracesAndValuesAtHead")
	defer span.End()
	const batchSize = 1000

	for _, rule := range rules {
		// This follows the bulk update recommendation from
		// https://www.cockroachlabs.com/docs/v20.2/bulk-update-data
		traceIDs, err := getNotIgnoredTraceIDs(ctx, tx, rule)
		if err != nil {
			return skerr.Wrap(err)
		}
		if len(traceIDs) == 0 {
			continue
		}
		err = util.ChunkIter(len(traceIDs), batchSize, func(startIdx int, endIdx int) error {
			_, err := tx.Exec(ctx, `UPDATE Traces SET matches_any_ignore_rule = TRUE
WHERE trace_id = ANY $1`, traceIDs[startIdx:endIdx])
			return err // don't wrap, might be retried
		})
		if err != nil {
			return err // don't wrap, might be retried
		}
		err = util.ChunkIter(len(traceIDs), batchSize, func(startIdx int, endIdx int) error {
			_, err := tx.Exec(ctx, `UPDATE ValuesAtHead SET matches_any_ignore_rule = TRUE
WHERE trace_id = ANY $1`, traceIDs[startIdx:endIdx])
			return err
		})
		if err != nil {
			return err // don't wrap, might be retried
		}
		sklog.Infof("Updated %d traces for rule %v", len(traceIDs), rule)
	}
	return nil
}

// getNotIgnoredTraceIDs returns a slice of trace IDs that correspond to traces that match the
// rule and are currently not ignored. They are returned as []interface{} so it is easier to
// pass into the next database call.
func getNotIgnoredTraceIDs(ctx context.Context, tx pgx.Tx, rule paramtools.ParamSet) ([]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "getNotIgnoredTraceIDs")
	defer span.End()
	statement := statementForNotIgnoredTraceIDs(rule)
	rows, err := tx.Query(ctx, statement)
	if err != nil {
		return nil, err // don't wrap, might be retried
	}
	defer rows.Close()
	// This type is more convenient for passing into an Exec call.
	var rv []interface{}
	for rows.Next() {
		var t schema.TraceID
		if err := rows.Scan(&t); err != nil {
			return nil, skerr.Wrap(err) // this is likely our fault, so wrapping helps debugging.
		}
		rv = append(rv, t)
	}
	return rv, nil
}

// getAllIgnoreRulesreturns all the ParamSet associated with all current ignore rules.
func getAllIgnoreRules(ctx context.Context, tx pgx.Tx) ([]paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "getAllIgnoreRules")
	defer span.End()
	var ignoreRules []paramtools.ParamSet
	rows, err := tx.Query(ctx, "SELECT query FROM IgnoreRules")
	if err != nil {
		return nil, err // don't wrap, it might be retried
	}
	defer rows.Close()
	for rows.Next() {
		rule := paramtools.ParamSet{}
		err := rows.Scan(&rule)
		if err != nil {
			return nil, skerr.Wrap(err) // An error here is likely our fault
		}
		ignoreRules = append(ignoreRules, rule)
	}
	return ignoreRules, nil
}

// statementForNotIgnoredTraceIDs returns a SQL Statement as a string that returns all traces
// matching the rule and are not already ignored. It is built as a string because kjlubick@ was not
// able to use the placeholder values to compare JSONB types removed from a JSONB object to
// a string while still using the indexes.
func statementForNotIgnoredTraceIDs(rule paramtools.ParamSet) string {
	rule.Normalize()
	keys := make([]string, 0, len(rule))
	for key := range rule {
		if key != sanitize(key) {
			sklog.Infof("key %q did not pass sanitization")
			continue
		}
		keys = append(keys, key)
	}
	statement := "WITH\n"
	sort.Strings(keys) // sort the keys for determinism
	for i, key := range keys {
		statement += fmt.Sprintf("U%d AS (\n", i)
		for j, value := range rule[key] {
			if j != 0 {
				statement += "\tUNION\n"
			}
			statement += fmt.Sprintf("\tSELECT trace_id FROM Traces WHERE keys -> '%s' = '%q'\n", key, sanitize(value))
		}
		if i == len(keys)-1 {
			statement += ")\n"
		} else {
			statement += "),\n"
		}
	}
	statement += "SELECT trace_id FROM Traces WHERE trace_id IN (\n"
	for i := range keys {
		if i != 0 {
			statement += "INTERSECT\n"
		}
		statement += fmt.Sprintf("SELECT trace_id FROM U%d\n", i)
	}
	statement += ") AND (matches_any_ignore_rule = FALSE OR matches_any_ignore_rule is NULL)"
	return statement
}

// sanitize removes unsafe characters from strings to avoid SQL injections. Namely quotes.
func sanitize(s string) string {
	s = strings.ReplaceAll(s, `'`, ``)
	return strings.ReplaceAll(s, `"`, ``)
}

// updateNullTraces updates all traces where matches_any_ignore_rule is NULL to be false, since
// we have not matched any other ignore rules. There is a limit to how many traces will be updated
// with this to avoid transactions from being too big.
func updateNullTraces(ctx context.Context, tx pgx.Tx) error {
	ctx, span := trace.StartSpan(ctx, "updateNullTraces")
	defer span.End()
	statement := `UPDATE Traces SET matches_any_ignore_rule = FALSE WHERE matches_any_ignore_rule is NULL LIMIT 1000 RETURNING 1;`
	// To avoid making this transaction too big, we'll stop after 10 batches. We can get the rest
	// of the traces in the next update call.
	for i := 0; i < 10; i++ {
		row := tx.QueryRow(ctx, statement)
		var count int
		if err := row.Scan(&count); err != nil {
			if err == pgx.ErrNoRows {
				break
			}
			return err // Don't wrap, it may be retried
		}
	}
	statement = `UPDATE ValuesAtHead SET matches_any_ignore_rule = FALSE WHERE matches_any_ignore_rule is NULL LIMIT 1000 RETURNING 1;`
	for i := 0; i < 10; i++ {
		row := tx.QueryRow(ctx, statement)
		var count int
		if err := row.Scan(&count); err != nil {
			if err == pgx.ErrNoRows {
				break
			}
			return err // Don't wrap, it may be retried
		}
	}
	return nil
}
