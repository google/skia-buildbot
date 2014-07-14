package main

import (
	"flag"
	"time"
)

import (
	"github.com/golang/glog"
)

var (
	runOnce       = flag.Bool("run_once", false, "Determines if the ingestor will run a single time or at intervals")
	pollInterval  = flag.Duration("poll_every", time.Duration(900)*time.Second, "Number of seconds between polls")
	useOAuth      = flag.Bool("oauth", true, "Run OAuth to authenticate user")
	bqDestDataset = flag.String("bq_dataset", "", "Destination BQ dataset for custom jobs")
	bqDestTable   = flag.String("bq_table", "", "Destination BQ table for custom jobs")
	csSrcDir      = flag.String("cs_dir", "", "Custom CS directory for the ingestor to pull from")
	schema        = flag.String("schema", "", "Custom schema to use")
	timestamp     = flag.Int64("timestamp", -1, "Timestamp of last data entry into BQ")
	oneshot       = flag.Bool("one_shot", false, "To run until finished or just once")
	logOnly       = flag.Bool("log_only", false, "Go through the motions, but don't actually push data into BigQuery.")
)

func main() {
	flag.Parse()
	Init()
	ingester := NewIngestService(*useOAuth)
	if ingester != nil {
		switch {
		case !(*runOnce):
			for _ = range time.Tick(*pollInterval) {
				err := ingester.NormalUpdate()
				if err != nil {
					glog.Errorln("Update failed with error", err)
				}
			}
		case len(*bqDestDataset) > 0 || len(*bqDestTable) > 0 || len(*csSrcDir) > 0:
			glog.Infoln("Running custom request")
			req := &CustomRequest{
				SourceCSDirectory:    *csSrcDir,
				DestinationBQDataset: *bqDestDataset,
				DestinationBQTable:   *bqDestTable,
				Schema:               *schema,
				Timestamp:            -1,
				OneShot:              false,
			}
			if len(*bqDestDataset) == 0 {
				req.DestinationBQDataset = "perf_skps_gotest"
			}
			if len(*bqDestTable) == 0 {
				req.DestinationBQTable = "skpbench"
			}
			if len(*csSrcDir) == 0 {
				req.SourceCSDirectory = "pics-json-v2"
			}
			if len(*schema) == 0 {
				req.Schema = "skpbench"
			}
			if timestamp != nil {
				req.Timestamp = *timestamp
			}
			if oneshot != nil {
				req.OneShot = *oneshot
			}
			ingester.CustomUpdate(req)
		case true:
			glog.Infoln("Running only once\n")
			ingester.NormalUpdate()
		}
	}
}
