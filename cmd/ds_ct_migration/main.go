package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/jmoiron/sqlx"

	"go.skia.org/infra/ct/go/db"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"

	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
)

// Command line flags
var (
	dsNamespace    = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	projectID      = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
	promptPassword = flag.Bool("password", false, "Prompt for root password.")

	DB *sqlx.DB = nil

	WORKER_POOL_SIZE = 50
)

//CAPTURE_SKPS_TASKS              Kind = "CaptureSkpsTasks"
//CHROMIUM_ANALYSIS_TASKS         Kind = "ChromiumAnalysisTasks"
//CHROMIUM_BUILD_TASKS            Kind = "ChromiumBuildTasks"
//CHROMIUM_PERF_TASKS             Kind = "ChromiumPerfTasks"
//LUA_SCRIPT_TASKS                Kind = "LuaScriptTasks"
//METRICS_ANALYSIS_TASKS          Kind = "MetricsAnalysisTasks"
//PIXEL_DIFF_TASKS                Kind = "PixelDiffTasks"
//RECREATE_PAGESETS_TASKS         Kind = "RecreatePageSetsTasks"
//RECREATE_WEBPAGE_ARCHIVES_TASKS Kind = "RecreateWebpageArchivesTasks"
//CLUSTER_TELEMETRY_IDS           Kind = "ClusterTelemetryIDs"

// List of entities we are importing
var targetKinds = []ds.Kind{
	ds.RECREATE_PAGESETS_TASKS,
	ds.RECREATE_WEBPAGE_ARCHIVES_TASKS,
	ds.CAPTURE_SKPS_TASKS,
	ds.CHROMIUM_ANALYSIS_TASKS,
	ds.CHROMIUM_BUILD_TASKS,
	ds.CHROMIUM_PERF_TASKS,
	ds.METRICS_ANALYSIS_TASKS,
	ds.PIXEL_DIFF_TASKS,
}

func main() {
	// Configure the MySQL database
	dbConf := database.ConfigFromFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_RW, db.PROD_DB_NAME, db.MigrationSteps())

	// Global init to initialize logging and parse arguments.
	common.Init()
	skiaversion.MustLogVersion()

	// Set up the SQL based expectations store
	vdb := setupMySQL(dbConf, *promptPassword)
	DB = sqlx.NewDb(vdb.DB, database.DEFAULT_DRIVER)

	// Set up the cloud data store based expectations store
	// Needed to use TimeSortableKey(...) which relies on an RNG. See docs there.
	rand.Seed(time.Now().UnixNano())
	if err := ds.InitWithOpt(*projectID, *dsNamespace); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}
	dsClient := ds.DS

	ctx := context.Background()
	//readWriteCaptureSKPs(ctx)
	//readWriteRecreatePageSets(ctx)
	//readWriteRecreateWebpageArchives(ctx)
	readWriteChromiumAnalysis(ctx)

	scanExisting(dsClient, targetKinds)

	sklog.Infoln("Database migration finished.")
}

type DSCommonCols struct {
	DatastoreKey    *datastore.Key `datastore:"__key__"`
	TsAdded         int64
	TsStarted       int64
	TsCompleted     int64
	Username        string
	Failure         bool
	RepeatAfterDays int64
	SwarmingLogs    string
	TaskDone        bool
}

type ClusterTelemetryIDs struct {
	HighestID int64
}

func AddHighestID(ctx context.Context, kind ds.Kind, highestID int64) {
	key := ds.NewKey(ds.CLUSTER_TELEMETRY_IDS)
	key.Name = string(kind)
	ids := ClusterTelemetryIDs{
		HighestID: highestID,
	}
	_, err := ds.DS.Put(ctx, key, &ids)
	if err != nil {
		sklog.Fatal(err)
	}
}

type DSCaptureSKPs struct {
	DSCommonCols

	PageSets      string
	IsTestPageSet bool
	ChromiumRev   string
	SkiaRev       string
	Description   string
}

type DSRecreatePageSets struct {
	DSCommonCols

	PageSets      string
	IsTestPageSet bool
}

type DSRecreateWebpageArchives struct {
	DSCommonCols

	PageSets      string
	IsTestPageSet bool
	ChromiumRev   string
	SkiaRev       string
}

type DSChromiumAnalysis struct {
	DSCommonCols

	Benchmark            string
	PageSets             string
	IsTestPageSet        bool
	BenchmarkArgs        string
	BrowserArgs          string
	Description          string
	CustomWebpagesGSPath string
	ChromiumPatchGSPath  string
	SkiaPatchGSPath      string
	CatapultPatchGSPath  string
	BenchmarkPatchGSPath string
	V8PatchGSPath        string
	RunInParallel        bool
	Platform             string
	RunOnGCE             bool
	RawOutput            string
	MatchStdoutTxt       string

	CustomWebpages string `datastore:"-"`
	ChromiumPatch  string `datastore:"-"`
	SkiaPatch      string `datastore:"-"`
	CatapultPatch  string `datastore:"-"`
	BenchmarkPatch string `datastore:"-"`
	V8Patch        string `datastore:"-"`
}

type DSChromiumBuilds struct {
	DSCommonCols

	ChromiumRev   string
	ChromiumRevTs int64
	SkiaRev       string
}

type DSChromiumPerf struct {
	DSCommonCols

	Benchmark            string
	Platform             string
	PageSets             string
	IsTestPageSet        bool
	RepeatRuns           int64
	RunInParallel        bool
	BenchmarkArgs        string
	BrowserArgsNoPatch   string
	BrowserArgsWithPatch string
	Description          string
	CustomWebpagesGSPath string
	ChromiumPatchGSPath  string
	BlinkPatchGSPath     string
	SkiaPatchGSPath      string
	CatapultPatchGSPath  string
	BenchmarkPatchGSPath string
	V8PatchGSPath        string
	Results              string
	NoPatchRawOutput     string
	WithPatchRawOutput   string

	CustomWebpages string `datastore:"-"`
	ChromiumPatch  string `datastore:"-"`
	BlinkPatch     string `datastore:"-"`
	SkiaPatch      string `datastore:"-"`
	CatapultPatch  string `datastore:"-"`
	BenchmarkPatch string `datastore:"-"`
	V8Patch        string `datastore:"-"`
}

type DSLuaScripts struct {
	DSCommonCols

	PageSets            string
	ChromiumRev         string
	SkiaRev             string
	LuaScript           string
	LuaAggregatorScript string
	Description         string
	ScriptOutput        string
	AggregatedOutput    string
}

type DSMetricsAnalysis struct {
	DSCommonCols

	MetricName          string
	AnalysisTaskId      string
	AnalysisOutputLink  string
	BenchmarkArgs       string
	Description         string
	CustomTracesGSPath  string
	ChromiumPatchGSPath string
	CatapultPatchGSPath string
	RawOutput           string

	CustomTraces  string `datastore:"-"`
	ChromiumPatch string `datastore:"-"`
	CatapultPatch string `datastore:"-"`
}

type DSPixelDiff struct {
	DSCommonCols

	PageSets             string
	IsTestPageSet        bool
	BenchmarkArgs        string
	BrowserArgsNoPatch   string
	BrowserArgsWithPatch string
	Description          string
	CustomWebpagesGSPath string
	ChromiumPatchGSPath  string
	SkiaPatchGSPath      string
	Results              string

	CustomWebpages string `datastore:"-"`
	ChromiumPatch  string `datastore:"-"`
	SkiaPatch      string `datastore:"-"`
}

func readWriteRecreatePageSets(ctx context.Context) {
	table := db.TABLE_RECREATE_PAGE_SETS_TASKS
	kind := ds.RECREATE_PAGESETS_TASKS
	// 3 places below.

	// Delete everything from the datastore first.
	if _, err := DeleteKind(ctx, kind); err != nil {
		sklog.Fatal(err)
	}

	tasks := []admin_tasks.RecreatePageSetsDBTask{}
	rowQuery := fmt.Sprintf("SELECT * FROM %s", table)

	rows, err := DB.Queryx(rowQuery)
	if err != nil {
		sklog.Fatalf("Could not query %s: %s", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		t := admin_tasks.RecreatePageSetsDBTask{}
		err := rows.StructScan(&t)
		if err != nil {
			sklog.Fatal(err)
		}
		tasks = append(tasks, t)
	}
	fmt.Println(len(tasks))
	highestID := int64(0)
	for _, t := range tasks {
		// Common Cols
		dsTask := &DSRecreatePageSets{}
		dsTask.Username = t.Username
		if t.Failure.Valid {
			dsTask.Failure = t.Failure.Bool
		}
		dsTask.RepeatAfterDays = t.RepeatAfterDays
		if t.SwarmingLogs.Valid {
			dsTask.SwarmingLogs = t.SwarmingLogs.String
		}
		if t.TsAdded.Valid {
			dsTask.TsAdded = t.TsAdded.Int64
		}
		if t.TsStarted.Valid {
			dsTask.TsStarted = t.TsStarted.Int64
		}
		if t.TsCompleted.Valid {
			dsTask.TsCompleted = t.TsCompleted.Int64
			dsTask.TaskDone = true
		}

		// Task specific cols.
		dsTask.PageSets = t.PageSets
		dsTask.IsTestPageSet = t.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k

		// Key
		key := ds.NewKey(kind)
		key.ID = t.Id
		highestID = util.MaxInt64(key.ID, highestID)
		dsTask.DatastoreKey = key

		_, err := ds.DS.Put(ctx, key, dsTask)
		if err != nil {
			sklog.Fatal("Error putting task in datastore: %s", err)
		}
	}
	AddHighestID(ctx, kind, highestID)
}

func readWriteRecreateWebpageArchives(ctx context.Context) {
	table := db.TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS
	kind := ds.RECREATE_WEBPAGE_ARCHIVES_TASKS
	// 3 places below.

	// Delete everything from the datastore first.
	if _, err := DeleteKind(ctx, kind); err != nil {
		sklog.Fatal(err)
	}

	tasks := []admin_tasks.RecreateWebpageArchivesDBTask{}
	rowQuery := fmt.Sprintf("SELECT * FROM %s", table)

	rows, err := DB.Queryx(rowQuery)
	if err != nil {
		sklog.Fatalf("Could not query %s: %s", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		t := admin_tasks.RecreateWebpageArchivesDBTask{}
		err := rows.StructScan(&t)
		if err != nil {
			sklog.Fatal(err)
		}
		tasks = append(tasks, t)
	}
	fmt.Println(len(tasks))
	highestID := int64(0)
	for _, t := range tasks {
		// Common Cols
		dsTask := &DSRecreateWebpageArchives{}
		dsTask.Username = t.Username
		if t.Failure.Valid {
			dsTask.Failure = t.Failure.Bool
		}
		dsTask.RepeatAfterDays = t.RepeatAfterDays
		if t.SwarmingLogs.Valid {
			dsTask.SwarmingLogs = t.SwarmingLogs.String
		}
		if t.TsAdded.Valid {
			dsTask.TsAdded = t.TsAdded.Int64
		}
		if t.TsStarted.Valid {
			dsTask.TsStarted = t.TsStarted.Int64
		}
		if t.TsCompleted.Valid {
			dsTask.TsCompleted = t.TsCompleted.Int64
			dsTask.TaskDone = true
		}

		// Task specific cols.
		dsTask.PageSets = t.PageSets
		dsTask.IsTestPageSet = t.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k
		dsTask.ChromiumRev = t.ChromiumRev
		dsTask.SkiaRev = t.SkiaRev

		// Key
		key := ds.NewKey(kind)
		key.ID = t.Id
		highestID = util.MaxInt64(key.ID, highestID)
		dsTask.DatastoreKey = key

		_, err := ds.DS.Put(ctx, key, dsTask)
		if err != nil {
			sklog.Fatal("Error putting task in datastore: %s", err)
		}
	}
	AddHighestID(ctx, kind, highestID)
}

func readWriteCaptureSKPs(ctx context.Context) {
	// Delete everything from the datastore first.
	if _, err := DeleteKind(ctx, ds.CAPTURE_SKPS_TASKS); err != nil {
		sklog.Fatal(err)
	}

	tasks := []capture_skps.DBTask{}
	rowQuery := fmt.Sprintf("SELECT * FROM %s", db.TABLE_CAPTURE_SKPS_TASKS)

	rows, err := DB.Queryx(rowQuery)
	if err != nil {
		sklog.Fatalf("Could not query %s: %s", db.TABLE_CAPTURE_SKPS_TASKS, err)
	}
	defer rows.Close()
	for rows.Next() {
		t := capture_skps.DBTask{}
		err := rows.StructScan(&t)
		if err != nil {
			sklog.Fatal(err)
		}
		tasks = append(tasks, t)
	}
	fmt.Println(len(tasks))
	highestID := int64(0)
	for _, t := range tasks {
		// Common Cols
		dsTask := &DSCaptureSKPs{}
		dsTask.Username = t.Username
		if t.Failure.Valid {
			dsTask.Failure = t.Failure.Bool
		}
		dsTask.RepeatAfterDays = t.RepeatAfterDays
		if t.SwarmingLogs.Valid {
			dsTask.SwarmingLogs = t.SwarmingLogs.String
		}
		if t.TsAdded.Valid {
			dsTask.TsAdded = t.TsAdded.Int64
		}
		if t.TsStarted.Valid {
			dsTask.TsStarted = t.TsStarted.Int64
		}
		if t.TsCompleted.Valid {
			dsTask.TsCompleted = t.TsCompleted.Int64
			dsTask.TaskDone = true
		}

		// Task specific cols.
		dsTask.PageSets = t.PageSets
		dsTask.IsTestPageSet = t.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k
		dsTask.ChromiumRev = t.ChromiumRev
		dsTask.SkiaRev = t.SkiaRev
		dsTask.Description = t.Description

		// Key
		key := ds.NewKey(ds.CAPTURE_SKPS_TASKS)
		key.ID = t.Id
		highestID = util.MaxInt64(key.ID, highestID)
		dsTask.DatastoreKey = key

		_, err := ds.DS.Put(ctx, key, dsTask)
		if err != nil {
			sklog.Fatal("Error putting task in datastore: %s", err)
		}
	}
	AddHighestID(ctx, ds.CAPTURE_SKPS_TASKS, highestID)
}

func readWriteChromiumAnalysis(ctx context.Context) {

	table := db.TABLE_CHROMIUM_ANALYSIS_TASKS
	kind := ds.CHROMIUM_ANALYSIS_TASKS
	// 3 places below.

	// Delete everything from the datastore first.
	if _, err := DeleteKind(ctx, kind); err != nil {
		sklog.Fatal(err)
	}

	tasks := []chromium_analysis.DBTask{}
	rowQuery := fmt.Sprintf("SELECT * FROM %s", table)

	rows, err := DB.Queryx(rowQuery)
	if err != nil {
		sklog.Fatalf("Could not query %s: %s", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		t := chromium_analysis.DBTask{}
		err := rows.StructScan(&t)
		if err != nil {
			sklog.Fatal(err)
		}
		tasks = append(tasks, t)
	}
	fmt.Println(len(tasks))

	var wg sync.WaitGroup
	tasksChannel := make(chan chromium_analysis.DBTask, len(tasks))
	for _, t := range tasks {
		tasksChannel <- t
	}
	close(tasksChannel)

	highestID := int64(0)
	var highestIDMutex sync.Mutex
	// Loop through workers in the worker pool.
	for i := 0; i < WORKER_POOL_SIZE; i++ {
		// Increment the WaitGroup counter.
		wg.Add(1)

		// Create and run a goroutine closure that captures SKPs.
		go func() {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()

			for t := range tasksChannel {
				// Common Cols
				dsTask := &DSChromiumAnalysis{}
				dsTask.Username = t.Username
				if t.Failure.Valid {
					dsTask.Failure = t.Failure.Bool
				}
				dsTask.RepeatAfterDays = t.RepeatAfterDays
				if t.SwarmingLogs.Valid {
					dsTask.SwarmingLogs = t.SwarmingLogs.String
				}
				if t.TsAdded.Valid {
					dsTask.TsAdded = t.TsAdded.Int64
				}
				if t.TsStarted.Valid {
					dsTask.TsStarted = t.TsStarted.Int64
				}
				if t.TsCompleted.Valid {
					dsTask.TsCompleted = t.TsCompleted.Int64
					dsTask.TaskDone = true
				}

				// Task specific cols.
				dsTask.Benchmark = t.Benchmark
				dsTask.PageSets = t.PageSets
				dsTask.IsTestPageSet = t.PageSets == ctutil.PAGESET_TYPE_DUMMY_1k
				dsTask.BenchmarkArgs = t.BenchmarkArgs
				dsTask.BrowserArgs = t.BrowserArgs
				dsTask.Description = t.Description
				dsTask.RunInParallel = t.RunInParallel
				dsTask.Platform = t.Platform
				dsTask.RunOnGCE = t.RunOnGCE
				if t.RawOutput.Valid {
					dsTask.RawOutput = t.RawOutput.String
				}
				dsTask.MatchStdoutTxt = t.MatchStdoutTxt

				// Patches and custom webpages/traces.
				customWebpagesGSPath, err := ctutil.SavePatchToStorage(t.CustomWebpages)
				if err != nil {
					sklog.Fatalf("Could not save custom webpages to storage: %s", err)
				}
				chromiumPatchGSPath, err := ctutil.SavePatchToStorage(t.ChromiumPatch)
				if err != nil {
					sklog.Fatalf("Could not save chromium patch to storage: %s", err)
				}
				skiaPatchGSPath, err := ctutil.SavePatchToStorage(t.SkiaPatch)
				if err != nil {
					sklog.Fatalf("Could not save skia patch to storage: %s", err)
				}
				catapultPatchGSPath, err := ctutil.SavePatchToStorage(t.CatapultPatch)
				if err != nil {
					sklog.Fatalf("Could not save catapult patch to storage: %s", err)
				}
				benchmarkPatchGSPath, err := ctutil.SavePatchToStorage(t.BenchmarkPatch)
				if err != nil {
					sklog.Fatalf("Could not save benchmark patch to storage: %s", err)
				}
				v8PatchGSPath, err := ctutil.SavePatchToStorage(t.V8Patch)
				if err != nil {
					sklog.Fatalf("Could not save v8 patch to storage: %s", err)
				}
				dsTask.CustomWebpagesGSPath = customWebpagesGSPath
				dsTask.ChromiumPatchGSPath = chromiumPatchGSPath
				dsTask.SkiaPatchGSPath = skiaPatchGSPath
				dsTask.CatapultPatchGSPath = catapultPatchGSPath
				dsTask.BenchmarkPatchGSPath = benchmarkPatchGSPath
				dsTask.V8PatchGSPath = v8PatchGSPath

				// Key
				key := ds.NewKey(kind)
				key.ID = t.Id
				dsTask.DatastoreKey = key

				// Update the lock.
				highestIDMutex.Lock()
				highestID = util.MaxInt64(key.ID, highestID)
				highestIDMutex.Unlock()

				if _, err := ds.DS.Put(ctx, key, dsTask); err != nil {
					sklog.Fatal("Error putting task in datastore: %s", err)
				}

				fmt.Printf("WE ADDED ID: %d\n", key.ID)
			}

		}()
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	AddHighestID(ctx, kind, highestID)
}

func DeleteKind(ctx context.Context, datastoreKind ds.Kind) (int, error) {
	var i int
	var lastSeenKey *datastore.Key
	q := ds.NewQuery(datastoreKind).Limit(500).KeysOnly().Order("__key__")
	timeout := time.After(time.Second * 60)
	for {
		select {
		case <-timeout:
			{
				return i, nil
			}
		default:
			{
				keys, err := ds.DS.GetAll(ctx, q, nil)
				fmt.Println("bl;ah blah blah")
				fmt.Println(len(keys))
				if err != nil || len(keys) == 0 {
					return i, err
				} else {
					lastSeenKey = keys[len(keys)-1]
					i = i + len(keys)
					if err := ds.DS.DeleteMulti(ctx, keys); err != nil {
						return i, err
					}
					fmt.Printf("persistence > DeleteKind(%s) Entries deleted: %d", datastoreKind, i)
				}
				q = ds.NewQuery(datastoreKind).Limit(500).KeysOnly().Order("__key__").Filter("__key__ >", lastSeenKey)
			}
		}
	}
	return i, nil
}

func scanExisting(client *datastore.Client, targetKinds []ds.Kind) {
	sklog.Infof("Starting to count")
	ctx := context.TODO()
	for _, kind := range targetKinds {
		t := timer.New("counting " + string(kind))
		q := ds.NewQuery(kind).KeysOnly()
		count, err := client.Count(ctx, q)
		if err != nil {
			sklog.Fatalf("Error counting: %s", err)
		}
		sklog.Infof("KIND %s   %d", kind, count)
		t.Stop()
	}
}

// Initialize the MySQL wrapper
func setupMySQL(dbConf *database.DatabaseConfig, promptPassword bool) *database.VersionedDB {
	if promptPassword {
		if err := dbConf.PromptForPassword(); err != nil {
			sklog.Fatal(err)
		}
	}
	vdb, err := dbConf.NewVersionedDB()
	if err != nil {
		sklog.Fatal(err)
	}
	return vdb
}
