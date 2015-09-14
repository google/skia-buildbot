package util

import (
	"fmt"
	"path/filepath"
	"time"
)

const (
	CT_USER                  = "chrome-bot"
	NUM_WORKERS          int = 100
	MASTER_NAME              = "build101-m5"
	WORKER_NAME_TEMPLATE     = "build%d-m5"
	GS_BUCKET_NAME           = "cluster-telemetry"
	GS_HTTP_LINK             = "https://storage.cloud.google.com/"
	LOGS_LINK_PREFIX         = "http://uberchromegw.corp.google.com/i/skia-ct-worker"

	// File names and dir names.
	TIMESTAMP_FILE_NAME          = "TIMESTAMP"
	CHROMIUM_BUILDS_DIR_NAME     = "chromium_builds"
	PAGESETS_DIR_NAME            = "page_sets"
	WEB_ARCHIVES_DIR_NAME        = "webpage_archives"
	SKPS_DIR_NAME                = "skps"
	STORAGE_DIR_NAME             = "storage"
	REPO_DIR_NAME                = "skia-repo"
	TASKS_DIR_NAME               = "tasks"
	LUA_TASKS_DIR_NAME           = "lua_runs"
	BENCHMARK_TASKS_DIR_NAME     = "benchmark_runs"
	CHROMIUM_PERF_TASKS_DIR_NAME = "chromium_perf_runs"
	FIX_ARCHIVE_TASKS_DIR_NAME   = "fix_archive_runs"

	// Limit the number of times CT tries to get a remote file before giving up.
	MAX_URI_GET_TRIES = 4

	// Activity constants.
	ACTIVITY_CREATING_PAGESETS        = "CREATING_PAGESETS"
	ACTIVITY_CAPTURING_ARCHIVES       = "CAPTURING_ARCHIVES"
	ACTIVITY_CAPTURING_SKPS           = "CAPTURING_SKPS"
	ACTIVITY_RUNNING_LUA_SCRIPTS      = "RUNNING_LUA_SCRIPTS"
	ACTIVITY_RUNNING_CHROMIUM_PERF    = "RUNNING_CHROMIUM_PERF"
	ACTIVITY_RUNNING_SKIA_CORRECTNESS = "RUNNING_SKIA_CORRECTNESS"
	ACTIVITY_FIXING_ARCHIVES          = "FIXING_ARCHIVES"

	// Pageset types supported by CT.
	PAGESET_TYPE_ALL        = "All"
	PAGESET_TYPE_10k        = "10k"
	PAGESET_TYPE_MOBILE_10k = "Mobile10k"
	PAGESET_TYPE_DUMMY_1k   = "Dummy1k" // Used for testing.

	// Names of binaries executed by CT.
	BINARY_CHROME          = "chrome"
	BINARY_RECORD_WPR      = "record_wpr"
	BINARY_RUN_BENCHMARK   = "ct_run_benchmark"
	BINARY_GCLIENT         = "gclient"
	BINARY_MAKE            = "make"
	BINARY_LUA_PICTURES    = "lua_pictures"
	BINARY_ADB             = "adb"
	BINARY_GIT             = "git"
	BINARY_RENDER_PICTURES = "render_pictures"
	BINARY_MAIL            = "mail"
	BINARY_LUA             = "lua"

	// Platforms supported by CT.
	PLATFORM_ANDROID = "Android"
	PLATFORM_LINUX   = "Linux"

	// Benchmarks supported by CT.
	BENCHMARK_DRAW_PROPERTIES   = "draw_properties"
	BENCHMARK_SKPICTURE_PRINTER = "skpicture_printer"
	BENCHMARK_RR                = "rasterize_and_record_micro"
	BENCHMARK_REPAINT           = "repaint"
	BENCHMARK_SMOOTHNESS        = "smoothness"

	// Logserver links. These are only accessible from Google corp.
	MASTER_LOGSERVER_LINK  = "http://uberchromegw.corp.google.com/i/skia-ct-master/"
	WORKERS_LOGSERVER_LINK = "http://uberchromegw.corp.google.com/i/skia-ct-master/all_logs"

	// Default browser args when running benchmarks.
	DEFAULT_BROWSER_ARGS = "--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting"

	// Timeouts

	PKILL_TIMEOUT = 5 * time.Minute

	// util.SyncDir
	GIT_PULL_TIMEOUT     = 10 * time.Minute
	GCLIENT_SYNC_TIMEOUT = 15 * time.Minute

	// util.BuildSkiaTools
	MAKE_CLEAN_TIMEOUT = 5 * time.Minute
	MAKE_TOOLS_TIMEOUT = 5 * time.Minute

	// util.ResetCheckout
	GIT_RESET_TIMEOUT = 5 * time.Minute
	GIT_CLEAN_TIMEOUT = 5 * time.Minute
	// util.resetChromiumCheckout calls ResetCheckout three times.
	RESET_CHROMIUM_CHECKOUT_TIMEOUT = 3 * (GIT_RESET_TIMEOUT + GIT_CLEAN_TIMEOUT)

	// util.CreateChromiumBuild
	SYNC_SKIA_IN_CHROME_TIMEOUT   = 2 * time.Hour
	GIT_LS_REMOTE_TIMEOUT         = 5 * time.Minute
	GIT_APPLY_TIMEOUT             = 5 * time.Minute
	GOMA_CTL_RESTART_TIMEOUT      = 10 * time.Minute
	GYP_CHROMIUM_TIMEOUT          = 30 * time.Minute
	NINJA_TIMEOUT                 = 2 * time.Hour
	CREATE_CHROMIUM_BUILD_TIMEOUT = SYNC_SKIA_IN_CHROME_TIMEOUT + GIT_LS_REMOTE_TIMEOUT +
		// Three patches are applied when applyPatches is specified.
		3*GIT_APPLY_TIMEOUT +
		// The build steps are repeated twice when applyPatches is specified.
		2*(GOMA_CTL_RESTART_TIMEOUT+GYP_CHROMIUM_TIMEOUT+NINJA_TIMEOUT+
			RESET_CHROMIUM_CHECKOUT_TIMEOUT)

	// util.InstallChromeAPK
	ADB_INSTALL_TIMEOUT = 15 * time.Minute

	// Allow extra time for updating frontend and any other computation not included in the
	// worker timeouts.
	MASTER_SCRIPT_TIMEOUT_PADDING = 30 * time.Minute

	// Build Chromium Task
	GIT_LOG_TIMEOUT                      = 5 * time.Minute
	MASTER_SCRIPT_BUILD_CHROMIUM_TIMEOUT = CREATE_CHROMIUM_BUILD_TIMEOUT + GIT_LOG_TIMEOUT +
		MASTER_SCRIPT_TIMEOUT_PADDING

	// Capture Archives
	// Setting a 5 day timeout since it may take a while to capture 1M archives.
	CAPTURE_ARCHIVES_TIMEOUT               = 5 * 24 * time.Hour
	MASTER_SCRIPT_CAPTURE_ARCHIVES_TIMEOUT = CAPTURE_ARCHIVES_TIMEOUT +
		MASTER_SCRIPT_TIMEOUT_PADDING

	// Capture SKPs
	REMOVE_INVALID_SKPS_TIMEOUT = 3 * time.Hour
	// Setting a 2 day timeout since it may take a while to capture 1M SKPs.
	CAPTURE_SKPS_TIMEOUT               = 2 * 24 * time.Hour
	MASTER_SCRIPT_CAPTURE_SKPS_TIMEOUT = CAPTURE_SKPS_TIMEOUT + MASTER_SCRIPT_TIMEOUT_PADDING

	// Check Workers Health
	ADB_DEVICES_TIMEOUT          = 30 * time.Minute
	ADB_SHELL_UPTIME_TIMEOUT     = 30 * time.Minute
	CHECK_WORKERS_HEALTH_TIMEOUT = ADB_DEVICES_TIMEOUT + ADB_SHELL_UPTIME_TIMEOUT +
		MASTER_SCRIPT_TIMEOUT_PADDING

	// Create Pagesets
	// Setting a 4 hour timeout since it may take a while to upload page sets to
	// Google Storage when doing 10k page sets per worker.
	CREATE_PAGESETS_TIMEOUT               = 4 * time.Hour
	MASTER_SCRIPT_CREATE_PAGESETS_TIMEOUT = CREATE_PAGESETS_TIMEOUT +
		MASTER_SCRIPT_TIMEOUT_PADDING

	// Run Chromium Perf
	ADB_VERSION_TIMEOUT            = 5 * time.Minute
	ADB_ROOT_TIMEOUT               = 5 * time.Minute
	CSV_PIVOT_TABLE_MERGER_TIMEOUT = 10 * time.Minute
	REBOOT_TIMEOUT                 = 5 * time.Minute
	CSV_MERGER_TIMEOUT             = 1 * time.Hour
	CSV_COMPARER_TIMEOUT           = 2 * time.Hour
	// Setting a 1 day timeout since it may take a while run benchmarks with many
	// repeats.
	RUN_CHROMIUM_PERF_TIMEOUT = 1 * 24 * time.Hour
	// csv_merger runs once for nopatch and once for withpatch
	MASTER_SCRIPT_RUN_CHROMIUM_PERF_TIMEOUT = CREATE_CHROMIUM_BUILD_TIMEOUT + REBOOT_TIMEOUT +
		RUN_CHROMIUM_PERF_TIMEOUT + 2*CSV_MERGER_TIMEOUT + CSV_COMPARER_TIMEOUT +
		MASTER_SCRIPT_TIMEOUT_PADDING

	// Run Lua
	LUA_PICTURES_TIMEOUT          = 2 * time.Hour
	RUN_LUA_TIMEOUT               = 2 * time.Hour
	LUA_AGGREGATOR_TIMEOUT        = 1 * time.Hour
	MASTER_SCRIPT_RUN_LUA_TIMEOUT = RUN_LUA_TIMEOUT + LUA_AGGREGATOR_TIMEOUT +
		MASTER_SCRIPT_TIMEOUT_PADDING

	// Fix Archives
	// Setting a 1 day timeout since it may take a while to validate archives.
	FIX_ARCHIVES_TIMEOUT = 1 * 24 * time.Hour

	// Poller
	MAKE_ALL_TIMEOUT = 15 * time.Minute

	WEBHOOK_SALT_MSG = `For prod, set this file to the value of GCE metadata key webhook_request_salt or call webhook.MustInitRequestSaltFromMetadata() if running in GCE. For testing, run 'echo -n "notverysecret" | base64 -w 0 > /b/storage/webhook_salt.data' or call frontend.InitForTesting().`
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
	Master = fmt.Sprintf(WORKER_NAME_TEMPLATE, 101)
	Slaves = GetCTWorkers()

	// Email address of cluster telemetry admins. They will be notified everytime
	// a task has started and completed.
	CtAdmins = []string{"rmistry@google.com"}

	// Names of local directories and files.
	StorageDir           = filepath.Join("/", "b", STORAGE_DIR_NAME)
	RepoDir              = filepath.Join("/", "b", REPO_DIR_NAME)
	GomaDir              = filepath.Join("/", "b", "build", "goma")
	ChromiumBuildsDir    = filepath.Join(StorageDir, CHROMIUM_BUILDS_DIR_NAME)
	ChromiumSrcDir       = filepath.Join(StorageDir, "chromium", "src")
	TelemetryBinariesDir = filepath.Join(ChromiumSrcDir, "tools", "perf")
	TelemetrySrcDir      = filepath.Join(ChromiumSrcDir, "tools", "telemetry")
	TaskFileDir          = filepath.Join(StorageDir, "current_task")
	GSTokenPath          = filepath.Join(StorageDir, "google_storage_token.data")
	EmailTokenPath       = filepath.Join(StorageDir, "email.data")
	WebappPasswordPath   = filepath.Join(StorageDir, "webapp.data")
	// Salt used to authenticate webhook requests, base64-encoded. See WEBHOOK_SALT_MSG.
	WebhookRequestSaltPath = filepath.Join(StorageDir, "webhook_salt.data")
	PagesetsDir            = filepath.Join(StorageDir, PAGESETS_DIR_NAME)
	WebArchivesDir         = filepath.Join(StorageDir, WEB_ARCHIVES_DIR_NAME)
	SkpsDir                = filepath.Join(StorageDir, SKPS_DIR_NAME)
	GLogDir                = filepath.Join(StorageDir, "glog")
	ApkName                = "ChromePublic.apk"
	SkiaTreeDir            = filepath.Join(RepoDir, "trunk")
	CtTreeDir              = filepath.Join(RepoDir, "go", "src", "go.skia.org", "infra", "ct")

	// Names of remote directories and files.
	LuaRunsDir          = filepath.Join(TASKS_DIR_NAME, LUA_TASKS_DIR_NAME)
	BenchmarkRunsDir    = filepath.Join(TASKS_DIR_NAME, BENCHMARK_TASKS_DIR_NAME)
	ChromiumPerfRunsDir = filepath.Join(TASKS_DIR_NAME, CHROMIUM_PERF_TASKS_DIR_NAME)
	FixArchivesRunsDir  = filepath.Join(TASKS_DIR_NAME, FIX_ARCHIVE_TASKS_DIR_NAME)

	// Information about the different CT benchmarks.
	BenchmarksToPagesetName = map[string]string{
		BENCHMARK_DRAW_PROPERTIES:   "DrawPropertiesCTPages",
		BENCHMARK_SKPICTURE_PRINTER: "SkpicturePrinter",
		BENCHMARK_RR:                "RasterizeAndRecordMicroCTPages",
		BENCHMARK_REPAINT:           "RepaintCTPages",
		BENCHMARK_SMOOTHNESS:        "SmoothnessCTPages",
	}

	// Information about the different CT pageset types.
	PagesetTypeToInfo = map[string]*PagesetTypeInfo{
		PAGESET_TYPE_ALL: &PagesetTypeInfo{
			NumPages:                   1000000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  60,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 1M (with desktop user-agent)",
		},
		PAGESET_TYPE_10k: &PagesetTypeInfo{
			NumPages:                   10000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  60,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 10K (with desktop user-agent)",
		},
		PAGESET_TYPE_MOBILE_10k: &PagesetTypeInfo{
			NumPages:                   10000,
			CSVSource:                  "csv/android-top-1m.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  60,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 10K (with mobile user-agent)",
		},
		PAGESET_TYPE_DUMMY_1k: &PagesetTypeInfo{
			NumPages:                   1000,
			CSVSource:                  "csv/android-top-1m.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  60,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 1K (used for testing)",
		},
	}

	// Frontend constants below.
	SupportedBenchmarks = []string{
		BENCHMARK_RR,
		BENCHMARK_REPAINT,
		BENCHMARK_DRAW_PROPERTIES,
	}

	SupportedPlatformsToDesc = map[string]string{
		PLATFORM_LINUX:   "Linux (100 Ubuntu12.04 machines)",
		PLATFORM_ANDROID: "Android (100 N5 devices)",
	}

	SupportedPageSetsToDesc = map[string]string{
		PLATFORM_LINUX:   "Linux (100 Ubuntu12.04 machines)",
		PLATFORM_ANDROID: "Android (100 N5 devices)",
	}
)
