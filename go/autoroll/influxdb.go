/*
	Generates AutoRoll data and pushes it into InfluxDB.
*/
package autoroll

import (
	"fmt"
	"path"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/influxdb"
)

const (
	CHROMIUM_REPO                 = "https://chromium.googlesource.com/chromium/src"
	SKIA_REPO                     = "https://skia.googlesource.com/skia"
	SERIES_AUTOROLL_CURRENTSTATUS = "autoroll.currentstatus"
)

// IngestAutoRollData continually determines the state of the AutoRoll Bot and
// pushes it into InfluxDB.
func IngestAutoRollData(dbClient *influxdb.Client, workdir string) {
	rollCheckoutsDir := path.Join(workdir, "autoroll_git")
	skiaRepo, err := gitinfo.CloneOrUpdate(SKIA_REPO, path.Join(rollCheckoutsDir, "skia"), false)
	if err != nil {
		glog.Errorf("Failed to check out skia: %s", err)
		return
	}
	chromiumRepo, err := gitinfo.CloneOrUpdate(CHROMIUM_REPO, path.Join(rollCheckoutsDir, "chromium"), false)
	if err != nil {
		glog.Errorf("Failed to check out chromium: %s", err)
		return
	}

	for _ = range time.Tick(time.Minute) {
		s, err := CurrentStatus(skiaRepo, chromiumRepo)
		if err != nil {
			glog.Error(err)
		} else {
			if err := dbClient.WriteDataPoint(s, SERIES_AUTOROLL_CURRENTSTATUS); err != nil {
				glog.Error(err)
			}
		}
		skiaRepo.Update(true, false)
		chromiumRepo.Update(true, false)
	}
}

// GetLastStatusFromInfluxDB obtains the last-ingested AutoRoll Bot state from
// InfluxDB.
func GetLastStatusFromInfluxDB(dbClient *influxdb.Client) (*AutoRollStatus, error) {
	rv := AutoRollStatus{}
	if err := dbClient.Query(&rv, fmt.Sprintf("select * from %s limit 1", SERIES_AUTOROLL_CURRENTSTATUS)); err != nil {
		return nil, err
	}
	return &rv, nil
}
