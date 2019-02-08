// Copies CT data from one cloud project to another.
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/ct/go/ctfe/task_types"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	srcProject = flag.String("src_project", "google.com:skia-buildbots", "The source project.")
	dstProject = flag.String("dst_project", "skia-public", "The destination project.")
	namespace  = flag.String("namespace", "cluster-telemetry-testing", "The Cloud Datastore namespace, such as 'cluster-telemetry'.")
)

func migrateData(ctx context.Context, srcClient, dstClient *datastore.Client, kind ds.Kind) {
	// Delete everything from the datastore first.
	if _, err := DeleteKind(ctx, dstClient, kind); err != nil {
		sklog.Fatal(err)
	}
	// Migrate data.
	q := ds.NewQuery(kind)
	for t := srcClient.Run(ctx, q); ; {
		pl := &datastore.PropertyList{}
		key, err := t.Next(pl)
		if err == iterator.Done {
			break
		}
		if err != nil {
			sklog.Fatal(err)
		}
		_, err = dstClient.Put(ctx, key, pl)
		sklog.Infof("%s: %s", kind, key)
		if err != nil {
			sklog.Fatal(err)
		}
	}
}

func main() {
	common.Init()

	// Construct clients.
	ts, err := auth.NewDefaultTokenSource(true, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := ds.InitWithOpt(*srcProject, *namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}
	srcClient := ds.DS
	if err := ds.InitWithOpt(*dstProject, *namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}
	dstClient := ds.DS

	ctx := context.Background()

	// Gather all the kinds we want to migrate.
	kinds := []ds.Kind{}
	for _, t := range task_types.Prototypes() {
		kinds = append(kinds, t.GetDatastoreKind())
	}
	kinds = append(kinds, ds.CLUSTER_TELEMETRY_IDS)

	// Start the migration.
	defer util.TimeTrack(time.Now(), "Database migration")
	for _, k := range kinds {
		migrateData(ctx, srcClient, dstClient, k)
	}
	sklog.Infoln("Database migration finished.")
}

func DeleteKind(ctx context.Context, dstClient *datastore.Client, datastoreKind ds.Kind) (int, error) {
	var i int
	var lastSeenKey *datastore.Key
	q := ds.NewQuery(datastoreKind).Limit(500).KeysOnly().Order("__key__")
	timeout := time.After(time.Second * 60)
	for {
		select {
		case <-timeout:
			{
				return i, fmt.Errorf("Could not delete all entries of %s within the time limit", datastoreKind)
			}
		default:
			{
				keys, err := dstClient.GetAll(ctx, q, nil)
				if err != nil || len(keys) == 0 {
					return i, err
				} else {
					lastSeenKey = keys[len(keys)-1]
					i = i + len(keys)
					if err := dstClient.DeleteMulti(ctx, keys); err != nil {
						return i, err
					}
					sklog.Infof("Deleted %s. Entries deleted: %d", datastoreKind, i)
				}
				q = ds.NewQuery(datastoreKind).Limit(500).KeysOnly().Order("__key__").Filter("__key__ >", lastSeenKey)
			}
		}
	}
	return i, nil
}
