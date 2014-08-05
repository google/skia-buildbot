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
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

var (
	// dirMap maps dataset name to a slice with Google Storage subdirectory and file prefix.
	dirMap = map[string][]string{
		"skps":  {"pics-json-v2", "bench_"},
		"micro": {"stats-json-v2", "microbench2_"},
	}

	trybotDataPath = regexp.MustCompile(`^[a-z]*[/]?([0-9]{4}/[0-9]{2}/[0-9]{2}/[0-9]{2}/[0-9a-zA-Z-]+-Trybot/[0-9]+/[0-9]+)$`)
)

const (
	GS_PROJECT_BUCKET = "chromium-skia-gm"
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

// RunInfo stores trybot run result info for a requester.
//
// Issues maps a string representing Reitveld issue info to a slice of dirs
// containing its try results. A sample dir looks like:
// "2014/07/31/18/Perf-Win7-ShuttleA-HD2000-x86-Release-Trybot/75/423413006"
type RunInfo struct {
	Requester string              `json:"requester"`
	Issues    map[string][]string `json:"issues"`
}

// TryInfo stores try result information on Google Storage bench files.
type TryInfo struct {
	Results []*RunInfo `json:"results"`
}

// IssueInfo stores information on a specific issue.
//
// Information is read from the Reitveld JSON api, for instance,
// https://codereview.chromium.org/api/427903003
// Only information we care about is stored.
type IssueInfo struct {
	Owner   string `json:"owner"`
	Subject string `json:"subject"`
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
// "trybots/micro/2014/07/16/01/Perf-Win7-ShuttleA-HD2000-x86-Release-Trybot/57/423413006"
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
					Key:  string(types.MakeTraceKey(i.Params)),
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
// a JSON bytes marshalled from TryInfo.
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
		results := &TryInfo{
			Results: []*RunInfo{},
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
		requesterIssues := map[string][]string{}
		issueDirs := map[string][]string{}
		issueDescription := map[string]string{}

		for k := range m {
			if match := trybotDataPath.FindStringSubmatch(k); match == nil {
				glog.Infoln("Unexpected try path, skipping: ", k)
				continue
			}
			s := strings.Split(k, "/")
			issue := s[len(s)-1]
			if _, ok := issueDirs[issue]; !ok {
				issueDirs[issue] = []string{}
			}
			issueDirs[issue] = append(issueDirs[issue], k)

			if _, ok := issueDescription[issue]; ok {
				continue
			}
			resp, err := http.Get("https://codereview.chromium.org/api/" + issue)
			defer resp.Body.Close()
			owner := "unknown"
			description := "unknown"
			if err != nil {
				glog.Warningln("Could not get Reitveld info, use unknown: ", err)
			} else {
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					glog.Warningln("Could not read Reitveld info, use unknown: ", err)
				}
				i := IssueInfo{}
				if err := json.Unmarshal(body, &i); err != nil {
					glog.Warningln("Could not unmarshal Reitveld info, use unknown: ", err)
				} else {
					owner = i.Owner
					description = i.Subject
				}
			}
			issueDescription[issue] = description
			if _, ok := requesterIssues[owner]; !ok {
				requesterIssues[owner] = []string{}
			}
			requesterIssues[owner] = append(requesterIssues[owner], issue)
		}
		for k, v := range requesterIssues {
			issues := map[string][]string{}
			for _, i := range v {
				issues[i+": "+issueDescription[i]] = issueDirs[i]
			}
			r := &RunInfo{
				Requester: k,
				Issues:    issues,
			}
			results.Results = append(results.Results, r)
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
