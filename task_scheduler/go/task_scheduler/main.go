package main

import (
	"flag"
	"path"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/scheduling"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "task_scheduler"

	// DB_NAME is the name of the database.
	DB_NAME = "task_scheduler_db"

	// DB_FILENAME is the name of the file in which the database is stored.
	DB_FILENAME = "task_scheduler.bdb"
)

var (
	// "Constants"

	// REPOS are the repositories to query.
	REPOS = []string{
		common.REPO_SKIA,
		common.REPO_SKIA_INFRA,
	}

	// Task Scheduler instance.
	ts *scheduling.TaskScheduler

	// Flags.
	local      = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	timePeriod = flag.String("timePeriod", "4d", "Time period to use.")
	workdir    = flag.String("workdir", "workdir", "Working directory to use.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

func main() {
	defer common.LogPanic()

	// Global init.
	common.InitWithMetrics2(APP_NAME, influxHost, influxUser, influxPassword, influxDatabase, local)

	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	// Parse the time period.
	period, err := human.ParseDuration(*timePeriod)
	if err != nil {
		glog.Fatal(err)
	}

	// Authenticated HTTP client.
	oauthCacheFile := path.Join(*workdir, "google_storage_token.data")
	httpClient, err := auth.NewClient(*local, oauthCacheFile, swarming.AUTH_SCOPE)
	if err != nil {
		glog.Fatal(err)
	}

	// Initialize Swarming client.
	swarm, err := swarming.NewApiClient(httpClient)
	if err != nil {
		glog.Fatal(err)
	}

	// Initialize Isolate client.
	isolate, err := isolate.NewClient(*workdir)
	if err != nil {
		glog.Fatal(err)
	}

	// Initialize the database.
	// TODO(benjaminwagner): Create a signal handler which closes the DB.
	d, err := local_db.NewDB(DB_NAME, path.Join(*workdir, DB_FILENAME))
	if err != nil {
		glog.Fatal(err)
	}
	defer util.Close(d)

	// ... and database cache.
	cache, err := db.NewTaskCache(d, period)
	if err != nil {
		glog.Fatal(err)
	}

	// Create and start the task scheduler.
	glog.Infof("Creating task scheduler.")
	ts, err = scheduling.NewTaskScheduler(d, cache, period, *workdir, REPOS, isolate, swarm)
	if err != nil {
		glog.Fatal(err)
	}

	glog.Infof("Created task scheduler. Starting loop.")
	ts.Start()

	select {}
}
