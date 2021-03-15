package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql"
)

func main() {
	var (
		sqlDB = flag.String("sql_db", "", "Something like the instance id (no dashes)")
	)
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	u := sql.GetConnectionURL("root@localhost:26234", *sqlDB)
	sklog.Infof(u)
	conf, err := pgxpool.ParseConfig(u)
	if err != nil {
		sklog.Fatalf("error getting postgres config %s: %s", u, err)
	}
	conf.MaxConns = 16
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Info("You must run\nkubectl port-forward gold-cockroachdb-0 26234:26234")
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	if err := updateIgnoredTraces(ctx, db); err != nil {
		sklog.Fatalf("Error updating ignore rules: %s", err)
	}
	sklog.Infof("Done")
}

func updateIgnoredTraces(ctx context.Context, db *pgxpool.Pool) error {
	ignoreRules, err := getIgnoreRules(ctx, db)
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("SELECTED %d rules", len(ignoreRules))
	if err := updateTraces(ctx, db, ignoreRules); err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("Traces updated")
	if err := updateValuesAtHead(ctx, db, ignoreRules); err != nil {
		return skerr.Wrap(err)
	}
	return skerr.Wrap(err)
}

// updateTraces applies ignore rules to all traces in batches of 1000.
func updateTraces(ctx context.Context, db *pgxpool.Pool, rules []paramtools.ParamSet) error {
	condition, arguments := convertIgnoreRules(rules)
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
			sklog.Infof("updated some traces")
			return nil
		})
		if err != nil {
			return skerr.Wrapf(err, "Updating primary traces with statement: %s", statement)
		}
	}
	return nil
}

// updateTraces applies ignore rules to all traces in batches of 1000.
func updateValuesAtHead(ctx context.Context, db *pgxpool.Pool, rules []paramtools.ParamSet) error {
	condition, arguments := convertIgnoreRules(rules)
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
			sklog.Infof("updated some values at head")
			return nil
		})
		if err != nil {
			return skerr.Wrapf(err, "Updating values at head with statement: %s", statement)
		}
	}
	return nil
}

func getIgnoreRules(ctx context.Context, db *pgxpool.Pool) ([]paramtools.ParamSet, error) {
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

// convertIgnoreRules turns a Paramset into a SQL clause that would match rows using a column
// named "keys". It is currently implemented with AND clauses, but could potentially be done
// with UNION/INTERSECT depending on performance needs.
func convertIgnoreRules(rules []paramtools.ParamSet) (string, []interface{}) {
	if len(rules) == 0 {
		return "false", nil
	}
	conditions := make([]string, 0, len(rules))
	var arguments []interface{}
	argIdx := 1

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
