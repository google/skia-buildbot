package main

import (
	"context"
	"flag"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/golden/go/sql"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/ignore/fs_ignorestore"
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

	url := sql.GetConnectionURL("root@localhost:26234", *newSQLDatabase)
	conf, err := pgxpool.ParseConfig(url)
	if err != nil {
		sklog.Fatalf("error getting postgres config %s: %s", url, err)
	}
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}

	old := fs_ignorestore.New(ctx, fsClient)
	rules, err := old.List(ctx)
	if err != nil {
		sklog.Fatalf("Loading old rules: %s", err)
	}

	err := storeToSQL(ctx, db, rules)
	if err != nil {
		sklog.Fatalf("Error storing to SQL: %s", err)
	}
	sklog.Infof("Done")
}
