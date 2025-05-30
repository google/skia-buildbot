package sqlignorestore

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/sql/schema"
)

// ConvertIgnoreRules turns a Paramset into a SQL clause that would match rows using a column
// named "keys". It is currently implemented with AND/OR clauses, but could potentially be done
// with UNION/INTERSECT depending on performance needs.
func ConvertIgnoreRules(rules []paramtools.ParamSet, databaseType config.DatabaseType) (string, []interface{}) {
	return convertIgnoreRules(rules, 1, databaseType)
}

// convertIgnoreRules takes a parameter that configures where the numbered params start.
// 1 is the lowest legal value. 2^16 is the biggest.
func convertIgnoreRules(rules []paramtools.ParamSet, startIndex int, databaseType config.DatabaseType) (string, []interface{}) {
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
			var subCondition string
			if databaseType == config.CockroachDB {
				subCondition = fmt.Sprintf("COALESCE(keys ->> $%d::STRING IN (", argIdx)
			} else {
				subCondition = fmt.Sprintf("COALESCE(keys ->> $%d::TEXT IN (", argIdx)
			}
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

// UpdateIgnoredTraces applies all the given ignore rules to all Traces and ValuesAtHead.
func UpdateIgnoredTraces(ctx context.Context, db *pgxpool.Pool) error {
	ctx, span := trace.StartSpan(ctx, "UpdateIgnoredTraces")
	defer span.End()

	const batchSize = 100000
	offset := 0
	for {
		// Get the ignore rules each time to limit the impact of a rule changing in the middle
		// of those operation.
		ignoreRules, err := getAllIgnoreRules(ctx, db)
		if err != nil {
			return skerr.Wrap(err)
		}
		if ok, err := updateBatch(ctx, db, ignoreRules, offset, batchSize); err != nil {
			return skerr.Wrap(err)
		} else if !ok {
			break
		}
		offset += batchSize
		sklog.Debugf("Updated one batch of Traces/ValuesAtHead")
	}
	sklog.Infof("Updated about %d traces", offset)
	return nil
}

// getAllIgnoreRulesreturns all the ParamSet associated with all current ignore rules.
func getAllIgnoreRules(ctx context.Context, db *pgxpool.Pool) ([]paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "getAllIgnoreRules")
	defer span.End()
	var ignoreRules []paramtools.ParamSet
	rows, err := db.Query(ctx, "SELECT query FROM IgnoreRules")
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

type idAndIgnored struct {
	traceID   schema.TraceID
	isIgnored bool
}

// updateBatch applies ignore rules to the batch of traces given by offset and batch to both
// the Traces and ValuesAtHead table. It applies updates individually, in case either table has
// gotten out of sync with the other. It returns true if it processed at least one trace in
// either table (and thus the process should continue).
// updateBatch is modified to pass the table name to applyUpdates
func updateBatch(ctx context.Context, db *pgxpool.Pool, rules []paramtools.ParamSet, offset, batch int) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "updateBatch")
	defer span.End()

	traceUpdates, haveMore1, err := fetchUpdates(ctx, db, rules, fetchTracesStatement, offset, batch)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if err := applyUpdates(ctx, db, traceUpdates, "Traces"); err != nil {
		return false, skerr.Wrapf(err, "applying updates to Traces table")
	}

	vahUpdates, haveMore2, err := fetchUpdates(ctx, db, rules, fetchValuesAtHeadStatement, offset, batch)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	// dbType is no longer strictly needed for applyUpdates if the individual UPDATE SQL is standard
	if err := applyUpdates(ctx, db, vahUpdates, "ValuesAtHead"); err != nil {
		return false, skerr.Wrapf(err, "applying updates to ValuesAtHead table")
	}
	return haveMore1 || haveMore2, nil
}

const fetchTracesStatement = `SELECT trace_id, keys, matches_any_ignore_rule FROM Traces OFFSET $1 LIMIT $2`
const fetchValuesAtHeadStatement = `SELECT trace_id, keys, matches_any_ignore_rule FROM ValuesAtHead OFFSET $1 LIMIT $2`

// fetchUpdates returns the trace ids and the new ignore status for any traces whose current
// value does not match what it is supposed to be.
func fetchUpdates(ctx context.Context, db *pgxpool.Pool, rules []paramtools.ParamSet, statement string, offset, batch int) ([]idAndIgnored, bool, error) {
	ctx, span := trace.StartSpan(ctx, "fetchUpdates")
	defer span.End()
	// We choose to get all the trace values and figure out the new "matches_any_ignore_rule" in
	// software instead of SQL statements because the latter makes for very complex queries that
	// are error-prone and hard to debug.
	rows, err := db.Query(ctx, statement, offset, batch)
	if err != nil {
		return nil, false, skerr.Wrap(err)
	}
	defer rows.Close()

	shouldContinue := false
	var updates []idAndIgnored
	for rows.Next() {
		shouldContinue = true
		var tID schema.TraceID
		var traceKeys paramtools.Params
		var matches pgtype.Bool
		if err := rows.Scan(&tID, &traceKeys, &matches); err != nil {
			return nil, false, skerr.Wrap(err)
		}
		newStatus := false
		for _, rule := range rules {
			if rule.MatchesParams(traceKeys) {
				newStatus = true
				break
			}
		}
		if matches.Status == pgtype.Null || newStatus != matches.Bool {
			updates = append(updates, idAndIgnored{
				traceID:   tID,
				isIgnored: newStatus,
			})
		}
	}
	return updates, shouldContinue, nil
}

// applyUpdates applies the given batch of updates to the target table.
// TODO(pasthana): It issues one database call per update, but they are all in a
// transaction. This is expensive, but couldn't figure out a way to do it in a
// single call for spanner for the time being. Come back to this later.
func applyUpdates(ctx context.Context, db *pgxpool.Pool, updates []idAndIgnored, targetTable string) error {
	if len(updates) == 0 {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "applyUpdates")
	span.AddAttributes(trace.Int64Attribute("num_updates", int64(len(updates))))
	defer span.End()

	err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		sql := fmt.Sprintf("UPDATE %s SET matches_any_ignore_rule = $1 WHERE trace_id = $2", targetTable)

		for _, u := range updates {
			batch.Queue(sql, u.isIgnored, u.traceID)
		}

		br := tx.SendBatch(ctx, batch)
		defer func() {
			if closeErr := br.Close(); closeErr != nil {
				// Log or handle the batch close error, but prioritize returning the first exec error
				sklog.Errorf("Error closing batch results for table %s: %v", targetTable, closeErr)
			}
		}()

		for i := 0; i < len(updates); i++ {
			if _, execErr := br.Exec(); execErr != nil {
				// The transaction will be rolled back by crdbpgx.ExecuteTx if this function returns an error.
				return skerr.Wrapf(execErr, "batch update failed for item %d, trace_id %s, in table %s", i, hex.EncodeToString(updates[i].traceID), targetTable)
			}
		}
		return nil
	})
	return skerr.Wrapf(err, "transaction failed for table %s", targetTable)
}
