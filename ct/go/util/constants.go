package util

import (
	"path/filepath"
	"time"

	"go.skia.org/infra/go/swarming"
)

const (
	MASTER_NAME                  = "build101-m5"
	NUM_BARE_METAL_MACHINES  int = 100
	BARE_METAL_NAME_TEMPLATE     = "build%d-m5"
	GS_HTTP_LINK                 = "https://storage.cloud.google.com/"
	CT_EMAIL_DISPLAY_NAME        = "Cluster Telemetry"

	// File names and dir names.
	CHROMIUM_BUILDS_DIR_NAME         = "chromium_builds"
	PAGESETS_DIR_NAME                = "page_sets"
	WEB_ARCHIVES_DIR_NAME            = "webpage_archives"
	SKPS_DIR_NAME                    = "skps"
	PDFS_DIR_NAME                    = "pdfs"
	STORAGE_DIR_NAME                 = "storage"
	REPO_DIR_NAME                    = "skia-repo"
	TASKS_DIR_NAME                   = "tasks"
	BINARIES_DIR_NAME                = "binaries"
	LUA_TASKS_DIR_NAME               = "lua_runs"
	BENCHMARK_TASKS_DIR_NAME         = "benchmark_runs"
	CHROMIUM_PERF_TASKS_DIR_NAME     = "chromium_perf_runs"
	CHROMIUM_ANALYSIS_TASKS_DIR_NAME = "chromium_analysis_runs"
	FIX_ARCHIVE_TASKS_DIR_NAME       = "fix_archive_runs"

	// Limit the number of times CT tries to get a remote file before giving up.
	MAX_URI_GET_TRIES = 4

	// Pageset types supported by CT.
	PAGESET_TYPE_ALL           = "All"
	PAGESET_TYPE_100k          = "100k"
	PAGESET_TYPE_MOBILE_100k   = "Mobile100k"
	PAGESET_TYPE_10k           = "10k"
	PAGESET_TYPE_MOBILE_10k    = "Mobile10k"
	PAGESET_TYPE_PDF_400m      = "PDF400m"
	PAGESET_TYPE_PDF_1m        = "PDF1m"
	PAGESET_TYPE_PDF_1k        = "PDF1k"
	PAGESET_TYPE_TRANSLATE_48k = "Translate48k"
	PAGESET_TYPE_DUMMY_1k      = "Dummy1k" // Used for testing.

	// Names of binaries executed by CT.
	BINARY_CHROME        = "chrome"
	BINARY_RECORD_WPR    = "record_wpr"
	BINARY_RUN_BENCHMARK = "run_benchmark"
	BINARY_GCLIENT       = "gclient"
	BINARY_MAKE          = "make"
	BINARY_NINJA         = "ninja"
	BINARY_LUA_PICTURES  = "lua_pictures"
	BINARY_ADB           = "adb"
	BINARY_GIT           = "git"
	BINARY_MAIL          = "mail"
	BINARY_LUA           = "lua"
	BINARY_PDFIUM_TEST   = "pdfium_test"
	// TODO(rmistry): Remove once all CT bots have been upgraded to use 2.7.11
	//                by default.
	BINARY_PYTHON_2_7_11 = "/usr/local/lib/python2.7.11/bin/python"

	// Platforms supported by CT.
	PLATFORM_ANDROID = "Android"
	PLATFORM_LINUX   = "Linux"

	// Benchmarks supported by CT.
	BENCHMARK_SKPICTURE_PRINTER = "skpicture_printer"
	BENCHMARK_RR                = "rasterize_and_record_micro"
	BENCHMARK_REPAINT           = "repaint"
	BENCHMARK_LOADING           = "loading.cluster_telemetry"

	// Logserver link. This is only accessible from Google corp.
	MASTER_LOGSERVER_LINK = "http://uberchromegw.corp.google.com/i/skia-ct-master/"

	// Default browser args when running benchmarks.
	DEFAULT_BROWSER_ARGS = "--disable-setuid-sandbox"

	// Timeouts

	PKILL_TIMEOUT       = 5 * time.Minute
	HTTP_CLIENT_TIMEOUT = 30 * time.Minute

	// util.SyncDir
	GIT_PULL_TIMEOUT     = 10 * time.Minute
	GCLIENT_SYNC_TIMEOUT = 15 * time.Minute

	// util.BuildSkiaTools
	MAKE_CLEAN_TIMEOUT = 5 * time.Minute
	MAKE_TOOLS_TIMEOUT = 15 * time.Minute

	// util.ResetCheckout
	GIT_RESET_TIMEOUT = 5 * time.Minute
	GIT_CLEAN_TIMEOUT = 5 * time.Minute

	// util.CreateChromiumBuildOnSwarming
	SYNC_SKIA_IN_CHROME_TIMEOUT = 2 * time.Hour
	GIT_LS_REMOTE_TIMEOUT       = 5 * time.Minute
	GIT_APPLY_TIMEOUT           = 5 * time.Minute
	GN_CHROMIUM_TIMEOUT         = 30 * time.Minute
	GYP_PDFIUM_TIMEOUT          = 5 * time.Minute
	NINJA_TIMEOUT               = 2 * time.Hour

	// util.InstallChromeAPK
	ADB_INSTALL_TIMEOUT = 15 * time.Minute

	// Capture Archives
	CAPTURE_ARCHIVES_DEFAULT_CT_BENCHMARK = "rasterize_and_record_micro_ct"

	// Capture SKPs
	REMOVE_INVALID_SKPS_TIMEOUT = 3 * time.Hour

	// Run Chromium Perf
	ADB_VERSION_TIMEOUT            = 5 * time.Minute
	ADB_ROOT_TIMEOUT               = 5 * time.Minute
	CSV_PIVOT_TABLE_MERGER_TIMEOUT = 10 * time.Minute
	CSV_MERGER_TIMEOUT             = 1 * time.Hour
	CSV_COMPARER_TIMEOUT           = 2 * time.Hour

	// Run Lua
	LUA_PICTURES_TIMEOUT   = 2 * time.Hour
	LUA_AGGREGATOR_TIMEOUT = 1 * time.Hour

	// Poller
	MAKE_ALL_TIMEOUT = 15 * time.Minute

	WEBHOOK_SALT_MSG = `For prod, set this file to the value of GCE metadata key webhook_request_salt or call webhook.MustInitRequestSaltFromMetadata() if running in GCE. For testing, run 'echo -n "notverysecret" | base64 -w 0 > /b/storage/webhook_salt.data' or call frontend.InitForTesting().`

	// Swarming constants.
	SWARMING_DIR_NAME     = "swarming"
	SWARMING_POOL         = "CT"
	BUILD_OUTPUT_FILENAME = "build_remote_dirs.txt"
	// Timeouts.
	BATCHARCHIVE_TIMEOUT = 10 * time.Minute
	XVFB_TIMEOUT         = 5 * time.Minute
	// Isolate files.
	CREATE_PAGESETS_ISOLATE        = "create_pagesets.isolate"
	CAPTURE_ARCHIVES_ISOLATE       = "capture_archives.isolate"
	CAPTURE_SKPS_ISOLATE           = "capture_skps.isolate"
	CAPTURE_SKPS_FROM_PDFS_ISOLATE = "capture_skps_from_pdfs.isolate"
	RUN_LUA_ISOLATE                = "run_lua.isolate"
	CHROMIUM_ANALYSIS_ISOLATE      = "chromium_analysis.isolate"
	CHROMIUM_PERF_ISOLATE          = "chromium_perf.isolate"
	BUILD_REPO_ISOLATE             = "build_repo.isolate"
	// Swarming links and params.
	SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE   = "https://chromium-swarm.appspot.com/tasklist?l=500&c=name&c=created_ts&c=bot&c=duration&c=state&f=runid:%s"
	SWARMING_RUN_ID_TASK_LINK_PREFIX_TEMPLATE = SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE + "&f=name:%s"
	// Priorities
	USER_TASKS_PRIORITY  = swarming.RECOMMENDED_PRIORITY
	ADMIN_TASKS_PRIORITY = swarming.RECOMMENDED_PRIORITY + 10 // Use lower priority for admin tasks because they can be long runned and we do not want to starve user jobs.
)

type PagesetTypeInfo struct {
	NumPages                   int
	CSVSource                  string
	UserAgent                  string
	CaptureArchivesTimeoutSecs int
	CreatePagesetsTimeoutSecs  int
	CaptureSKPsTimeoutSecs     int
	RunChromiumPerfTimeoutSecs int
	Description                string
}

var (
	CtUser          = "chrome-bot"
	GSBucketName    = "cluster-telemetry"
	BareMetalSlaves = GetCTBareMetalWorkers()

	// Email address of cluster telemetry admins. They will be notified everytime
	// a task has started and completed.
	CtAdmins = []string{"rmistry@google.com", "benjaminwagner@google.com"}

	// Names of local directories and files.
	StorageDir             = filepath.Join("/", "b", STORAGE_DIR_NAME)
	RepoDir                = filepath.Join("/", "b", REPO_DIR_NAME)
	DepotToolsDir          = filepath.Join("/", "b", "depot_tools")
	ChromiumBuildsDir      = filepath.Join(StorageDir, CHROMIUM_BUILDS_DIR_NAME)
	ChromiumSrcDir         = filepath.Join(StorageDir, "chromium", "src")
	TelemetryBinariesDir   = filepath.Join(ChromiumSrcDir, "tools", "perf")
	TelemetrySrcDir        = filepath.Join(ChromiumSrcDir, "tools", "telemetry")
	RelativeCatapultSrcDir = filepath.Join("third_party", "catapult")
	CatapultSrcDir         = filepath.Join(ChromiumSrcDir, RelativeCatapultSrcDir)
	TaskFileDir            = filepath.Join(StorageDir, "current_task")
	ClientSecretPath       = filepath.Join(StorageDir, "client_secret.json")
	GSTokenPath            = filepath.Join(StorageDir, "google_storage_token.data")
	EmailTokenPath         = filepath.Join(StorageDir, "email.data")
	WebappPasswordPath     = filepath.Join(StorageDir, "webapp.data")
	// Salt used to authenticate webhook requests, base64-encoded. See WEBHOOK_SALT_MSG.
	WebhookRequestSaltPath = filepath.Join(StorageDir, "webhook_salt.data")
	PagesetsDir            = filepath.Join(StorageDir, PAGESETS_DIR_NAME)
	WebArchivesDir         = filepath.Join(StorageDir, WEB_ARCHIVES_DIR_NAME)
	PdfsDir                = filepath.Join(StorageDir, PDFS_DIR_NAME)
	SkpsDir                = filepath.Join(StorageDir, SKPS_DIR_NAME)
	GLogDir                = filepath.Join(StorageDir, "glog")
	ApkName                = "ChromePublic.apk"
	SkiaTreeDir            = filepath.Join(RepoDir, "trunk")
	PDFiumTreeDir          = filepath.Join(RepoDir, "pdfium")
	CtTreeDir              = filepath.Join(RepoDir, "go", "src", "go.skia.org", "infra", "ct")

	// Names of remote directories and files.
	BinariesDir             = filepath.Join(BINARIES_DIR_NAME)
	LuaRunsDir              = filepath.Join(TASKS_DIR_NAME, LUA_TASKS_DIR_NAME)
	BenchmarkRunsDir        = filepath.Join(TASKS_DIR_NAME, BENCHMARK_TASKS_DIR_NAME)
	ChromiumPerfRunsDir     = filepath.Join(TASKS_DIR_NAME, CHROMIUM_PERF_TASKS_DIR_NAME)
	ChromiumAnalysisRunsDir = filepath.Join(TASKS_DIR_NAME, CHROMIUM_ANALYSIS_TASKS_DIR_NAME)
	FixArchivesRunsDir      = filepath.Join(TASKS_DIR_NAME, FIX_ARCHIVE_TASKS_DIR_NAME)

	// Map CT benchmarks to the names recognized by Telemetry.
	BenchmarksToTelemetryName = map[string]string{
		BENCHMARK_SKPICTURE_PRINTER: "skpicture_printer_ct",
		BENCHMARK_RR:                "rasterize_and_record_micro_ct",
		BENCHMARK_REPAINT:           "repaint_ct",
		BENCHMARK_LOADING:           "loading.cluster_telemetry",
	}

	// Information about the different CT pageset types.
	PagesetTypeToInfo = map[string]*PagesetTypeInfo{
		PAGESET_TYPE_ALL: &PagesetTypeInfo{
			NumPages:                   1000000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 1M (with desktop user-agent)",
		},
		PAGESET_TYPE_100k: &PagesetTypeInfo{
			NumPages:                   100000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 100K (with desktop user-agent)",
		},
		PAGESET_TYPE_MOBILE_100k: &PagesetTypeInfo{
			NumPages:                   100000,
			CSVSource:                  "csv/android-top-1m.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 100K (with mobile user-agent)",
		},
		PAGESET_TYPE_10k: &PagesetTypeInfo{
			NumPages:                   10000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 10K (with desktop user-agent)",
		},
		PAGESET_TYPE_MOBILE_10k: &PagesetTypeInfo{
			NumPages:                   10000,
			CSVSource:                  "csv/android-top-1m.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 10K (with mobile user-agent)",
		},
		PAGESET_TYPE_DUMMY_1k: &PagesetTypeInfo{
			NumPages:                   1000,
			CSVSource:                  "csv/android-top-1m.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 1K (used for testing, hidden from Runs History by default)",
		},
		PAGESET_TYPE_PDF_400m: &PagesetTypeInfo{
			NumPages:                   400000000,
			CSVSource:                  "csv/pdf-400m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "PDF 400M",
		},
		PAGESET_TYPE_PDF_1m: &PagesetTypeInfo{
			NumPages:                   1000000,
			CSVSource:                  "csv/pdf-top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "PDF 1M",
		},
		PAGESET_TYPE_PDF_1k: &PagesetTypeInfo{
			NumPages:                   1000,
			CSVSource:                  "csv/pdf-top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "PDF 1K",
		},
		PAGESET_TYPE_TRANSLATE_48k: &PagesetTypeInfo{
			NumPages:                   48000,
			CSVSource:                  "csv/translate-48k.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Translate 48K",
		},
	}

	// Frontend constants below.
	SupportedBenchmarks = []string{
		BENCHMARK_RR,
		BENCHMARK_REPAINT,
		BENCHMARK_LOADING,
	}

	SupportedPlatformsToDesc = map[string]string{
		PLATFORM_LINUX:   "Linux (100 Ubuntu14.04 machines)",
		PLATFORM_ANDROID: "Android (100 N5 devices)",
	}

	// Swarming machine dimensions.
	GCE_WORKER_DIMENSIONS          = map[string]string{"pool": SWARMING_POOL, "cores": "2"}
	GCE_ANDROID_BUILDER_DIMENSIONS = map[string]string{"pool": "AndroidBuilder", "cores": "32"}
	GCE_LINUX_BUILDER_DIMENSIONS   = map[string]string{"pool": "LinuxBuilder", "cores": "32"}
	GOLO_WORKER_DIMENSIONS         = map[string]string{"pool": SWARMING_POOL, "os": "Android"}
)
