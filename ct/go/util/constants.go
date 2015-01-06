package util

import "path/filepath"

const (
	// TODO(rmistry): Switch this to use chrome-bot when ready to run in prod
	CT_USER                  = "rmistry"
	NUM_WORKERS          int = 100
	WORKER_NAME_TEMPLATE     = "build%d-m5"
	GS_BUCKET_NAME           = "cluster-telemetry"
	GS_HTTP_LINK             = "https://storage.cloud.google.com/"

	// File names and dir names.
	TIMESTAMP_FILE_NAME             = "TIMESTAMP"
	CHROMIUM_BUILDS_DIR_NAME        = "chromium_builds"
	PAGESETS_DIR_NAME               = "page_sets"
	WEB_ARCHIVES_DIR_NAME           = "webpage_archives"
	SKPS_DIR_NAME                   = "skps"
	STORAGE_DIR_NAME                = "storage"
	REPO_DIR_NAME                   = "skia-repo"
	TASKS_DIR_NAME                  = "tasks"
	LUA_TASKS_DIR_NAME              = "lua_runs"
	BENCHMARK_TASKS_DIR_NAME        = "benchmark_runs"
	SKIA_CORRECTNESS_TASKS_DIR_NAME = "skia_correctness_runs"

	// Limit the number of times CT tries to get a remote file before giving up.
	MAX_URI_GET_TRIES = 4

	// Activity constants.
	ACTIVITY_CREATING_PAGESETS        = "CREATING_PAGESETS"
	ACTIVITY_CAPTURING_ARCHIVES       = "CAPTURING_ARCHIVES"
	ACTIVITY_RUNNING_BENCHMARKS       = "RUNNING_BENCHMARKS"
	ACTIVITY_RUNNING_LUA_SCRIPTS      = "RUNNING_LUA_SCRIPTS"
	ACTIVITY_RUNNING_SKIA_CORRECTNESS = "RUNNING_SKIA_CORRECTNESS"

	// Pageset types supported by CT.
	PAGESET_TYPE_ALL        = "All"
	PAGESET_TYPE_10k        = "10k"
	PAGESET_TYPE_MOBILE_10k = "Mobile10k"
	PAGESET_TYPE_DUMMY_100  = "Dummy100" // Used for testing.

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
	BENCHMARK_SKPICTURE_PRINTER = "skpicture_printer"
	BENCHMARK_RR                = "rasterize_and_record_micro"
	BENCHMARK_REPAINT           = "repaint"
	BENCHMARK_SMOOTHNESS        = "smoothness"

	// Webapp constants.
	WEBAPP_ROOT = "https://skia-tree-status.appspot.com/skia-telemetry/"
)

type PagesetTypeInfo struct {
	NumPages                   int
	CSVSource                  string
	UserAgent                  string
	CaptureArchivesTimeoutSecs int
	CreatePagesetsTimeoutSecs  int
	RunBenchmarksTimeoutSecs   int
}

var (
	// Slaves  = GetCTWorkers()
	// TODO(rmistry): Switch this to use GetCTWorkers() when ready to run in prod
	Slaves = []string{
		"piraeus.cnc.corp.google.com",
		"172.23.212.25",
		"epoger-linux.cnc.corp.google.com",
	}

	// Names of local directories and files.
	StorageDir           = filepath.Join("/", "b", STORAGE_DIR_NAME)
	RepoDir              = filepath.Join("/", "b", REPO_DIR_NAME)
	ChromiumBuildsDir    = filepath.Join(StorageDir, CHROMIUM_BUILDS_DIR_NAME)
	TelemetryBinariesDir = filepath.Join(StorageDir, "chromium", "src", "tools", "perf")
	TelemetrySrcDir      = filepath.Join(StorageDir, "chromium", "src", "tools", "telemetry")
	TaskFileDir          = filepath.Join(StorageDir, "current_task")
	GSTokenPath          = filepath.Join(StorageDir, "google_storage_token.data")
	EmailTokenPath       = filepath.Join(StorageDir, "email.data")
	WebappPasswordPath   = filepath.Join(StorageDir, "webapp.data")
	PagesetsDir          = filepath.Join(StorageDir, PAGESETS_DIR_NAME)
	WebArchivesDir       = filepath.Join(StorageDir, WEB_ARCHIVES_DIR_NAME)
	SkpsDir              = filepath.Join(StorageDir, SKPS_DIR_NAME)
	GLogDir              = filepath.Join(StorageDir, "glog")
	ApkPath              = filepath.Join("apks", "ChromeShell.apk")
	SkiaTreeDir          = filepath.Join(RepoDir, "trunk")
	CtTreeDir            = filepath.Join(RepoDir, "go", "src", "skia.googlesource.com", "buildbot.git", "ct")

	// Names of remote directories and files.
	LuaRunsDir             = filepath.Join(TASKS_DIR_NAME, LUA_TASKS_DIR_NAME)
	BenchmarkRunsDir       = filepath.Join(TASKS_DIR_NAME, BENCHMARK_TASKS_DIR_NAME)
	SkiaCorrectnessRunsDir = filepath.Join(TASKS_DIR_NAME, SKIA_CORRECTNESS_TASKS_DIR_NAME)

	// Webapp subparts.
	AdminTasksWebapp       = WEBAPP_ROOT + "admin_tasks"
	UpdateAdminTasksWebapp = WEBAPP_ROOT + "update_admin_task"
	LuaTasksWebapp         = WEBAPP_ROOT + "lua_script"
	UpdateLuaTasksWebapp   = WEBAPP_ROOT + "update_lua_task"

	// Information about the different CT pageset types.
	PagesetTypeToInfo = map[string]*PagesetTypeInfo{
		PAGESET_TYPE_ALL: &PagesetTypeInfo{
			NumPages:                   1000000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  60,
			CaptureArchivesTimeoutSecs: 300,
			RunBenchmarksTimeoutSecs:   300,
		},
		PAGESET_TYPE_10k: &PagesetTypeInfo{
			NumPages:                   10000,
			CSVSource:                  "csv/top-1m.csv",
			UserAgent:                  "desktop",
			CreatePagesetsTimeoutSecs:  60,
			CaptureArchivesTimeoutSecs: 300,
			RunBenchmarksTimeoutSecs:   300,
		},
		PAGESET_TYPE_MOBILE_10k: &PagesetTypeInfo{
			NumPages:                   10000,
			CSVSource:                  "csv/android-top-1m.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  60,
			CaptureArchivesTimeoutSecs: 300,
			RunBenchmarksTimeoutSecs:   300,
		},
		PAGESET_TYPE_DUMMY_100: &PagesetTypeInfo{
			NumPages:                   1000,
			CSVSource:                  "csv/android-top-1m.csv",
			UserAgent:                  "mobile",
			CreatePagesetsTimeoutSecs:  60,
			CaptureArchivesTimeoutSecs: 300,
			RunBenchmarksTimeoutSecs:   300,
		},
	}
)
