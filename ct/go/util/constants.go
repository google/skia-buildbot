package util

import (
	"path"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/swarming"
)

const (
	// Use the CTFE proxy to Google Storage. See skbug.com/6762
	GCS_HTTP_LINK = "https://ct.skia.org/results/"

	// File names and dir names.
	CHROMIUM_BUILDS_DIR_NAME         = "chromium_builds"
	PAGESETS_DIR_NAME                = "page_sets"
	WEB_ARCHIVES_DIR_NAME            = "webpage_archives"
	STORAGE_DIR_NAME                 = "storage"
	REPO_DIR_NAME                    = "skia-repo"
	TASKS_DIR_NAME                   = "tasks"
	BINARIES_DIR_NAME                = "binaries"
	BENCHMARK_TASKS_DIR_NAME         = "benchmark_runs"
	CHROMIUM_PERF_TASKS_DIR_NAME     = "chromium_perf_runs"
	CHROMIUM_ANALYSIS_TASKS_DIR_NAME = "chromium_analysis_runs"
	FIX_ARCHIVE_TASKS_DIR_NAME       = "fix_archive_runs"
	TRACE_DOWNLOADS_DIR_NAME         = "trace_downloads"
	CHROMIUM_BUILD_ZIP_NAME          = "chromium_build.zip"
	CUSTOM_APK_DIR_NAME              = "custom-apk"

	// Limit the number of times CT tries to get a remote file before giving up.
	MAX_URI_GET_TRIES = 4

	// Pageset types supported by CT.
	PAGESET_TYPE_ALL                 = "All"
	PAGESET_TYPE_100k                = "100k"
	PAGESET_TYPE_MOBILE_100k         = "Mobile100k"
	PAGESET_TYPE_LAYOUTSHIFT_100k    = "LayoutShift100k"
	PAGESET_TYPE_10k                 = "10k"
	PAGESET_TYPE_MOBILE_10k          = "Mobile10k"
	PAGESET_TYPE_MOBILE_VOLT_10k     = "VoltMobile10k"
	PAGESET_TYPE_LAYOUTSHIFT_10k     = "LayoutShift10k"
	PAGESET_TYPE_AMP_LIVE_REPRO      = "AMPLiveRepro"
	PAGESET_TYPE_AMP_PUPPETEER_SITES = "AMPPuppeteerSites"
	PAGESET_TYPE_DUMMY_1k            = "Dummy1k"       // Used for testing.
	PAGESET_TYPE_MOBILE_DUMMY_1k     = "DummyMobile1k" // Used for testing.

	// Names of binaries executed by CT.
	BINARY_CHROME          = "chrome"
	BINARY_CHROME_WINDOWS  = "chrome.exe"
	BINARY_RECORD_WPR      = "record_wpr"
	BINARY_RUN_BENCHMARK   = "run_benchmark"
	BINARY_ANALYZE_METRICS = "analyze_metrics_ct.py"
	BINARY_GCLIENT         = "gclient"
	BINARY_NINJA           = "ninja"
	BINARY_ADB             = "adb"
	BINARY_MAIL            = "mail"
	BINARY_PYTHON          = "python3"
	// chromium/src's analyze_metrics_ct.py and record_wpr still seem to be on
	// python2.
	BINARY_VPYTHON  = "vpython"
	BINARY_VPYTHON3 = "vpython3"

	// Platforms supported by CT.
	PLATFORM_ANDROID = "Android"
	PLATFORM_LINUX   = "Linux"
	PLATFORM_WINDOWS = "Windows"

	// Benchmarks supported by CT.
	BENCHMARK_RR                       = "rasterize_and_record_micro_ct"
	BENCHMARK_LOADING                  = "loading.cluster_telemetry"
	BENCHMARK_SCREENSHOT               = "screenshot_ct"
	BENCHMARK_RENDERING                = "rendering.cluster_telemetry"
	BENCHMARK_USECOUNTER               = "usecounter_ct"
	BENCHMARK_LEAK_DETECTION           = "leak_detection.cluster_telemetry"
	BENCHMARK_MEMORY                   = "memory.cluster_telemetry"
	BENCHMARK_V8_LOADING               = "v8.loading.cluster_telemetry"
	BENCHMARK_V8_LOADING_RUNTIME_STATS = "v8.loading_runtime_stats.cluster_telemetry"
	BENCHMARK_GENERIC_TRACE            = "generic_trace_ct"
	BENCHMARK_AD_TAGGING               = "ad_tagging.cluster_telemetry"
	BENCHMARK_LAYOUT_SHIFT             = "layout_shift.cluster_telemetry"

	// Default browser args when running benchmarks.
	DEFAULT_BROWSER_ARGS = ""
	// Default value column name to use when merging CSVs.
	DEFAULT_VALUE_COLUMN_NAME = "avg"

	// Use live sites flag.
	USE_LIVE_SITES_FLAGS = "--use-live-sites"
	// Pageset repeat flag.
	PAGESET_REPEAT_FLAG = "--pageset-repeat"
	// User agent flag.
	USER_AGENT_FLAG = "--user-agent"
	// Run Benchmark timeout flag.
	RUN_BENCHMARK_TIMEOUT_FLAG = "--run-benchmark-timeout"
	// Max pages per bot flag.
	MAX_PAGES_PER_BOT = "--max-pages-per-bot"
	// Num of retries used by analysis task.
	NUM_ANALYSIS_RETRIES = "--num-analysis-retries"

	// Defaults for custom webpages.
	DEFAULT_CUSTOM_PAGE_ARCHIVEPATH = "dummy_path"

	// Timeouts

	PKILL_TIMEOUT              = 5 * time.Minute
	HTTP_CLIENT_TIMEOUT        = 30 * time.Minute
	FETCH_GN_TIMEOUT           = 2 * time.Minute
	GN_GEN_TIMEOUT             = 2 * time.Minute
	UPDATE_DEPOT_TOOLS_TIMEOUT = 5 * time.Minute

	// util.SyncDir
	GIT_PULL_TIMEOUT     = 30 * time.Minute
	GCLIENT_SYNC_TIMEOUT = 30 * time.Minute

	// util.ResetCheckout
	GIT_CHECKOUT_TIMEOUT = 10 * time.Minute
	GIT_REBASE_TIMEOUT   = 10 * time.Minute
	GIT_RESET_TIMEOUT    = 10 * time.Minute
	GIT_CLEAN_TIMEOUT    = 10 * time.Minute

	// util.CreateChromiumBuildOnSwarming
	SYNC_SKIA_IN_CHROME_TIMEOUT = 2 * time.Hour
	GIT_LS_REMOTE_TIMEOUT       = 5 * time.Minute
	GIT_APPLY_TIMEOUT           = 5 * time.Minute
	GN_CHROMIUM_TIMEOUT         = 30 * time.Minute
	NINJA_TIMEOUT               = 2 * time.Hour

	// util.UnInstallChromeAPK
	ADB_UNINSTALL_TIMEOUT = time.Minute

	// util.InstallChromeAPK
	ADB_INSTALL_TIMEOUT = 5 * time.Minute

	// Capture Archives
	CAPTURE_ARCHIVES_DEFAULT_CT_BENCHMARK = "rasterize_and_record_micro_ct"
	CAPTURE_ARCHIVES_AMP_STORY            = "layout_shift_cluster_telemetry"

	// Run Chromium Perf
	ADB_VERSION_TIMEOUT            = 5 * time.Minute
	ADB_ROOT_TIMEOUT               = 5 * time.Minute
	CSV_PIVOT_TABLE_MERGER_TIMEOUT = 10 * time.Minute
	CSV_MERGER_TIMEOUT             = 1 * time.Hour
	CSV_COMPARER_TIMEOUT           = 2 * time.Hour

	// Poller
	MAKE_ALL_TIMEOUT = 15 * time.Minute

	// Swarming constants.
	SWARMING_DIR_NAME               = "swarming"
	SWARMING_POOL                   = "CT"
	BUILD_OUTPUT_FILENAME           = "build_remote_dirs.txt"
	ISOLATE_TELEMETRY_FILENAME      = "isolate_telemetry_hash.txt"
	MAX_SWARMING_HARD_TIMEOUT_HOURS = 24
	// Timeouts.
	BATCHARCHIVE_TIMEOUT = 10 * time.Minute
	XVFB_TIMEOUT         = 5 * time.Minute

	// Swarming links and params.
	// TODO(rmistry): The below link contains "st=1262304000000" which is from 2010. This is done so
	// that swarming will not use today's timestamp as default. See if there is a better way to handle
	// this.
	SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE   = "https://chrome-swarming.appspot.com/tasklist?l=500&c=name&c=created_ts&c=bot&c=duration&c=state&f=runid:%s&st=1262304000000"
	SWARMING_RUN_ID_TASK_LINK_PREFIX_TEMPLATE = SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE + "&f=name:%s"

	// Priorities
	TASKS_PRIORITY_HIGH   = swarming.RECOMMENDED_PRIORITY
	TASKS_PRIORITY_MEDIUM = swarming.RECOMMENDED_PRIORITY + 10
	TASKS_PRIORITY_LOW    = swarming.RECOMMENDED_PRIORITY + 20

	// ct-perf.skia.org constants.
	CT_PERF_BUCKET = "cluster-telemetry-perf"
	CT_PERF_REPO   = "https://skia.googlesource.com/perf-ct"

	// The CT service account is unfortunately named, it runs on not just Golo
	// bots but also on GCE instances. Would be nice to rename this one day following
	// the steps in https://bugs.chromium.org/p/skia/issues/detail?id=10187#c1
	CT_SERVICE_ACCOUNT = "ct-golo@ct-swarming-bots.iam.gserviceaccount.com"

	CHROME_ANDROID_PACKAGE_NAME  = "com.google.android.apps.chrome"
	ADB_CIPD_PACKAGE             = "cipd_bin_packages:infra/adb/linux-amd64:adb_version:1.0.36"
	LUCI_AUTH_CIPD_PACKAGE_LINUX = "cipd_bin_packages:infra/tools/luci-auth/linux-amd64:git_revision:41a7e9bcbf18718dcda83dd5c6188cfc44271e70"
	LUCI_AUTH_CIPD_PACKAGE_WIN   = "cipd_bin_packages:infra/tools/luci-auth/windows-amd64:git_revision:41a7e9bcbf18718dcda83dd5c6188cfc44271e70"
)

type PagesetTypeInfo struct {
	NumPages                   int
	CSVSource                  string
	UserAgent                  string
	CaptureArchivesTimeoutSecs int
	CreatePagesetsTimeoutSecs  int
	RunChromiumPerfTimeoutSecs int
	Description                string
}

var (
	CtUser = "chrome-bot"
	// Whenever the bucket name changes, getGSLink in ctfe.js will have to be
	// updated as well.
	GCSBucketName = "cluster-telemetry"

	// Email address of cluster telemetry admins. They will be notified every time
	// a task has started and completed.
	CtAdmins = []string{"rmistry@google.com", "borenet@google.com"}

	// Names of local directories and files.
	StorageDir             = filepath.Join("/", "b", STORAGE_DIR_NAME)
	RepoDir                = filepath.Join("/", "b", REPO_DIR_NAME)
	DepotToolsDir          = filepath.Join("/", "home", "chrome-bot", "depot_tools")
	ChromiumBuildsDir      = filepath.Join(StorageDir, CHROMIUM_BUILDS_DIR_NAME)
	ChromiumSrcDir         = filepath.Join(StorageDir, "chromium", "src")
	TelemetryBinariesDir   = filepath.Join(ChromiumSrcDir, "tools", "perf")
	TelemetrySrcDir        = filepath.Join(ChromiumSrcDir, "tools", "telemetry")
	RelativeCatapultSrcDir = filepath.Join("third_party", "catapult")
	CatapultSrcDir         = filepath.Join(ChromiumSrcDir, RelativeCatapultSrcDir)
	V8SrcDir               = filepath.Join(ChromiumSrcDir, "v8")
	TaskFileDir            = filepath.Join(StorageDir, "current_task")
	GCSTokenPath           = filepath.Join(StorageDir, "google_storage_token.data")
	PagesetsDir            = filepath.Join(StorageDir, PAGESETS_DIR_NAME)
	WebArchivesDir         = filepath.Join(StorageDir, WEB_ARCHIVES_DIR_NAME)
	ApkName                = "ChromePublic.apk"
	SkiaTreeDir            = filepath.Join(RepoDir, "trunk")
	CtTreeDir              = filepath.Join(RepoDir, "go", "src", "go.skia.org", "infra", "ct")

	// Names of local and remote directories and files.
	BinariesDir                    = filepath.Join(BINARIES_DIR_NAME)
	BenchmarkRunsDir               = filepath.Join(TASKS_DIR_NAME, BENCHMARK_TASKS_DIR_NAME)
	BenchmarkRunsStorageDir        = path.Join(TASKS_DIR_NAME, BENCHMARK_TASKS_DIR_NAME)
	ChromiumPerfRunsDir            = filepath.Join(TASKS_DIR_NAME, CHROMIUM_PERF_TASKS_DIR_NAME)
	ChromiumPerfRunsStorageDir     = path.Join(TASKS_DIR_NAME, CHROMIUM_PERF_TASKS_DIR_NAME)
	ChromiumAnalysisRunsStorageDir = path.Join(TASKS_DIR_NAME, CHROMIUM_ANALYSIS_TASKS_DIR_NAME)
	FixArchivesRunsDir             = filepath.Join(TASKS_DIR_NAME, FIX_ARCHIVE_TASKS_DIR_NAME)
	TraceDownloadsDir              = filepath.Join(TASKS_DIR_NAME, TRACE_DOWNLOADS_DIR_NAME)

	// Information about the different CT pageset types.
	PagesetTypeToInfo = map[string]*PagesetTypeInfo{
		PAGESET_TYPE_ALL: {
			NumPages:                   1000000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 1M (with desktop user-agent)",
		},
		PAGESET_TYPE_100k: {
			NumPages:                   100000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 100K (with desktop user-agent)",
		},
		PAGESET_TYPE_MOBILE_100k: {
			NumPages:                   100000,
			CSVSource:                  "csv/android-top-1m.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 100K (with mobile user-agent)",
		},
		PAGESET_TYPE_LAYOUTSHIFT_100k: {
			NumPages:                   100000,
			CSVSource:                  "csv/layout-shift-100k.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Layout Shift 100K (with desktop user-agent)",
		},
		PAGESET_TYPE_10k: {
			NumPages:                   10000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 10K (with desktop user-agent)",
		},
		PAGESET_TYPE_MOBILE_10k: {
			NumPages:                   10000,
			CSVSource:                  "csv/android-top-1m.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 10K (with mobile user-agent)",
		},
		PAGESET_TYPE_MOBILE_VOLT_10k: {
			NumPages:                   10000,
			CSVSource:                  "csv/volt-10k.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Volt 10K (with mobile user-agent)",
		},
		PAGESET_TYPE_LAYOUTSHIFT_10k: {
			NumPages:                   10000,
			CSVSource:                  "csv/layout-shift-10k.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Layout Shift 10K (with desktop user-agent)",
		},
		PAGESET_TYPE_AMP_LIVE_REPRO: {
			NumPages:                   3600,
			CSVSource:                  "csv/amp-cls-ct-live-repro.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "AMP live repro (with mobile user-agent)",
		},
		PAGESET_TYPE_AMP_PUPPETEER_SITES: {
			NumPages:                   1000,
			CSVSource:                  "csv/amp-cls-puppeteer-sites.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "AMP puppeteer sites (with mobile user-agent)",
		},
		PAGESET_TYPE_DUMMY_1k: {
			NumPages:                   1000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 1K (with desktop user-agent, for testing, hidden from Runs History by default)",
		},
		PAGESET_TYPE_MOBILE_DUMMY_1k: {
			NumPages:                   1000,
			CSVSource:                  "csv/android-top-1m.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  1800,
			CaptureArchivesTimeoutSecs: 300,
			RunChromiumPerfTimeoutSecs: 300,
			Description:                "Top 1K (with mobile user-agent, for testing, hidden from Runs History by default)",
		},
	}

	// Frontend constants below.
	SupportedBenchmarksToDoc = map[string]string{
		BENCHMARK_RR:                       "https://cs.chromium.org/chromium/src/tools/perf/contrib/cluster_telemetry/rasterize_and_record_micro_ct.py",
		BENCHMARK_LOADING:                  "https://cs.chromium.org/chromium/src/tools/perf/contrib/cluster_telemetry/v8_loading_ct.py",
		BENCHMARK_USECOUNTER:               "https://docs.google.com/document/d/1FSzJm2L2ow6pZTM_CuyHNJecXuX7Mx3XmBzL4SFHyLA/",
		BENCHMARK_LEAK_DETECTION:           "https://docs.google.com/document/d/1wUWa7dWUdvr6dLdYHFfMQdnvgzt7lrrvzYfpAK-_6e0/",
		BENCHMARK_RENDERING:                "https://cs.chromium.org/chromium/src/tools/perf/contrib/cluster_telemetry/rendering_ct.py",
		BENCHMARK_MEMORY:                   "https://cs.chromium.org/chromium/src/tools/perf/contrib/cluster_telemetry/memory_ct.py",
		BENCHMARK_V8_LOADING:               "https://cs.chromium.org/chromium/src/tools/perf/contrib/cluster_telemetry/v8_loading_ct.py",
		BENCHMARK_V8_LOADING_RUNTIME_STATS: "https://cs.chromium.org/chromium/src/tools/perf/contrib/cluster_telemetry/v8_loading_runtime_stats_ct.py",
		BENCHMARK_GENERIC_TRACE:            "https://docs.google.com/document/d/1vGd7dnrxayMYHPO72wWkwTvjMnIRrel4yxzCr1bMiis/",
		BENCHMARK_AD_TAGGING:               "https://docs.google.com/document/d/1zlWQoLjGuYOWDR_vkVRYoVbU89JetNDOlcDuOaNAzDc/",
		BENCHMARK_LAYOUT_SHIFT:             "https://docs.google.com/document/d/1bYffpPHWFVaaAve2OZCFuv5xentVlFZF4GxZTaDLoXc/",
	}

	SupportedPlatformsToDesc = map[string]string{
		PLATFORM_LINUX:   "Linux (Ubuntu18.04 machines)",
		PLATFORM_ANDROID: "Android (Pixel2 devices)",
	}

	TaskPrioritiesToDesc = map[int]string{
		TASKS_PRIORITY_HIGH:   "High priority",
		TASKS_PRIORITY_MEDIUM: "Medium priority",
		TASKS_PRIORITY_LOW:    "Low priority",
	}

	// Swarming machine dimensions.
	GCE_LINUX_MASTER_DIMENSIONS = map[string]string{"pool": "CTMaster", "os": "Linux", "cores": "4"}

	GCE_LINUX_WORKER_DIMENSIONS   = map[string]string{"pool": SWARMING_POOL, "os": "Linux", "cores": "4"}
	GCE_WINDOWS_WORKER_DIMENSIONS = map[string]string{"pool": SWARMING_POOL, "os": "Windows", "cores": "4"}

	GCE_ANDROID_BUILDER_DIMENSIONS = map[string]string{"pool": "CTAndroidBuilder", "cores": "64"}
	GCE_LINUX_BUILDER_DIMENSIONS   = map[string]string{"pool": "CTLinuxBuilder", "cores": "64"}
	GCE_WINDOWS_BUILDER_DIMENSIONS = map[string]string{"pool": "CTBuilder", "os": "Windows"}

	GOLO_ANDROID_WORKER_DIMENSIONS = map[string]string{"pool": SWARMING_POOL, "os": "Android", "inside_docker": "1", "device_type": "walleye"}
	GOLO_LINUX_WORKER_DIMENSIONS   = map[string]string{"pool": SWARMING_POOL, "os": "Linux", "cores": "8"}

	// ct-perf.skia.org constants.
	CTPerfWorkDir = filepath.Join(StorageDir, "ct-perf-workdir")
)
