// The fsmigrator executable migrates various data from firestore to an SQL database.
// It uses port forwarding, as that is the simplest approach and there shouldn't be
// too much data.
package main

import (
	"context"
	"flag"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/clstore/fs_clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
)

func main() {
	var (
		fsProjectID          = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		oldFSNamespace       = flag.String("old_fs_namespace", "", "Typically the instance id. e.g. 'chrome-gpu', 'skia', etc")
		newSQLDatabase       = flag.String("new_sql_db", "", "Something like the instance id (no dashes)")
		crsList              = flag.String("crs_list", "", "comma-separated list of valid Code Review Systems")
		remakePatchSetSchema = flag.Bool("remake_ps_schema", false, "If true, will drop existing Patchsets table (and dependencies) and remake it.")
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

	if *remakePatchSetSchema {
		sklog.Infof("Waiting 3s before dropping tables. Stop now if that is accidental.")
		time.Sleep(3 * time.Second)
		// Tryjobs table has Patchsets foreign key, so it must be dropped first.
		if _, err := db.Exec(ctx, `DROP TABLE Tryjobs`); err != nil {
			sklog.Fatalf("could not drop existing table Tryjobs: %s", err)
		}
		if _, err := db.Exec(ctx, `DROP TABLE Patchsets`); err != nil {
			sklog.Fatalf("could not drop existing table Patchsets: %s", err)
		}
		if _, err := db.Exec(ctx, schema.Schema); err != nil {
			sklog.Fatalf("could not recreate table(s): %s", err)
		}
	}

	crsNames := strings.Split(*crsList, ",")
	for _, crs := range crsNames {
		old := fs_clstore.New(fsClient, crs)

		start := 0
		const limit = 500
		const numChunks = 8
		for {
			xcl, _, err := old.GetChangelists(ctx, clstore.SearchOptions{
				StartIdx: start,
				Limit:    limit,
			})
			if err != nil {
				sklog.Fatalf("Fetching Changelists for system %s %d: %s", crs, start, err)
			}
			if len(xcl) == 0 {
				break
			}
			if err := storeCLsToSQL(ctx, db, crs, xcl); err != nil {
				sklog.Fatalf("Storing CLs for system %s %d: %s", crs, start, err)
			}
			// For each CL, fetch the patchsets associated with it in parallel.
			patchsetsPerCL := make([][]code_review.Patchset, len(xcl))
			chunkSize := (len(xcl) / numChunks) + 1 // add one to avoid integer truncation.
			err = util.ChunkIterParallel(ctx, len(xcl), chunkSize, func(ctx context.Context, startIdx int, endIdx int) error {
				for i, cl := range xcl[startIdx:endIdx] {
					if err := ctx.Err(); err != nil {
						sklog.Errorf("Context error: %s", err)
						return nil
					}
					xps, err := old.GetPatchsets(ctx, cl.SystemID)
					if err != nil {
						return skerr.Wrapf(err, "fetching patchsets for %s-%s", crs, cl.SystemID)
					}
					if len(xps) == 0 {
						sklog.Infof("CL %s has no Patchsets", cl.SystemID)
					}
					patchsetsPerCL[startIdx+i] = xps
				}
				return nil
			})
			if err != nil {
				sklog.Fatalf("Fetching patchsets: %s", err)
			}
			if err := storePSsToSQL(ctx, db, crs, patchsetsPerCL); err != nil {
				sklog.Fatalf("Storing PSs for system %s %d: %s", crs, start, err)
			}

			sklog.Infof("Stored data for %d CLs and Patchsets (%d)", len(xcl), start)
			if len(xcl) != limit {
				break
			}
			start += len(xcl)
		}
		sklog.Infof("Done with %s", crs)
	}
}

func qualify(system, id string) string {
	return system + "_" + id
}

func convertFromStatusEnum(status code_review.CLStatus) schema.ChangelistStatus {
	switch status {
	case code_review.Abandoned:
		return schema.StatusAbandoned
	case code_review.Open:
		return schema.StatusOpen
	case code_review.Landed:
		return schema.StatusLanded
	}
	sklog.Warningf("Unknown status: %d", status)
	return schema.StatusAbandoned
}

func storeCLsToSQL(ctx context.Context, db *pgxpool.Pool, crs string, xcl []code_review.Changelist) error {
	const statement = `
UPSERT INTO Changelists (changelist_id, system, status, owner_email, subject, last_ingested_data) VALUES `
	const valuesPerRow = 6
	placeholders := sql.ValuesPlaceholders(valuesPerRow, len(xcl))
	arguments := make([]interface{}, 0, valuesPerRow*len(xcl))
	for _, cl := range xcl {
		arguments = append(arguments, qualify(crs, cl.SystemID), crs,
			convertFromStatusEnum(cl.Status), cl.Owner, cl.Subject, cl.Updated)
	}
	_, err := db.Exec(ctx, statement+placeholders, arguments...)
	if err != nil {
		return skerr.Wrapf(err, "storing %d CLs", len(xcl))
	}
	return nil
}

func storePSsToSQL(ctx context.Context, db *pgxpool.Pool, crs string, patchsets [][]code_review.Patchset) error {
	const statement = `
UPSERT INTO Patchsets (patchset_id, system, changelist_id, ps_order, git_hash,
  commented_on_cl, last_checked_if_comment_necessary) VALUES `
	const valuesPerRow = 7

	arguments := make([]interface{}, 0, valuesPerRow*len(patchsets))
	totalPS := 0
	for i := range patchsets {
		for _, ps := range patchsets[i] {
			totalPS++
			arguments = append(arguments, qualify(crs, ps.SystemID), crs, qualify(crs, ps.ChangelistID),
				ps.Order, ps.GitHash, ps.CommentedOnCL, ps.LastCheckedIfCommentNecessary)
		}
	}
	if totalPS == 0 {
		return nil
	}
	placeholders := sql.ValuesPlaceholders(valuesPerRow, totalPS)
	_, err := db.Exec(ctx, statement+placeholders, arguments...)
	if err != nil {
		return skerr.Wrapf(err, "storing %d PSs", totalPS)
	}
	sklog.Infof("Stored %d PSs", totalPS)
	return nil
}
