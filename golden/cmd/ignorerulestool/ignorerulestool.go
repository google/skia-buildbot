package main

import (
	"context"
	"flag"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/golden/go/ignore/sqlignorestore"

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
	if err := sqlignorestore.UpdateIgnoredTraces(ctx, db); err != nil {
		sklog.Fatalf("Error updating ignore rules: %s", err)
	}
	sklog.Infof("Done")
}
