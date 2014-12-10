/*
	Pulls data from multiple sources and funnels into InfluxDB.
*/

package main

import (
	"flag"
	"time"

	"github.com/golang/glog"
	influxdb "github.com/influxdb/influxdb/client"
	"skia.googlesource.com/buildbot.git/go/buildbot"
	"skia.googlesource.com/buildbot.git/go/common"
	"skia.googlesource.com/buildbot.git/go/metadata"
	"skia.googlesource.com/buildbot.git/monitoring/go/autoroll_ingest"
)

const (
	INFLUXDB_NAME_METADATA_KEY     = "influxdb_name"
	INFLUXDB_PASSWORD_METADATA_KEY = "influxdb_password"
)

// flags
var (
	graphiteServer   = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	useMetadata      = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	influxDbHost     = flag.String("influxdb_host", "localhost:8086", "The InfluxDB hostname.")
	influxDbName     = flag.String("influxdb_name", "root", "The InfluxDB username.")
	influxDbPassword = flag.String("influxdb_password", "root", "The InfluxDB password.")
	influxDbDatabase = flag.String("influxdb_database", "", "The InfluxDB database.")
	workdir          = flag.String("workdir", ".", "Working directory used by data processors.")
	testing          = flag.Bool("testing", false, "Whether we're running in non-production, testing mode.")
)

func main() {
	common.InitWithMetrics("datahopper", *graphiteServer)

	// Prepare the InfluxDB credentials. Load from metadata if appropriate.
	if *useMetadata {
		*influxDbName = metadata.MustGet(INFLUXDB_NAME_METADATA_KEY)
		*influxDbPassword = metadata.MustGet(INFLUXDB_PASSWORD_METADATA_KEY)
	}
	dbClient, err := influxdb.New(&influxdb.ClientConfig{
		Host:       *influxDbHost,
		Username:   *influxDbName,
		Password:   *influxDbPassword,
		Database:   *influxDbDatabase,
		HttpClient: nil,
		IsSecure:   false,
		IsUDP:      false,
	})
	if err != nil {
		glog.Fatalf("Failed to initialize InfluxDB client: %s", err)
	}

	// Data generation goroutines.
	go autoroll_ingest.LoadAutoRollData(dbClient, *workdir)
	go func() {
		// Initialize the buildbot database.
		if err := buildbot.InitDB(buildbot.ProdDatabaseConfig(*testing)); err != nil {
			glog.Fatal(err)
		}

		// Ingest data in a loop.
		for _ = range time.Tick(30 * time.Second) {
			if err := buildbot.IngestNewBuilds(); err != nil {
				glog.Errorf("Failed to ingest new builds: %v", err)
			}
		}
	}()

	// Wait while the above goroutines generate data.
	select {}
}
