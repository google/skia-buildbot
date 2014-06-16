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
	"time"
)

import (
	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/google-api-go-client/bigquery/v2"
	"github.com/oxtoacart/webbrowser"
)

const (
	// JSON doesn't support NaN or +/- Inf, so we need a valid float
	// to signal missing data that also has a compact JSON representation.
	MISSING_DATA_SENTINEL = 1e100
)

// Shouldn't need auth when running from GCE, but will need it for local dev.
// TODO(jcgregorio) Move to reading this from client_secrets.json and void these keys at that point.
var config = &oauth.Config{
	ClientId:     "470362608618-nlbqngfl87f4b3mhqqe9ojgaoe11vrld.apps.googleusercontent.com",
	ClientSecret: "J4YCkfMXFJISGyuBuVEiH60T",
	Scope:        bigquery.BigqueryScope,
	AuthURL:      "https://accounts.google.com/o/oauth2/auth",
	TokenURL:     "https://accounts.google.com/o/oauth2/token",
	RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
	TokenCache:   oauth.CacheFile("bqtoken.data"),
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
	CommitTime    time.Time    `json:"commit_time"`
	Hash          string       `json:"hash"`
	GitNumber     int          `json:"git_number"`
	CommitMessage string       `json:"commit_msg"`
	Annotations   []Annotation `json:"annotations,omitempty"`
}

// Choices is a list of possible values for a param. See AllData.
type Choices []string

// AllData is the top level struct we return via JSON to the UI.
//
// The length of the Commits array is the same length as all of the Values
// arrays in all of the Traces.
type AllData struct {
	Traces   []Trace            `json:"traces"`
	ParamSet map[string]Choices `json:"param_set"`
	Commits  []Commit           `json:"commits"`
}

// gitCommitsWithTestData returns the list of commits that have perf data
// associated with them.
//
// Not all commits will have perf data, the builders don't necessarily run for
// each commit.
func gitCommitsWithTestData(service *bigquery.Service) (map[string]bool, error) {
	query := `
SELECT
  gitHash
FROM
  (TABLE_DATE_RANGE(perf_skps_v2.skpbench,
      DATE_ADD(CURRENT_TIMESTAMP(),
        -2,
        'DAY'),
      CURRENT_TIMESTAMP()))
GROUP BY
  gitHash;
  `
	iter, err := NewRowIter(service, query)
	if err != nil {
		return nil, fmt.Errorf("Failed to query for the Git hashes used: %s", err)
	}

	hashes := make(map[string]bool)
	for iter.Next() {
		h := &struct {
			Hash string `bq:"gitHash"`
		}{}
		err := iter.Decode(h)
		if err != nil {
			return nil, fmt.Errorf("Failed reading hashes from BigQuery: %s", err)
		}
		hashes[h.Hash] = true
	}
	return hashes, nil
}

// GitHash represents information on a single Git commit.
type GitHash struct {
	Hash      string
	TimeStamp time.Time
}

// readCommitsFromGit reads the commit history from a Git repository.
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
				return fmt.Errorf("can't decode into field of type: %s", sv.Field(i).Kind())
			}
		}
	}
	return nil
}

// populateTraces reads the measurement data from BigQuery and populates the Traces.
func populateTraces(service *bigquery.Service, all *AllData, hashToIndex map[string]int, numSamples int) error {
	type Measurement struct {
		Value float64 `bq:"value"`
		Key   string  `bq:"key"`
		Hash  string  `bq:"gitHash"`
	}

	// Now query the actual samples.
	query := `
	   SELECT
	     *
	   FROM
	     (TABLE_DATE_RANGE(perf_skps_v2.skpbench,
	         DATE_ADD(CURRENT_TIMESTAMP(),
	           -2,
	           'DAY'),
	         CURRENT_TIMESTAMP()))
     WHERE
       params.benchName="tabl_worldjournal.skp"
           OR
       params.benchName="desk_amazon.skp"
	   ORDER BY
	     key DESC,
	     timestamp DESC;
	     `
	iter, err := NewRowIter(service, query)
	if err != nil {
		return fmt.Errorf("Failed to query data from BigQuery: %s", err)
	}
	var trace *Trace = nil
	currentKey := ""
	for iter.Next() {
		m := &Measurement{}
		if err := iter.Decode(m); err != nil {
			return fmt.Errorf("Failed to decode Measurement from BigQuery: %s", err)
		}
		if m.Key != currentKey {
			if trace != nil {
				all.Traces = append(all.Traces, *trace)
			}
			currentKey = m.Key
			trace = NewTrace(numSamples)
			trace.Params = iter.DecodeParams()
			trace.Key = m.Key
		}
		if index, ok := hashToIndex[m.Hash]; ok {
			trace.Values[index] = m.Value
		}
	}
	all.Traces = append(all.Traces, *trace)

	return nil
}

// Data is the full set of traces for the last N days all parsed into structs.
type Data struct {
	all *AllData
}

// AsJSON serializes the data as JSON.
func (d *Data) AsJSON(w io.Writer) error {
	// TODO(jcgregorio) Keep a cache of the gzipped JSON around and serve that as long as it's fresh.
	return json.NewEncoder(w).Encode(d.all)
}

// populateParamSet returns the set of all possible values for all the 'params'
// in AllData.
func populateParamSet(all *AllData) {
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

// NewData loads the data the first time and then starts a go routine to
// preiodically refresh the data.
//
// TODO(jcgregorio) Actuall do the bit where we start a go routine.
func NewData(doOauth bool, gitRepoDir string) (*Data, error) {
	var err error
	var client *http.Client
	if doOauth {
		client, err = runFlow(config)
		if err != nil {
			return nil, fmt.Errorf("Failed to auth: %s", err)
		}
	} else {
		client = http.DefaultClient
	}
	service, err := bigquery.New(client)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a new BigQuery service object: %s", err)
	}

	// First query and get the list of hashes we are interested in and use that
	// and the git log results to fill in the Commits.
	allGitHashes, err := readCommitsFromGit(gitRepoDir)
	if err != nil {
		return nil, fmt.Errorf("Failed to read hashes from Git log: %s", err)
	}

	hashesTested, err := gitCommitsWithTestData(service)
	if err != nil {
		return nil, fmt.Errorf("Failed to read hashes from BigQuery: %s", err)
	}

	// Order the git hashes by commit log order.
	commits := make([]Commit, 0, len(hashesTested))
	for i := len(allGitHashes) - 1; i >= 0; i-- {
		h := allGitHashes[i]
		if _, ok := hashesTested[h.Hash]; ok {
			commits = append(commits, Commit{Hash: h.Hash, CommitTime: h.TimeStamp})
		}
	}

	// The number of samples that appear in each trace.
	numSamples := len(commits)

	// A mapping of Git hashes to where they appear in the Commits array, also the index
	// at which a measurement gets stored in the Values array.
	hashToIndex := make(map[string]int)
	for i, commit := range commits {
		hashToIndex[commit.Hash] = i
	}

	all := &AllData{
		Traces:   make([]Trace, 0, 0),
		ParamSet: make(map[string]Choices),
		Commits:  commits,
	}

	if err := populateTraces(service, all, hashToIndex, numSamples); err != nil {
		// Fail fast, monit will restart us if we fail for some reason.
		panic(err)
	}

	populateParamSet(all)

	return &Data{all: all}, nil
}
