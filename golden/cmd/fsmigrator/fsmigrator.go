// The fsmigrator executable migrates various data from firestore to an SQL database.
// It uses port forwarding, as that is the simplest approach and there shouldn't be
// too much data.
package main

import (
	"context"
	"flag"
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/tjstore/fs_tjstore"
	"go.skia.org/infra/golden/go/tjstore/sqltjstore"
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
	oldStore := fs_tjstore.New(fsClient)

	u := sql.GetConnectionURL("root@localhost:26234", *newSQLDatabase)
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

	// We've already migrated all the CL and PS data, so we can query the SQL for that instead of
	// the Firestore. We picked the first of the year as the cutoff date because it would take too
	// long to migrate all the data, and this seemed like a good idea.
	rows, err := db.Query(ctx, `
SELECT Patchsets.changelist_id, Patchsets.system, Patchsets.patchset_id, Changelists.last_ingested_data
FROM Changelists JOIN Patchsets ON
  Changelists.changelist_id = Patchsets.changelist_id
WHERE last_ingested_data > '2021-01-01'
ORDER BY 1, 2;
`)
	if err != nil {
		sklog.Fatalf("getting newish CLs from DB %s", err)
	}
	defer rows.Close()
	var toMigrate []tjstore.CombinedPSID
	var lastUpdatedTimestamps []time.Time
	for rows.Next() {
		var cid tjstore.CombinedPSID
		var qCLID string
		var qPSID string
		var ts time.Time
		if err := rows.Scan(&qCLID, &cid.CRS, &qPSID, &ts); err != nil {
			sklog.Fatalf("Invalid row: %s")
		}
		cid.CL = sql.Unqualify(qCLID)
		cid.PS = sql.Unqualify(qPSID)
		toMigrate = append(toMigrate, cid)
		lastUpdatedTimestamps = append(lastUpdatedTimestamps, ts.UTC())
	}
	rows.Close()

	sklog.Infof("There are %d PS with Tryjob data that needs migrating", len(toMigrate))

	if len(toMigrate) == 0 || len(lastUpdatedTimestamps) == 0 {
		sklog.Infof("Trivially done")
		return
	}

	sqlStore := sqltjstore.New(db)

	workChan := make(chan int, len(toMigrate))
	for i := range toMigrate {
		workChan <- i
	}
	close(workChan)

	wg := sync.WaitGroup{}
	const numChunks = 4
	wg.Add(4)
	for i := 0; i < numChunks; i++ {
		go func() {
			defer wg.Done()
			for idx := range workChan {
				if err := ctx.Err(); err != nil {
					sklog.Errorf("Context error: %s", err)
					return
				}
				cid := toMigrate[idx]
				ts := lastUpdatedTimestamps[idx]
				sklog.Infof("%v", cid)
				tryjobs, err := oldStore.GetTryJobs(ctx, cid)
				if err != nil {
					sklog.Fatalf("Error fetching tryjobs from FS for %v: %s", cid, err)
				}
				data, err := oldStore.GetResults(ctx, cid, time.Time{})
				if err != nil {
					sklog.Fatalf("Error fetching data from FS for %v: %s", cid, err)
				}
				err = storeTryJobs(ctx, db, cid, tryjobs)
				if err != nil {
					sklog.Fatalf("Error writing tryjobs (len %d) to SQL for %v: %s", len(tryjobs), cid, err)
				}
				err = sqlStore.PutResults(ctx, cid, "unknown (migrated from FS)", data, ts)
				if err != nil {
					sklog.Fatalf("Error writing data (len %d) to SQL for %v: %s", len(data), cid, err)
				}
			}
		}()
	}
	wg.Wait()
	sklog.Infof("Done")
}

func storeTryJobs(ctx context.Context, db *pgxpool.Pool, cID tjstore.CombinedPSID, xtj []ci.TryJob) error {
	const statement = `
UPSERT INTO Tryjobs (tryjob_id, system, changelist_id, patchset_id, display_name, last_ingested_data)
VALUES `
	const valuesPerRow = 6
	placeholders := sql.ValuesPlaceholders(valuesPerRow, len(xtj))
	arguments := make([]interface{}, 0, valuesPerRow*len(xtj))
	clID := sql.Qualify(cID.CRS, cID.CL)
	psID := sql.Qualify(cID.CRS, cID.PS)

	for _, tj := range xtj {
		tjID := sql.Qualify(tj.System, tj.SystemID)
		arguments = append(arguments, tjID, tj.System, clID, psID, tj.DisplayName, tj.Updated)
	}
	_, err := db.Exec(ctx, statement+placeholders, arguments...)
	if err != nil {
		return skerr.Wrapf(err, "Storing %d Tryjobs", len(xtj))
	}
	return nil
}
