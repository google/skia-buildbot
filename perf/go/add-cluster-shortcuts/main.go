// A command-line tool to add Shortcut values to every Regression in the Datastore.
package main

import (
	"context"
	"encoding/json"
	"flag"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// flags
var (
	projectName = flag.String("project_name", "skia-public", "The Google Cloud project name.")
	namespace   = flag.String("namespace", "perf-localhost-jcgregorio", "The Cloud Datastore namespace, such as 'perf'.")
)

func main() {
	common.Init()
	ts, err := auth.NewDefaultTokenSource(true, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatalf("Failed to get TokenSource: %s", err)
	}
	if err := ds.InitWithOpt(*projectName, *namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatalf("Failed to init Cloud Datastore: %s", err)
	}
	q := ds.NewQuery(ds.REGRESSION)
	ctx := context.Background()
	it := ds.DS.Run(ctx, q)
	for {
		var dsRegression regression.DSRegression
		key, err := it.Next(&dsRegression)
		if err == iterator.Done {
			break
		} else if err != nil {
			sklog.Fatal(err)
		}
		sklog.Infof("Key: %q", key)
		reg := regression.New()
		if err := json.Unmarshal([]byte(dsRegression.Body), reg); err != nil {
			sklog.Fatal(err)
		}
		for _, r := range reg.ByAlertID {
			var err error
			if r.High != nil {
				sklog.Infof("High shortcut before: %q", r.High.Shortcut)
				if r.High.Shortcut, err = shortcut2.InsertShortcut(&shortcut2.Shortcut{Keys: r.High.Keys}); err != nil {
					sklog.Fatal(err)
				}
				sklog.Infof("shortcut: %q", r.High.Shortcut)
			}
			if r.Low != nil {
				sklog.Infof("Low shortcut before: %q", r.Low.Shortcut)
				if r.Low.Shortcut, err = shortcut2.InsertShortcut(&shortcut2.Shortcut{Keys: r.Low.Keys}); err != nil {
					sklog.Fatal(err)
				}
				sklog.Infof("shortcut: %q", r.Low.Shortcut)
			}
		}
		// Reserialize back to JSON and write back to DS.
		b, err := json.Marshal(reg)
		if err != nil {
			sklog.Fatal(err)
		}
		dsRegression.Body = string(b)
		if _, err = ds.DS.Put(ctx, key, &dsRegression); err != nil {
			sklog.Fatal(err)
		}
	}
}
