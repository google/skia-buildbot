// Package gs implements utility for accessing Skia perf data in Google Storage.
package gs

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

import (
	"github.com/skia-dev/glog"
	storage "google.golang.org/api/storage/v1"
)

var (
	// dirMap maps dataset name to a slice with Google Storage subdirectory and file prefix.
	dirMap = map[string][]string{
		"skps":  {"pics-json-v2", "bench_"},
		"micro": {"stats-json-v2", "microbench2_"},
	}

	trybotDataPath = regexp.MustCompile(`^[a-z]*[/]?([0-9]{4}/[0-9]{2}/[0-9]{2}/[0-9]{2}/[0-9a-zA-Z-]+-Trybot/[0-9]+/[0-9]+)$`)
)

// GetStorageService returns a Cloud Storage service.
func GetStorageService() (*storage.Service, error) {
	return storage.New(http.DefaultClient)
}

// lastDate takes a year and month, and returns the last day of the month.
//
// This is done by going to the first day 0:00 of the next month, subtracting an
// hour, then returning the date.
func lastDate(year int, month time.Month) int {
	return time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Hour).Day()
}

// GetLatestGSDirs gets the appropriate directory names in which data
// would be stored between the given timestamp range.
//
// The returning directories cover the range till the date of startTS, and may
// be precise to the hour.
func GetLatestGSDirs(startTS int64, endTS int64, bsSubdir string) []string {
	startTime := time.Unix(startTS, 0).UTC()
	startYear, startMonth, startDay := startTime.Date()
	glog.Infoln("GS dir start time: ", startTime)
	endTime := time.Unix(endTS, 0).UTC()
	lastAddedTime := startTime
	results := make([]string, 0)
	newYear, newMonth, newDay := endTime.Date()
	newHour := endTime.Hour()
	lastYear, lastMonth, _ := lastAddedTime.Date()
	if lastYear != newYear {
		for i := lastYear; i < newYear; i++ {
			if i != startYear {
				results = append(results, fmt.Sprintf("%04d", i))
			} else {
				for j := startMonth; j <= time.December; j++ {
					if j == startMonth && startDay > 1 {
						for k := startDay; k <= lastDate(i, j); k++ {
							results = append(results, fmt.Sprintf("%04d/%02d/%02d", i, j, k))
						}
					} else {
						results = append(results, fmt.Sprintf("%04d/%02d", i, j))
					}
				}
			}
		}
		lastAddedTime = time.Date(newYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	lastYear, lastMonth, _ = lastAddedTime.Date()
	if lastMonth != newMonth {
		for i := lastMonth; i < newMonth; i++ {
			if i != startMonth {
				results = append(results, fmt.Sprintf("%04d/%02d", lastYear, i))
			} else {
				for j := startDay; j <= lastDate(lastYear, i); j++ {
					results = append(results, fmt.Sprintf("%04d/%02d/%02d", lastYear, i, j))
				}
			}
		}
		lastAddedTime = time.Date(newYear, newMonth, 1, 0, 0, 0, 0, time.UTC)
	}
	lastYear, lastMonth, lastDay := lastAddedTime.Date()
	if lastDay != newDay {
		for i := lastDay; i < newDay; i++ {
			results = append(results, fmt.Sprintf("%04d/%02d/%02d", lastYear, lastMonth, i))
		}
		lastAddedTime = time.Date(newYear, newMonth, newDay, 0, 0, 0, 0, time.UTC)
	}
	lastYear, lastMonth, lastDay = lastAddedTime.Date()
	lastHour := lastAddedTime.Hour()
	for i := lastHour; i < newHour+1; i++ {
		results = append(results, fmt.Sprintf("%04d/%02d/%02d/%02d", lastYear, lastMonth, lastDay, i))
	}
	for i := range results {
		results[i] = fmt.Sprintf("%s/%s", bsSubdir, results[i])
	}
	return results
}

// RequestForStorageURL returns an http.Request for a given Cloud Storage URL.
// This is workaround of a known issue: embedded slashes in URLs require use of
// URL.Opaque property
func RequestForStorageURL(url string) (*http.Request, error) {
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTP new request error: %s", err)
	}
	schemePos := strings.Index(url, ":")
	queryPos := strings.Index(url, "?")
	if queryPos == -1 {
		queryPos = len(url)
	}
	r.URL.Opaque = url[schemePos+1 : queryPos]
	return r, nil
}
