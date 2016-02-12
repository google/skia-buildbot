package main

import (
	"flag"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/influxdb"
)

var (
	oldDbFile = flag.String("old_db_file", "", "The database file from which to migrate.")
	newDbFile = flag.String("new_db_file", "", "The file to use for the new database.")
	local     = flag.Bool("local", false, "Whether or not we're in local testing mode.")
	workdir   = flag.String("workdir", ".", "Working directory.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

func main() {
	defer common.LogPanic()
	common.InitWithMetrics2("buildbot_migration", influxHost, influxUser, influxPassword, influxDatabase, local)

	if *oldDbFile == "" {
		glog.Fatal("Must provide --old_db_file.")
	}
	if *newDbFile == "" {
		glog.Fatal("Must provide --new_db_file.")
	}

	newDB, err := buildbot.NewLocalDB(*newDbFile, gitinfo.NewRepoMap(*workdir))
	if err != nil {
		glog.Fatal(err)
	}

	glog.Infof("Migrating builds...")
	if err := buildbot.MigrateBuilds(newDB, *oldDbFile); err != nil {
		glog.Fatal(err)
	}
}
