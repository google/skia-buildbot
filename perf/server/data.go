// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

import (
	"code.google.com/p/goauth2/compute/serviceaccount"
	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/google-api-go-client/bigquery/v2"
	"github.com/golang/glog"
	"github.com/oxtoacart/webbrowser"
	"github.com/rcrowley/go-metrics"
)

const (
	// JSON doesn't support NaN or +/- Inf, so we need a valid float
	// to signal missing data that also has a compact JSON representation.
	MISSING_DATA_SENTINEL = 1e100

	// Don't consider data before this time. May be due to schema changes, etc.
	// Note that the limit is exclusive, this date does not contain good data.
	// TODO(jcgregorio) Make into a flag.
	BEGINNING_OF_TIME = "20140614"

	// Limit the number of commits we hold in memory and do bulk analysis on.
	MAX_COMMITS_IN_MEMORY = 30

	REFRESH_PERIOD = time.Minute * 30
)

// Shouldn't need auth when running from GCE, but will need it for local dev.
// TODO(jcgregorio) Move to reading this from client_secrets.json and void these keys at that point.
var (
	config = &oauth.Config{
		ClientId:     "470362608618-nlbqngfl87f4b3mhqqe9ojgaoe11vrld.apps.googleusercontent.com",
		ClientSecret: "J4YCkfMXFJISGyuBuVEiH60T",
		Scope:        bigquery.BigqueryScope,
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		TokenCache:   oauth.CacheFile("bqtoken.data"),
	}

	lastSkpUpdate          = time.Now()
	timeSinceLastSkpUpdate = metrics.NewRegisteredGauge("data.bigquery.skps.refresh.time_since_last_update", metrics.DefaultRegistry)
)

func init() {
	go func() {
		for _ = range time.Tick(time.Minute) {
			timeSinceLastSkpUpdate.Update(int64(time.Since(lastSkpUpdate).Seconds()))
		}
	}()
}

// runFlow runs through a 3LO OAuth 2.0 flow to get credentials for BigQuery.
func runFlow(config *oauth.Config) (*http.Client, error) {
	transport := &oauth.Transport{Config: config}
	if _, err := config.TokenCache.Token(); err != nil {
		url := config.AuthCodeURL("")
		fmt.Printf(`Your browser has been opened to visit:

  %s

Enter the verification code:`, url)
		webbrowser.Open(url)
		var code string
		fmt.Scan(&code)
		if _, err := transport.Exchange(code); err != nil {
			return nil, err
		}
	}

	return transport.Client(), nil
}

// Trace represents all the values of a single measurement over time.
type Trace struct {
	Key    string            `json:"key"`
	Values []float64         `json:"values"`
	Params map[string]string `json:"params"`
	Trybot bool              `json:"trybot"`
}

// NewTrace allocates a new Trace set up for the given number of samples.
//
// The Trace Values are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewTrace(numSamples int) *Trace {
	t := &Trace{
		Values: make([]float64, numSamples, numSamples),
		Params: make(map[string]string),
		Trybot: false,
	}
	for i, _ := range t.Values {
		t.Values[i] = MISSING_DATA_SENTINEL
	}
	return t
}

// Annotations for commits.
//
// Will map to the table of annotation notes in MySQL. See DESIGN.md
// for the MySQL schema.
type Annotation struct {
	ID     int    `json:"id"`
	Notes  string `json:"notes"`
	Author string `json:"author"`
	Type   int    `json:"type"`
}

// Commit is information about each Git commit.
type Commit struct {
	CommitTime    int64        `json:"commit_time" bq:"timestamp"`
	Hash          string       `json:"hash"        bq:"gitHash"`
	GitNumber     int64        `json:"git_number"  bq:"gitNumber"`
	CommitMessage string       `json:"commit_msg"`
	Annotations   []Annotation `json:"annotations,omitempty"`
}

func NewCommit() *Commit {
	return &Commit{
		Annotations: []Annotation{},
	}
}

// Choices is a list of possible values for a param. See AllData.
type Choices []string

// AllData is the top level struct we return via JSON to the UI.
//
// The length of the Commits array is the same length as all of the Values
// arrays in all of the Traces.
type AllData struct {
	Traces   []*Trace           `json:"traces"`
	ParamSet map[string]Choices `json:"param_set"`
	Commits  []*Commit          `json:"commits"`
	mutex    sync.Mutex
	service  *bigquery.Service
	fullData bool
}

// DateIter allows for easily iterating backwards, one day at a time, until
// reaching the BEGINNING_OF_TIME.
type DateIter struct {
	day       time.Time
	firstLoop bool
}

func NewDateIter() *DateIter {
	return &DateIter{
		day:       time.Now(),
		firstLoop: true,
	}
}

// Next is the iterator step function to be used in a for loop.
func (i *DateIter) Next() bool {
	if i.firstLoop {
		i.firstLoop = false
		return true
	}
	i.day = i.day.Add(-24 * time.Hour)
	return i.Date() != BEGINNING_OF_TIME
}

// Date returns the day formatted as we use them on BigQuery table name suffixes.
func (i *DateIter) Date() string {
	return i.day.Format("20060102")
}

// gitCommitsWithTestData returns the list of commits that have perf data
// associated with them.
//
// Returns a list of dates of tables we had to query, the list of commits ordered
// from oldest to newest, and an error code.
//
// Not all commits will have perf data, the builders don't necessarily run for
// each commit.  Will limit itself to returning only the number of days that
// are needed to get MAX_COMMITS_IN_MEMORY.
func gitCommitsWithTestData(service *bigquery.Service) ([]string, []*Commit, error) {
	dateList := []string{}
	allCommits := make([]*Commit, 0)
	queryTemplate := `
SELECT
  gitHash, FIRST(timestamp) as timestamp, FIRST(gitNumber) as gitNumber
FROM
  perf_skps_v2.skpbench%s
GROUP BY
  gitHash
ORDER BY
  timestamp DESC;
  `
	// Loop over table going backward until we find 30 commits, or hit the BEGINNING_OF_TIME.
	dates := NewDateIter()
	totalCommits := 0
	for dates.Next() {
		query := fmt.Sprintf(queryTemplate, dates.Date())
		iter, err := NewRowIter(service, query)
		if err != nil {
			glog.Warningln("Tried to query a table that didn't exist", dates.Date(), err)
			continue
		}

		for iter.Next() {
			c := NewCommit()
			if err := iter.Decode(c); err != nil {
				return nil, allCommits, fmt.Errorf("Failed reading hashes from BigQuery: %s", err)
			}
			totalCommits++
			if len(allCommits) < MAX_COMMITS_IN_MEMORY {
				allCommits = append(allCommits, c)
			} else {
				break
			}
		}
		dateList = append(dateList, dates.Date())
		glog.Infof("Finding hashes with data, finished day %s, total commits so far %d", dates.Date(), totalCommits)
		if totalCommits >= MAX_COMMITS_IN_MEMORY {
			break
		}
	}
	// Now reverse allCommits so that it is oldest first.
	reversedCommits := make([]*Commit, len(allCommits), len(allCommits))
	for i, c := range allCommits {
		reversedCommits[len(allCommits)-i-1] = c
	}
	return dateList, reversedCommits, nil
}

// GitHash represents information on a single Git commit.
type GitHash struct {
	Hash      string
	TimeStamp time.Time
}

// readCommitsFromGit reads the commit history from a Git repository.
//
// Don't bother with this for now, will eventually need for commit message.
func readCommitsFromGit(dir string) ([]GitHash, error) {
	cmd := exec.Command("git", strings.Split("log --format=%H%x20%ci", " ")...)
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Failed to run Git: %s", err)
	}
	lines := strings.Split(string(b), "\n")
	hashes := make([]GitHash, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			t, err := time.Parse("2006-01-02 15:04:05 -0700", parts[1])
			if err != nil {
				return nil, fmt.Errorf("Failed parsing Git log timestamp: %s", err)
			}
			hashes = append(hashes, GitHash{Hash: parts[0], TimeStamp: t})
		}
	}
	return hashes, nil
}

// RowIter is a utility for reading data from a BigQuery query response.
//
// RowIter will iterate over all the results, even if they span more than one
// page of results. It automatically uses page tokens to iterate over all the
// pages to retrieve all results.
type RowIter struct {
	response      *bigquery.GetQueryResultsResponse
	jobId         string
	service       *bigquery.Service
	nextPageToken string
	row           int
}

// poll until the job is complete.
func (r *RowIter) poll() error {
	var queryResponse *bigquery.GetQueryResultsResponse
	for {
		var err error
		queryCall := r.service.Jobs.GetQueryResults("google.com:chrome-skia", r.jobId)
		if r.nextPageToken != "" {
			queryCall.PageToken(r.nextPageToken)
		}
		queryResponse, err = queryCall.Do()
		if err != nil {
			return err
		}
		if queryResponse.JobComplete {
			break
		}
		time.Sleep(time.Second)
	}
	r.nextPageToken = queryResponse.PageToken
	r.response = queryResponse
	return nil
}

// NewRowIter starts a query and returns a RowIter for iterating through the
// results.
func NewRowIter(service *bigquery.Service, query string) (*RowIter, error) {
	job := &bigquery.Job{
		Configuration: &bigquery.JobConfiguration{
			Query: &bigquery.JobConfigurationQuery{
				Query: query,
			},
		},
	}
	jobResponse, err := service.Jobs.Insert("google.com:chrome-skia", job).Do()
	if err != nil {
		return nil, err
	}

	r := &RowIter{
		jobId:   jobResponse.JobReference.JobId,
		service: service,
		row:     -1, // Start at -1 so the first call to Next() puts us at the 0th Row.
	}
	return r, r.poll()
}

// Next moves to the next row in the response and returns true as long as data
// is availble, returning false when the end of the results are reached.
//
// Calling Next() the first time actually points the iterator at the first row,
// which makes it possible to use Next if a for loop:
//
//    for iter.Next() { ... }
//
func (r *RowIter) Next() bool {
	r.row++
	if r.row >= len(r.response.Rows) {
		if r.nextPageToken != "" {
			r.poll()
			r.row = 0
			return len(r.response.Rows) > 0
		} else {
			return false
		}
	}
	return true
}

// DecodeParams pulls all the values in the params record out as a map[string]string.
//
// The schema for each table has a nested record called 'params' that contains
// various axes along which queries could be built, such as the gpu the test was
// run against. Pull out the entire record as a generic map[string]string.
func (r *RowIter) DecodeParams() map[string]string {
	row := r.response.Rows[r.row]
	schema := r.response.Schema
	params := map[string]string{}
	for i, cell := range row.F {
		if cell.V != nil {
			name := schema.Fields[i].Name
			if strings.HasPrefix(name, "params_") {
				params[strings.TrimPrefix(name, "params_")] = cell.V.(string)
			}
		}
	}
	return params
}

// Decode uses struct tags to decode a single row into a struct.
//
// For example, given a struct:
//
//   type A struct {
//     Name string   `bq:"name"`
//     Value float64 `bq:"measurement"`
//   }
//
// And a BigQuery table that contained two columns named "name" and
// "measurement". Then calling Decode as follows would parse the column values
// for "name" and "measurement" and place them in the Name and Value fields
// respectively.
//
//   a = &A{}
//   iter.Decode(a)
//
// Implementation Details:
//
//   If a tag names a column that doesn't exist, the field is merely ignored,
//   i.e. it is left unchanged from when it was passed into Decode.
//
//   Not all columns need to be tagged in the struct.
//
//   The decoder doesn't handle nested structs, only the top level fields are decoded.
//
//   The decoder only handles struct fields of type string, int, int32, int64,
//   float, float32 and float64.
func (r *RowIter) Decode(s interface{}) error {
	row := r.response.Rows[r.row]
	schema := r.response.Schema
	// Collapse the data in the row into a map[string]string.
	rowMap := map[string]string{}
	for i, cell := range row.F {
		if cell.V != nil {
			rowMap[schema.Fields[i].Name] = cell.V.(string)
		}
	}

	// Then iter over the fields of 's' and set them from the row data.
	sv := reflect.ValueOf(s).Elem()
	st := sv.Type()
	for i := 0; i < sv.NumField(); i++ {
		columnName := st.Field(i).Tag.Get("bq")
		if columnValue, ok := rowMap[columnName]; ok {
			switch sv.Field(i).Kind() {
			case reflect.String:
				sv.Field(i).SetString(columnValue)
			case reflect.Float32, reflect.Float64:
				f, err := strconv.ParseFloat(columnValue, 64)
				if err != nil {
					return err
				}
				sv.Field(i).SetFloat(f)
			case reflect.Int32, reflect.Int64:
				parsedInt, err := strconv.ParseInt(columnValue, 10, 64)
				if err != nil {
					return err
				}
				sv.Field(i).SetInt(parsedInt)
			default:
				return fmt.Errorf("Can't decode into field of type: %s %s", columnName, sv.Field(i).Kind())
			}
		}
	}
	return nil
}

// populateTraces reads the measurement data from BigQuery and populates the Traces.
//
// dates is the list of table date suffixes that we will need to iterate over.
// earliestTimestamp is the timestamp of the earliest commit.
func (all *AllData) populateTraces(dates []string, fullData bool) error {
	// Keep a map of key to Trace.
	allTraces := map[string]*Trace{}

	numSamples := len(all.Commits)

	earliestTimestamp := all.Commits[0].CommitTime

	// A mapping of Git hashes to where they appear in the Commits array, also the index
	// at which a measurement gets stored in the Values array.
	hashToIndex := make(map[string]int)
	for i, commit := range all.Commits {
		hashToIndex[commit.Hash] = i
	}

	// Restricted set of traces if we don't want full data.
	restrictedSet := `
      AND (
        params.benchName="tabl_worldjournal.skp"
                   OR
        params.benchName="desk_amazon.skp"
        )
  `

	// Now query the actual samples.
	queryTemplate := `
   SELECT
    *
   FROM
    perf_skps_v2.skpbench%s
   WHERE
     isTrybot=false
     AND (
        params.measurementType="gpu" OR
        params.measurementType="wall"
        )
      AND
        timestamp >= %d
      %s
   ORDER BY
     key DESC,
     timestamp DESC;
	     `
	if fullData {
		restrictedSet = ""
	}
	// Query each table one day at a time. This protects us from schema changes.
	for _, date := range dates {
		query := fmt.Sprintf(queryTemplate, date, earliestTimestamp, restrictedSet)
		iter, err := NewRowIter(all.service, query)
		if err != nil {
			return fmt.Errorf("Failed to query data from BigQuery: %s", err)
		}
		var trace *Trace = nil
		currentKey := ""
		for iter.Next() {
			m := &struct {
				Value float64 `bq:"value"`
				Key   string  `bq:"key"`
				Hash  string  `bq:"gitHash"`
			}{}
			if err := iter.Decode(m); err != nil {
				return fmt.Errorf("Failed to decode Measurement from BigQuery: %s", err)
			}
			if m.Key != currentKey {
				currentKey = m.Key
				// If we haven't seen this key before, create a new Trace for it and store
				// the Trace in allTraces.
				if _, ok := allTraces[currentKey]; !ok {
					trace = NewTrace(numSamples)
					trace.Params = iter.DecodeParams()
					trace.Key = currentKey
					allTraces[currentKey] = trace
				} else {
					trace = allTraces[currentKey]
				}
			}
			if index, ok := hashToIndex[m.Hash]; ok {
				trace.Values[index] = m.Value
			}
		}
		glog.Infof("Loading data, finished day %s", date)
	}
	// Flatten allTraces into all.Traces.
	for _, trace := range allTraces {
		all.Traces = append(all.Traces, trace)
	}

	return nil
}

// Data is the full set of traces for the last N days all parsed into structs.
type Data struct {
	all *AllData
}

// AsJSON serializes the data as JSON.
func (d *Data) AsJSON(w io.Writer) error {
	d.all.mutex.Lock()
	defer d.all.mutex.Unlock()

	// TODO(jcgregorio) Keep a cache of the gzipped JSON around and serve that as long as it's fresh.
	return json.NewEncoder(w).Encode(d.all)
}

// populateParamSet returns the set of all possible values for all the 'params'
// in AllData.
func (all *AllData) populateParamSet() {
	// First pull the data out into a map of sets.
	type ChoiceSet map[string]bool
	c := make(map[string]ChoiceSet)
	for _, t := range all.Traces {
		for k, v := range t.Params {
			if set, ok := c[k]; !ok {
				c[k] = make(map[string]bool)
				c[k][v] = true
			} else {
				set[v] = true
			}
		}
	}
	// Now flatten the sets into []string and populate all.ParamsSet with that.
	for k, v := range c {
		allOptions := []string{}
		for option, _ := range v {
			allOptions = append(allOptions, option)
		}
		all.ParamSet[k] = allOptions
	}
}

func (all *AllData) populate() error {
	all.mutex.Lock()
	defer all.mutex.Unlock()

	dates, commits, err := gitCommitsWithTestData(all.service)
	if err != nil {
		return fmt.Errorf("Failed to read hashes from BigQuery: %s", err)
	}
	glog.Info("Successfully read hashes from BigQuery")

	all.Commits = commits

	if err := all.populateTraces(dates, all.fullData); err != nil {
		return fmt.Errorf("Failed to read traces from BigQuery: %s", err)
	}
	glog.Info("Successfully read traces from BigQuery")

	all.populateParamSet()

	lastSkpUpdate = time.Now()

	return nil
}

// NewData loads the data the first time and then starts a go routine to
// preiodically refresh the data.
//
func NewData(doOauth bool, gitRepoDir string, fullData bool) (*Data, error) {
	var err error
	var client *http.Client
	if doOauth {
		client, err = runFlow(config)
		if err != nil {
			return nil, fmt.Errorf("Failed to auth: %s", err)
		}
	} else {
		client, err = serviceaccount.NewClient(nil)
		if err != nil {
			return nil, fmt.Errorf("Failed to auth using a service account: %s", err)
		}
	}
	service, err := bigquery.New(client)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a new BigQuery service object: %s", err)
	}

	all := &AllData{
		Traces:   make([]*Trace, 0, 0),
		ParamSet: make(map[string]Choices),
		Commits:  make([]*Commit, 0, 0),
		service:  service,
		fullData: fullData,
	}

	if err := all.populate(); err != nil {
		// Fail fast, monit will restart us if we fail for some reason.
		glog.Fatal(err)
	}
	go func() {
		for _ = range time.Tick(REFRESH_PERIOD) {
			if err := all.populate(); err != nil {
				glog.Errorln("Failed to refresh data from BigQuery: ", err)
			}
		}
	}()

	return &Data{all: all}, nil
}
