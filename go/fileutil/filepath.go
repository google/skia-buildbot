package fileutil

import (
	"fmt"
	"time"
)

// GetHourlyDirs generates paths with the given prefix for all that paths of the
// form 'prefix/<year>/<month>/<day>/<hour>'. All path segments are padded with zeros
// where necessary, e.g. prefix/2018/01/02/09. All hours are from 0-23. This is
// intended to enumerate the directories that should be polled when ingesting data.
func GetHourlyDirs(prefixDir string, startTS int64, endTS int64) []string {
	if endTS <= startTS {
		return []string{}
	}

	// The result will be roughly the number of hourse in the range.
	ret := make([]string, 0, (endTS-startTS)/3600)
	currTime := time.Unix(startTS, 0).UTC()
	endTime := time.Unix(endTS, 0).UTC().Add(time.Hour)

	for currTime.Before(endTime) {
		year, month, day := currTime.Date()
		hour := currTime.Hour()
		ret = append(ret, fmt.Sprintf("%s/%04d/%02d/%02d/%02d", prefixDir, year, month, day, hour))
		currTime = currTime.Add(time.Hour)
	}
	return ret
}
