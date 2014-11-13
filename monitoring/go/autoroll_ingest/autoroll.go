/*
	Generates AutoRoll data and pushes it into InfluxDB.
*/
package autoroll_ingest

import (
	"path"
	"time"

	"github.com/golang/glog"
	influxdb "github.com/influxdb/influxdb/client"
	"skia.googlesource.com/buildbot.git/go/autoroll"
	"skia.googlesource.com/buildbot.git/go/gitinfo"
)

const (
	CHROMIUM_REPO                 = "https://chromium.googlesource.com/chromium/src"
	SKIA_REPO                     = "https://skia.googlesource.com/skia"
	SERIES_AUTOROLL_CURRENTSTATUS = "autoroll.currentstatus"
)

var (
	COLUMNS_AUTOROLL_CURRENTSTATUS = []string{
		"last_roll_revision",
		"current_roll_revision",
		"current_roll",
		"head",
		"status",
	}
)

// writeAutoRollDataPoint pushes the current AutoRollStatus into InfluxDB.
func writeAutoRollDataPoint(dbClient *influxdb.Client, status *autoroll.AutoRollStatus) error {
	issue := -1
	if status.Status != autoroll.STATUS_IDLE {
		issue = status.CurrentRoll.Issue
	}
	point := []interface{}{
		interface{}(status.LastRollRevision),
		interface{}(status.CurrentRollRevision),
		interface{}(issue),
		interface{}(status.Head),
		interface{}(status.Status),
	}
	series := influxdb.Series{
		Name:    SERIES_AUTOROLL_CURRENTSTATUS,
		Columns: COLUMNS_AUTOROLL_CURRENTSTATUS,
		Points:  [][]interface{}{point},
	}
	seriesList := []*influxdb.Series{&series}
	glog.Infof("Pushing datapoint to %s: %v", SERIES_AUTOROLL_CURRENTSTATUS, point)
	return dbClient.WriteSeries(seriesList)
}

// LoadAutoRollData continually determines the state of the AutoRoll Bot and
// pushes it into InfluxDB.
func LoadAutoRollData(dbClient *influxdb.Client, workdir string) {
	rollCheckoutsDir := path.Join(workdir, "autoroll_git")
	skiaRepo, err := gitinfo.CloneOrUpdate(SKIA_REPO, path.Join(rollCheckoutsDir, "skia"))
	if err != nil {
		glog.Errorf("Failed to check out skia: %s", err)
		return
	}
	chromiumRepo, err := gitinfo.CloneOrUpdate(CHROMIUM_REPO, path.Join(rollCheckoutsDir, "chromium"))
	if err != nil {
		glog.Errorf("Failed to check out chromium: %s", err)
		return
	}

	for _ = range time.Tick(time.Minute) {
		s, err := autoroll.CurrentStatus(skiaRepo, chromiumRepo)
		if err != nil {
			glog.Error(err)
		} else {
			err := writeAutoRollDataPoint(dbClient, s)
			if err != nil {
				glog.Error(err)
			}
		}
		skiaRepo.Update(true)
		chromiumRepo.Update(true)
	}
}
