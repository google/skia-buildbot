package util

import "path/filepath"

const (
	// TODO(rmistry): Switch this to use chrome-bot when ready to run in prod
	CT_USER                  = "rmistry"
	NUM_WORKERS          int = 100
	WORKER_NAME_TEMPLATE     = "build%s-m5"
	GS_BUCKET_NAME           = "cluster-telemetry"

	// File names and dir names.
	TIMESTAMP_FILE_NAME   = "TIMESTAMP"
	PAGESETS_DIR_NAME     = "page_sets"
	WEB_ARCHIVES_DIR_NAME = "webpage_archives"
	SKPS_DIR_NAME         = "skp"

	// Limit the number of times CT tries to get a remote file before giving up.
	MAX_URI_GET_TRIES = 4

	// Activity constants.
	ACTIVITY_CREATING_PAGESETS   = "CREATING_PAGESETS"
	ACTIVITY_CAPTURING_ARCHIVES  = "CAPTURING_ARCHIVES"
	ACTIVITY_RUNNING_BENCHMARKS  = "RUNNING_BENCHMARKS"
	ACTIVITY_RUNNING_LUA_SCRIPTS = "RUNNING_LUA_SCRIPTS"

	// Pageset types supported by CT.
	PAGESET_TYPE_ALL        = "All"
	PAGESET_TYPE_10k        = "10k"
	PAGESET_TYPE_MOBILE_10k = "Mobile10k"
	PAGESET_TYPE_DUMMY_10k  = "Dummy10k" // Used for testing.
)

type PagesetTypeInfo struct {
	NumPages  int
	CSVSource string
	UserAgent string
}

var (
	// Slaves  = GetCTWorkers()
	// TODO(rmistry): Switch this to use GetCTWorkers() when ready to run in prod
	Slaves = []string{
		"epoger-linux.cnc.corp.google.com",
		"piraeus.cnc.corp.google.com",
		"172.23.212.25",
	}

	// Names of local directories and files.
	StorageDir     = filepath.Join("/", "b", "storage")
	TaskFileDir    = filepath.Join(StorageDir, "current_task")
	GSTokenPath    = filepath.Join(StorageDir, "google_storage_token.data")
	PagesetsDir    = filepath.Join(StorageDir, PAGESETS_DIR_NAME)
	WebArchivesDir = filepath.Join(StorageDir, WEB_ARCHIVES_DIR_NAME)
	SkpsDir        = filepath.Join(StorageDir, SKPS_DIR_NAME)

	// Information about the different CT pageset types.
	PagesetTypeToInfo = map[string]*PagesetTypeInfo{
		PAGESET_TYPE_ALL: &PagesetTypeInfo{
			NumPages:  1000000,
			CSVSource: "csv/top-1m.csv",
			UserAgent: "desktop"},
		PAGESET_TYPE_10k: &PagesetTypeInfo{
			NumPages:  10000,
			CSVSource: "csv/top-1m.csv",
			UserAgent: "desktop"},
		PAGESET_TYPE_MOBILE_10k: &PagesetTypeInfo{
			NumPages:  10000,
			CSVSource: "csv/android-top-1m.csv",
			UserAgent: "mobile"},
		PAGESET_TYPE_DUMMY_10k: &PagesetTypeInfo{
			NumPages:  10000,
			CSVSource: "csv/android-top-1m.csv",
			UserAgent: "mobile"},
	}
)
