package fileutil

import (
	"fmt"
	"time"
)

// GetHourlyDirs generates paths with the given prefix for all the paths of the
// form 'prefix/<year>/<month>/<day>/<hour>'. All path segments are padded with zeros
// where necessary, e.g. prefix/2018/01/02/09. All hours are from 0-23. This is
// intended to enumerate the directories that should be polled when ingesting data.
// startTS and endTS are seconds since the Unix epoch. Both time stamps are inclusive.
// The generated directories are based on UTC being the locale.
func GetHourlyDirs(prefixDir string, startTS int64, endTS int64) []string {
	if endTS < startTS {
		return []string{}
	}

	// The result will be roughly the number of hours in the range.
	ret := make([]string, 0, (endTS-startTS)/3600+1)
	currTime := time.Unix(startTS, 0).UTC()
	endTime := time.Unix(endTS, 0).UTC()

	// Change the timestamp of the last hour to the very last millisecond of the
	// hour. This guarantees that we always include the last hour because
	// endTime is just below the next hour but larger than any incremented value
	// of currTime can be since currTime is measured in seconds.
	lastHourDelta := time.Duration(59-endTime.Minute())*time.Minute +
		time.Duration(59-endTime.Second())*time.Second +
		999*time.Millisecond
	endTime = endTime.Add(lastHourDelta)

	for currTime.Before(endTime) {
		year, month, day := currTime.Date()
		hour := currTime.Hour()
		ret = append(ret, fmt.Sprintf("%s/%04d/%02d/%02d/%02d", prefixDir, year, month, day, hour))
		currTime = currTime.Add(time.Hour)
	}
	return ret
}
