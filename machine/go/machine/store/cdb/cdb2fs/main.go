// Command line tool to migrate date from CockroachDB to Firestore.
//
// Before running this command you must run:
//
//    $ kubectl port-forward service/machineserver-cockroachdb-public 25001:26257
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/configs"
	"go.skia.org/infra/machine/go/machine"
	machineStore "go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machine/store/cdb"
	"go.skia.org/infra/machine/go/machineserver/config"
)

func main() {
	ctx := context.Background()

	connectionString := fmt.Sprintf("postgresql://root@127.0.0.1:25001/%s?sslmode=disable", cdb.DatabaseName)
	db, err := pgxpool.Connect(ctx, connectionString)
	if err != nil {
		sklog.Fatal(err)
	}

	source := cdb.New(db)

	var instanceConfig config.InstanceConfig
	b, err := fs.ReadFile(configs.Configs, "prod.json")
	if err != nil {
		sklog.Fatalf("read config file %q: %s", "prod.json", err)
	}
	err = json.Unmarshal(b, &instanceConfig)
	if err != nil {
		sklog.Fatal(err)
	}

	receiver, err := machineStore.NewFirestoreImpl(ctx, true, instanceConfig)
	if err != nil {
		sklog.Fatal(err)
	}

	all, err := source.List(ctx)
	if err != nil {
		sklog.Fatal(err)
	}

	for i, d := range all {
		fmt.Printf("%d ", i)
		err := receiver.Update(ctx, d.Dimensions[machine.DimID][0], func(in machine.Description) machine.Description {
			return d
		})
		if err != nil {
			sklog.Fatal(err)
		}
	}
	fmt.Println("")
}
