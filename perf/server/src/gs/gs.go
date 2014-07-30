// Package gs implements utility for accessing Skia perf data in Google Storage.
package gs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

import (
	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/golang/glog"
)

import (
	"config"
	"types"
)

var (
	// dirMap maps dataset name to a slice with Google Storage subdirectory and file prefix.
	dirMap = map[string][]string{
		"skps":  {"pics-json-v2", "bench_"},
		"micro": {"stats-json-v2", "microbench2_"},
	}

	trybotDataPath = regexp.MustCompile(`^[a-z]*[/]?([0-9]{4}/[0-9]{2}/[0-9]{2}/[0-9]{2}/[0-9a-zA-Z-]+-Trybot/[0-9]+)$`)
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

// JSONPerfInput stores the input JSON data that we care about. Currently this
// includes "key" and "value" fields in perf/server/(microbench|skpbench).json.
type JSONPerfInput struct {
	Value  float64                `json:"value"`
	Params map[string]interface{} `json:"params"`
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

// getTryData takes a prefix path and dataset, and returns the trybot JSON data stored in
// Google Storage under the prefix.
//
// The given prefix path is the path to a trybot build result, such as:
// "trybots/micro/2014/07/16/01/Perf-Win7-ShuttleA-HD2000-x86-Release-Trybot/57"
//
// Currently it takes in JSON format that's used for BigQuery ingestion, and
// outputs in the TileGUI format defined in src/types. Only the Traces fields
// are populated in the TileGUI, with data containing [[0, <value>]] for just
// one data point per key.
//
// TODO(bensong) adjust input/output formats as needed by the inputs and the
// frontend.
func getTryData(prefix string, dataset config.DatasetName) ([]byte, error) {
	gs, err := GetStorageService()
	if err != nil {
		return nil, fmt.Errorf("Unable to get GS service: %s", nil)
	}
	t := types.NewGUITile(-1, -1) // Tile level/number don't matter here.
	req := gs.Objects.List(GS_PROJECT_BUCKET).Prefix(prefix)
	for req != nil {
		resp, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("Google Storage request error: %s", err)
		}
		for _, result := range resp.Items {
			r, err := RequestForStorageURL(result.MediaLink)
			if err != nil {
				return nil, fmt.Errorf("Google Storage MediaLink request error: %s", err)
			}
			res, err := http.DefaultClient.Do(r)
			defer res.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("GET error: %s", err)
			}
			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				return nil, fmt.Errorf("Read body error: %s", err)
			}
			i := JSONPerfInput{}
			for _, j := range bytes.Split(body, []byte("\n")) {
				if len(j) == 0 {
					continue
				}
				if err := json.Unmarshal(j, &i); err != nil {
					return nil, fmt.Errorf("JSON unmarshal error: %s", err)
				}
				newData := make([][2]float64, 0)
				newData = append(newData, [2]float64{
					0.0, // Commit timestamp is unused.
					i.Value,
				})
				if _, exists := i.Params["builderName"]; !exists {
					continue
				}
				// Remove the -Trybot prefix so the trybot keys
				// and normal keys match.
				i.Params["builderName"] = strings.Replace(fmt.Sprint(i.Params["builderName"]), "-Trybot", "", 1)
				t.Traces = append(t.Traces, types.TraceGUI{
					Data: newData,
					Key:  string(types.MakeTraceKey(i.Params, dataset)),
				})
			}
		}
		if len(resp.NextPageToken) > 0 {
			req.PageToken(resp.NextPageToken)
		} else {
			req = nil
		}
	}
	d, err := json.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("JSON marshal error: %s", err)
	}
	return d, nil
}

// GetTryResults takes a string of path info, an end timestamp, and the
// number of days to look back, and returns corresponding trybot results from
// Google Storage.
//
// When a full bench path is not given (only the optional dataset info), returns
// a JSON bytes like:
// {"dirs":["2014/07/16/01/Perf-Win7-ShuttleA-HD2000-x86-Release-Trybot/57"]}
//
// If a valid urlpath is given for a specific try run, returns JSON from
// getTryData() above.
//
// TODO(bensong): add metrics for GS roundtrip time and failure rates.
func GetTryResults(urlpath string, endTS int64, daysback int) ([]byte, error) {
	dirParts := strings.Split(urlpath, "/")
	datasetName := config.DATASET_SKP
	dataset := "pics-json-v2"
	dataFilePrefix := "bench_"
	if k, ok := dirMap[dirParts[0]]; ok {
		datasetName = config.DatasetName(dirParts[0])
		dataset = k[0]
		dataFilePrefix = k[1]
	}
	if len(dirParts) == 1 { // Tries to return list of try result dirs.
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
	} else { // Tries to read stats from the given dir.
		if !trybotDataPath.MatchString(urlpath) {
			return nil, fmt.Errorf("Wrong URL path format for trybot stats: %s\n", urlpath)
		}
		trymatch := trybotDataPath.FindStringSubmatch(urlpath)
		if trymatch == nil { // This should never happen after the check above?
			return nil, fmt.Errorf("Cannot find trybot path in regexp for: %s\n", urlpath)
		}
		return getTryData(path.Join("trybot", dataset, trymatch[1], dataFilePrefix), datasetName)
	}
}
