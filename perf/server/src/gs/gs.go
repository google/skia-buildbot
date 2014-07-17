// Package gs implements utility for accessing Skia perf data in Google Storage.
package gs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"
)

import (
	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/golang/glog"
)

var (
	// dirMap maps dataset name to Google Storage subdirectory.
	dirMap = map[string]string{
		"skps":  "pics-json-v2",
		"micro": "stats-json-v2",
	}
)

const (
	GS_PROJECT_BUCKET = "chromium-skia-gm"
)

// GetStorageService returns a Cloud Storage service.
func GetStorageService() (*storage.Service, error) {
	return storage.New(http.DefaultClient)
}

// GetLatestGSDirs gets the appropriate directory names in which data
// would be stored between the given timestamp range.
//
// Adapted from perf/server/src/ingest/ingest.go for future sharing.
func GetLatestGSDirs(startTS int64, endTS int64, bsSubdir string) []string {
	startTime := time.Unix(startTS, 0).UTC()
	glog.Infoln("GS dir start time: ", startTime)
	endTime := time.Unix(endTS, 0).UTC()
	lastAddedTime := startTime
	results := make([]string, 0)
	newYear, newMonth, newDay := endTime.Date()
	newHour := endTime.Hour()
	lastYear, lastMonth, _ := lastAddedTime.Date()
	if lastYear != newYear {
		for i := lastMonth; i < 12; i++ {
			results = append(results, fmt.Sprintf("%04d/%02d", lastYear, lastMonth))
		}
		for i := lastYear + 1; i < newYear; i++ {
			results = append(results, fmt.Sprintf("%04d", i))
		}
		lastAddedTime = time.Date(newYear, 0, 1, 0, 0, 0, 0, time.UTC)
	}
	lastYear, lastMonth, _ = lastAddedTime.Date()
	if lastMonth != newMonth {
		for i := lastMonth; i < newMonth; i++ {
			results = append(results, fmt.Sprintf("%04d/%02d", lastYear, i))
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

// DirInfo stores directory information on Google Storage bench files.
type DirInfo struct {
	Dirs []string `json:"dirs"`
}

// GetTryResults takes a string of path info, an end timestamp, and the
// number of days to look back, and returns corresponding trybot results from
// Google Storage.
//
// When a full bench path is not given (only the optional dataset info), returns
// a JSON bytes like:
// {"dirs":["2014/07/16/01/Perf-Win7-ShuttleA-HD2000-x86-Release-Trybot/57"]}
//
// TODO(bensong): returns actual bench data if a full path to file level is given.
// TODO(bensong): add metrics for GS roundtrip time and failure rates.
func GetTryResults(urlpath string, endTS int64, daysback int) ([]byte, error) {
	// TODO(bensong): validate path elements with regexp.
	dirParts := strings.Split(urlpath, "/")
	dataset := "pics-json-v2"
	if k, ok := dirMap[dirParts[0]]; ok {
		dataset = k
	}
	if len(dirParts) == 1 {  // Tries to return list of try result dirs.
		results := &DirInfo{
			Dirs: []string{},
		}
		dirs := GetLatestGSDirs(time.Unix(endTS, 0).UTC().AddDate(0, 0, 0-daysback).Unix(), endTS, "trybot/"+dataset)
		if len(dirs) == 0 {
			return json.Marshal(results)
		}
		gs, err := GetStorageService()
		if err != nil {
			return nil, fmt.Errorf("Unable to get GS service: %s", nil)
		}
		m := make(map[string]bool)
		for _, dir := range dirs {
			req := gs.Objects.List(GS_PROJECT_BUCKET).Prefix(dir)
			for req != nil {
				resp, err := req.Do()
				if err != nil {
					return nil, fmt.Errorf("Google Storage request error: %s", err)
				}
				for _, result := range resp.Items {
					// Extracts the useful parts.
					dirPath := path.Dir(result.Name)
					// Removes "trybot" and dataset
					toReturn := strings.Split(dirPath, "/")[2:]
					m[strings.Join(toReturn, "/")] = true
				}
				if len(resp.NextPageToken) > 0 {
					req.PageToken(resp.NextPageToken)
				} else {
					req = nil
				}
			}
		}
		for k := range m {
			results.Dirs = append(results.Dirs, k)
		}
		return json.Marshal(results)
	} else {  // Tries to read stats from the given dir.
		return nil, fmt.Errorf("Try bench stats request not supported yet.")
	}
}
