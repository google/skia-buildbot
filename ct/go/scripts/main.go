package main

import (
	"context"
	"flag"
	"fmt"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/metrics_analysis"
)

// Command line flags
var (
	dsNamespace = flag.String("ds_namespace", "cluster-telemetry-staging", "Cloud datastore namespace to be used by this instance.")
	projectID   = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")

	WORKER_POOL_SIZE = 10
	// Tried 10
	// Tried 25
	// Tried 50
)

const (
	AFTER_Q1 = 20190301000000
)

// List of entities we are importing
var targetKinds = []ds.Kind{
	ds.RECREATE_PAGESETS_TASKS,
	ds.RECREATE_WEBPAGE_ARCHIVES_TASKS,
	ds.CAPTURE_SKPS_TASKS,
	ds.CHROMIUM_ANALYSIS_TASKS,
	ds.CHROMIUM_BUILD_TASKS,
	ds.CHROMIUM_PERF_TASKS,
	ds.METRICS_ANALYSIS_TASKS,
	ds.LUA_SCRIPT_TASKS,
}

func main() {
	// Global init to initialize logging and parse arguments.
	common.Init()
	skiaversion.MustLogVersion()

	if err := ds.InitWithOpt(*projectID, *dsNamespace); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}
	//dsClient := ds.DS

	ctx := context.Background()
	getUsagePerf(ctx)
	getUsageAnalysis(ctx)
	getUsageMetricsAnalysis(ctx)
	getUsageLua(ctx)
}

func getUsagePerf(ctx context.Context) {
	kind := ds.CHROMIUM_PERF_TASKS

	query := ds.NewQuery(kind).Filter("TsAdded >", AFTER_Q1)
	var results []*chromium_perf.DatastoreTask
	_, err := ds.DS.GetAll(ctx, query, &results)
	if err != nil {
		sklog.Fatal(err)
	}

	uniqueUsersToCounts := map[string]int{}
	totalCount := 0
	for _, r := range results {
		if r.CommonCols.Username == "rmistry@google.com" || r.CommonCols.Failure != false {
			continue
		}
		totalCount++
		user := r.Username
		if val, ok := uniqueUsersToCounts[user]; ok {
			uniqueUsersToCounts[user] = val + 1
		} else {
			uniqueUsersToCounts[user] = 1
		}
	}
	fmt.Printf("Total runs of %s with user != rmistry@google.com and tsAdded > %d are: %d\n", kind, AFTER_Q1, totalCount)
	fmt.Printf("Unique users: %d\n", len(uniqueUsersToCounts))
	for k, v := range uniqueUsersToCounts {
		fmt.Printf("  %s: %d\n", k, v)
	}
	fmt.Println("---------------------------------------------")
}

func getUsageAnalysis(ctx context.Context) {
	kind := ds.CHROMIUM_ANALYSIS_TASKS

	query := ds.NewQuery(kind).Filter("TsAdded >", AFTER_Q1)
	var results []*chromium_analysis.DatastoreTask
	_, err := ds.DS.GetAll(ctx, query, &results)
	if err != nil {
		sklog.Fatal(err)
	}

	uniqueUsersToCounts := map[string]int{}
	totalCount := 0
	for _, r := range results {
		if r.CommonCols.Username == "rmistry@google.com" || r.CommonCols.Failure != false {
			continue
		}
		totalCount++
		user := r.Username
		if val, ok := uniqueUsersToCounts[user]; ok {
			uniqueUsersToCounts[user] = val + 1
		} else {
			uniqueUsersToCounts[user] = 1
		}
	}
	fmt.Printf("Total runs of %s with user != rmistry@google.com and tsAdded > %d are: %d\n", kind, AFTER_Q1, totalCount)
	fmt.Printf("Unique users: %d\n", len(uniqueUsersToCounts))
	for k, v := range uniqueUsersToCounts {
		fmt.Printf("  %s: %d\n", k, v)
	}
	fmt.Println("---------------------------------------------")
}

func getUsageMetricsAnalysis(ctx context.Context) {
	kind := ds.METRICS_ANALYSIS_TASKS

	query := ds.NewQuery(kind).Filter("TsAdded >", AFTER_Q1)
	var results []*metrics_analysis.DatastoreTask
	_, err := ds.DS.GetAll(ctx, query, &results)
	if err != nil {
		sklog.Fatal(err)
	}

	uniqueUsersToCounts := map[string]int{}
	totalCount := 0
	for _, r := range results {
		if r.CommonCols.Username == "rmistry@google.com" || r.CommonCols.Failure != false {
			continue
		}
		totalCount++
		user := r.Username
		if val, ok := uniqueUsersToCounts[user]; ok {
			uniqueUsersToCounts[user] = val + 1
		} else {
			uniqueUsersToCounts[user] = 1
		}
	}
	fmt.Printf("Total runs of %s with user != rmistry@google.com and tsAdded > %d are: %d\n", kind, AFTER_Q1, totalCount)
	fmt.Printf("Unique users: %d\n", len(uniqueUsersToCounts))
	for k, v := range uniqueUsersToCounts {
		fmt.Printf("  %s: %d\n", k, v)
	}
	fmt.Println("---------------------------------------------")
}

func getUsageLua(ctx context.Context) {
	kind := ds.LUA_SCRIPT_TASKS

	query := ds.NewQuery(kind).Filter("TsAdded >", AFTER_Q1)
	var results []*lua_scripts.DatastoreTask
	_, err := ds.DS.GetAll(ctx, query, &results)
	if err != nil {
		sklog.Fatal(err)
	}

	uniqueUsersToCounts := map[string]int{}
	totalCount := 0
	for _, r := range results {
		if r.CommonCols.Username == "rmistry@google.com" || r.CommonCols.Failure != false {
			continue
		}
		totalCount++
		user := r.Username
		if val, ok := uniqueUsersToCounts[user]; ok {
			uniqueUsersToCounts[user] = val + 1
		} else {
			uniqueUsersToCounts[user] = 1
		}
	}
	fmt.Printf("Total runs of %s with user != rmistry@google.com and tsAdded > %d are: %d\n", kind, AFTER_Q1, totalCount)
	fmt.Printf("Unique users: %d\n", len(uniqueUsersToCounts))
	for k, v := range uniqueUsersToCounts {
		fmt.Printf("  %s: %d\n", k, v)
	}
	fmt.Println("---------------------------------------------")
}
