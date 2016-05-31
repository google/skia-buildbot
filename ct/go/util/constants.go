package util

import (
	"path/filepath"
	"time"
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
	PAGESET_TYPE_ALL        = "All"
	PAGESET_TYPE_10k        = "10k"
	PAGESET_TYPE_MOBILE_10k = "Mobile10k"
	PAGESET_TYPE_PDF_1m     = "PDF1m"
	PAGESET_TYPE_PDF_1k     = "PDF1k"
	PAGESET_TYPE_DUMMY_1k   = "Dummy1k" // Used for testing.

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

	// Platforms supported by CT.
	PLATFORM_ANDROID = "Android"
	PLATFORM_LINUX   = "Linux"

	// Benchmarks supported by CT.
	BENCHMARK_SKPICTURE_PRINTER = "skpicture_printer"
	BENCHMARK_RR                = "rasterize_and_record_micro"
	BENCHMARK_REPAINT           = "repaint"

	// Logserver link. This is only accessible from Google corp.
	MASTER_LOGSERVER_LINK = "http://uberchromegw.corp.google.com/i/skia-ct-master/"

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

	// util.CreateChromiumBuildOnSwarming
	SYNC_SKIA_IN_CHROME_TIMEOUT   = 2 * time.Hour
	GIT_LS_REMOTE_TIMEOUT         = 5 * time.Minute
	GIT_APPLY_TIMEOUT             = 5 * time.Minute
	GOMA_CTL_RESTART_TIMEOUT      = 10 * time.Minute
	GN_CHROMIUM_TIMEOUT           = 30 * time.Minute
	GYP_PDFIUM_TIMEOUT            = 5 * time.Minute
	NINJA_TIMEOUT                 = 2 * time.Hour
	CREATE_CHROMIUM_BUILD_TIMEOUT = SYNC_SKIA_IN_CHROME_TIMEOUT + GIT_LS_REMOTE_TIMEOUT +
		// Three patches are applied when applyPatches is specified.
		3*GIT_APPLY_TIMEOUT +
		// The build steps are repeated twice when applyPatches is specified.
		2*(GOMA_CTL_RESTART_TIMEOUT+GN_CHROMIUM_TIMEOUT+NINJA_TIMEOUT+
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
	CAPTURE_ARCHIVES_DEFAULT_CT_BENCHMARK  = "rasterize_and_record_micro_ct"
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
	SWARMING_TASKS_LINK_TEMPLATE = "https://chromium-swarm.appspot.com/user/tasks?limit=500&sort=created_ts&state=all&task_tag=runid:%s"
	SWARMING_NAME_PARAM          = "%0D%0Aname:"
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
	StorageDir           = filepath.Join("/", "b", STORAGE_DIR_NAME)
	RepoDir              = filepath.Join("/", "b", REPO_DIR_NAME)
	DepotToolsDir        = filepath.Join("/", "b", "depot_tools")
	GomaDir              = filepath.Join("/", "b", "build", "goma")
	ChromiumBuildsDir    = filepath.Join(StorageDir, CHROMIUM_BUILDS_DIR_NAME)
	ChromiumSrcDir       = filepath.Join(StorageDir, "chromium", "src")
	TelemetryBinariesDir = filepath.Join(ChromiumSrcDir, "tools", "perf")
	TelemetrySrcDir      = filepath.Join(ChromiumSrcDir, "tools", "telemetry")
	CatapultSrcDir       = filepath.Join(ChromiumSrcDir, "third_party", "catapult")
	TaskFileDir          = filepath.Join(StorageDir, "current_task")
	ClientSecretPath     = filepath.Join(StorageDir, "client_secret.json")
	GSTokenPath          = filepath.Join(StorageDir, "google_storage_token.data")
	EmailTokenPath       = filepath.Join(StorageDir, "email.data")
	WebappPasswordPath   = filepath.Join(StorageDir, "webapp.data")
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
			Description:                "Top 1K (used for testing, hidden from Runs History by default)",
		},
		PAGESET_TYPE_PDF_1m: &PagesetTypeInfo{
			NumPages:                   1000000,
			CSVSource:                  "csv/pdf-top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  60,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "PDF 1M",
		},
		PAGESET_TYPE_PDF_1k: &PagesetTypeInfo{
			NumPages:                   1000,
			CSVSource:                  "csv/pdf-top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  60,
			CaptureArchivesTimeoutSecs: 300,
			CaptureSKPsTimeoutSecs:     300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "PDF 1K",
		},
	}

	// Frontend constants below.
	SupportedBenchmarks = []string{
		BENCHMARK_RR,
		BENCHMARK_REPAINT,
	}

	SupportedPlatformsToDesc = map[string]string{
		PLATFORM_LINUX:   "Linux (100 Ubuntu12.04 machines)",
		PLATFORM_ANDROID: "Android (100 N5 devices)",
	}

	// Swarming machine dimensions.
	GCE_WORKER_DIMENSIONS          = map[string]string{"pool": SWARMING_POOL, "cores": "2"}
	GCE_ANDROID_BUILDER_DIMENSIONS = map[string]string{"pool": "AndroidBuilder", "cores": "32"}
	GCE_LINUX_BUILDER_DIMENSIONS   = map[string]string{"pool": "LinuxBuilder", "cores": "32"}
	GOLO_WORKER_DIMENSIONS         = map[string]string{"pool": SWARMING_POOL, "os": "Android"}
)
