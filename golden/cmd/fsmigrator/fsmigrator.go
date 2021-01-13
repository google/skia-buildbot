// The fsmigrator executable migrates various data from firestore to an SQL database.
// It uses port forwarding, as that is the simplest approach and there shouldn't be
// too much data.
package main

import (
	"context"
	"flag"
	"net/url"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/ignore/fs_ignorestore"
	"go.skia.org/infra/golden/go/sql"
)

func main() {
	var (
		fsProjectID    = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		oldFSNamespace = flag.String("old_fs_namespace", "", "Typically the instance id. e.g. 'chrome-gpu', 'skia', etc")
		newSQLDatabase = flag.String("new_sql_db", "", "Something like the instance id (no dashes)")
	)
	flag.Parse()

	if *oldFSNamespace == "" {
		sklog.Fatalf("You must include fs_namespace")
	}

	if *newSQLDatabase == "" {
		sklog.Fatalf("You must include new_sql_db")
	}

	ctx := context.Background()
	fsClient, err := ifirestore.NewClient(ctx, *fsProjectID, "gold", *oldFSNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}

	u := sql.GetConnectionURL("root@localhost:26234", *newSQLDatabase)
	conf, err := pgxpool.ParseConfig(u)
	if err != nil {
		sklog.Fatalf("error getting postgres config %s: %s", u, err)
	}
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Info("You must run\nkubectl port-forward gold-cockroachdb-0 26234:26234")
		sklog.Fatalf("error connecting to the database: %s", err)
	}

	old := fs_ignorestore.New(ctx, fsClient)
	var rules []ignore.Rule
	// Wait for initial fetch to complete
	for len(rules) == 0 {
		time.Sleep(1000)
		rules, err = old.List(ctx)
		if err != nil {
			sklog.Fatalf("Loading old rules: %s", err)
		}
	}

	err = storeToSQL(ctx, db, rules)
	if err != nil {
		sklog.Fatalf("Error storing to SQL: %s", err)
	}
	sklog.Infof("Done")
}

func storeToSQL(ctx context.Context, db *pgxpool.Pool, rules []ignore.Rule) error {
	if len(rules) == 0 {
		return skerr.Fmt("Rules cannot be empty")
	}
	const statement = `INSERT INTO IgnoreRules (creator_email, updated_email, expires, note, query) VALUES `
	const valuesPerRow = 5
	placeholders := sql.ValuesPlaceholders(valuesPerRow, len(rules))
	arguments := make([]interface{}, 0, valuesPerRow*len(rules))
	for _, rule := range rules {
		query, err := url.ParseQuery(rule.Query)
		if err != nil {
			// Hopefully never happens
			return skerr.Wrapf(err, "invalid ignore query %q", rule.Query)
		}
		arguments = append(arguments, rule.CreatedBy, rule.UpdatedBy, rule.Expires, rule.Note, query)
	}

	_, err := db.Exec(ctx, statement+placeholders, arguments...)
	if err != nil {
		return skerr.Wrapf(err, "storing %d rules", len(rules))
	}
	return nil
}
