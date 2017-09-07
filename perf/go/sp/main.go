package main

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/admin/database/apiv1"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

func createClients(ctx context.Context, db string) (*database.DatabaseAdminClient, *spanner.Client) {
	adminClient, err := database.NewDatabaseAdminClient(ctx, option.WithCredentialsFile("service-account.json"))

	//adminClient, err := database.NewDatabaseAdminClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatal(err)
	}

	dataClient, err := spanner.NewClient(ctx, db)
	if err != nil {
		sklog.Fatal(err)
	}
	return adminClient, dataClient
}

func main() {
	common.Init()
	//cl, err := auth.NewDefaultJWTServiceAccountClient()

	ctx := context.Background()
	_, client := createClients(context.Background(), "projects/google.com:skia-buildbots/instances/skiainfra/databases/perftest")
	/*
		  // This create code works.

			op, err := ad.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
				Database: "projects/google.com:skia-buildbots/instances/skiainfra/databases/perftest",
				Statements: []string{
					`CREATE TABLE sk_db_version (
						id         INT64 NOT NULL,
						version    INT64 NOT NULL,
						updated    TIMESTAMP NOT NULL
					)
					PRIMARY KEY (id)`,
				},
			})
			if err != nil {
				sklog.Fatal(err)
			}
			err = op.Wait(ctx)
			if err != nil {
				sklog.Fatal(err)
			}
	*/

	// insert or update the version.
	_, err := client.Apply(ctx, []*spanner.Mutation{spanner.InsertOrUpdate("sk_db_version", []string{"id", "version", "updated"}, []interface{}{1, 1, time.Now()})})
	if err != nil {
		sklog.Fatal(err)
	}

	// Read back the version.
	row, err := client.Single().ReadRow(ctx, "sk_db_version", spanner.Key{1}, []string{"version"})
	if err != nil {
		sklog.Fatal(err)
	}
	var version int64
	if err := row.Columns(&version); err != nil {
		sklog.Fatal(err)
	}
	fmt.Printf("Running at %d", version)
}
