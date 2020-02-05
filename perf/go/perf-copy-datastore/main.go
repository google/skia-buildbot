// Copies Perf data from one project to another.
package main

import (
	"context"
	"flag"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/activitylog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	srcProject = flag.String("src_project", "google.com:skia-buildbots", "The source project.")
	dstProject = flag.String("dst_project", "skia-public", "The destination project.")
	namespace  = flag.String("namespace", "perf", "The Cloud Datastore namespace, such as 'perf'.")
)

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

	// Copy Alerts.
	q := ds.NewQuery(ds.ALERT)
	for t := srcClient.Run(ctx, q); ; {
		var x alerts.Alert
		key, err := t.Next(&x)
		if err == iterator.Done {
			break
		}
		if err != nil {
			sklog.Fatal(err)
		}
		_, err = dstClient.Put(ctx, key, &x)
		sklog.Infof("Alert: %s", key)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Copy Regressions.
	q = ds.NewQuery(ds.REGRESSION)
	for t := srcClient.Run(ctx, q); ; {
		var x regression.DSRegression
		key, err := t.Next(&x)
		if err == iterator.Done {
			break
		}
		if err != nil {
			sklog.Fatal(err)
		}
		_, err = dstClient.Put(ctx, key, &x)
		sklog.Infof("Regression: %s", key)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Copy Activities.
	q = ds.NewQuery(ds.ACTIVITY)
	for t := srcClient.Run(ctx, q); ; {
		var x activitylog.Activity
		key, err := t.Next(&x)
		if err == iterator.Done {
			break
		}
		if err != nil {
			sklog.Fatal(err)
		}
		_, err = dstClient.Put(ctx, key, &x)
		sklog.Infof("Activity: %s", key)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Copy Shortcuts.
	q = ds.NewQuery(ds.SHORTCUT)
	for t := srcClient.Run(ctx, q); ; {
		var x shortcut2.Shortcut
		key, err := t.Next(&x)
		if err == iterator.Done {
			break
		}
		if err != nil {
			sklog.Fatal(err)
		}
		_, err = dstClient.Put(ctx, key, &x)
		sklog.Infof("Shortcut: %s", key)
		if err != nil {
			sklog.Fatal(err)
		}
	}
}
