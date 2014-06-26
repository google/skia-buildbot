// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
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

import (
	"config"
	"ctrace"
	"kmeans"
)

const (
	NUM_SAMPLE_TRACES_PER_CLUSTER = 10

	// K is the k in k-means.
	K = 100

	KMEANS_ITERATIONS = 10

	// TIMEOUT is the http timeout when making BigQuery requests.
	TIMEOUT = time.Duration(time.Minute)
)

// Shouldn't need auth when running from GCE, but will need it for local dev.
// TODO(jcgregorio) Move to reading this from client_secrets.json and void these keys at that point.
var (
	oauthConfig = &oauth.Config{
		ClientId:     "470362608618-nlbqngfl87f4b3mhqqe9ojgaoe11vrld.apps.googleusercontent.com",
		ClientSecret: "J4YCkfMXFJISGyuBuVEiH60T",
		Scope:        bigquery.BigqueryScope,
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		TokenCache:   oauth.CacheFile("bqtoken.data"),
	}

	// TODO(jcgregorio) Fix metrics so that skps and microbenches are reported separately.
	lastSkpUpdate             = time.Now()
	timeSinceLastSkpUpdate    = metrics.NewRegisteredGauge("data.bigquery.skps.refresh.time_since_last_update", metrics.DefaultRegistry)
	skpUpdateLatency          = metrics.NewRegisteredTimer("data.bigquery.skps.refresh.latency", metrics.DefaultRegistry)
	clusterCalculationLatency = metrics.NewRegisteredTimer("cluster.skps.latency", metrics.DefaultRegistry)

	// BigQuery query as a template.
	traceQueryTemplate *template.Template
)

// TraceQuery is the data to pass into traceQueryTemplate when expanding the template.
type TraceQuery struct {
	TablePrefix          string
	Date                 string
	DatasetPredicates    string
	AdditionalPredicates string
	GitHash              string
	Timestamp            int64
}

func init() {
	go func() {
		for _ = range time.Tick(time.Minute) {
			timeSinceLastSkpUpdate.Update(int64(time.Since(lastSkpUpdate).Seconds()))
		}
	}()

	traceQueryTemplate = template.Must(template.New("traceQueryTemplate").Parse(`
   SELECT
    *
   FROM
   {{.TablePrefix}}{{.Date}}
   WHERE
     isTrybot=false
     AND (gitHash = {{GitHash}})")
     {{.DatasetPredicates}}
      AND
        timestamp >= {{.Timestamp}}
     {{.AdditionalPredicates}}
   ORDER BY
     key DESC,
     timestamp DESC;
	     `))
}

// dialTimeout is a dialer that sets a timeout.
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, TIMEOUT)
}

// runFlow runs through a 3LO OAuth 2.0 flow to get credentials for BigQuery.
func runFlow(config *oauth.Config) (*http.Client, error) {
	transport := &oauth.Transport{
		Config: config,
		Transport: &http.Transport{
			Dial: dialTimeout,
		},
	}
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
		t.Values[i] = config.MISSING_DATA_SENTINEL
	}
	return t
}

// Annotations for commits.
//
// Will map to the table of annotation notes in MySQL. See DESIGN.md
// for the MySQL schema.
type Annotation struct {
	ID     int    `json:"id"    db:"id"`
	Notes  string `json:"notes" db:"notes"`
	Author string `json:"author db:"author"`
	Type   int    `json:"type"  db:"type"`
}

// Commit is information about each Git commit.
type Commit struct {
	CommitTime    int64        `json:"commit_time" bq:"timestamp" db:"ts"`
	Hash          string       `json:"hash"        bq:"gitHash"   db:"githash"`
	GitNumber     int64        `json:"git_number"  bq:"gitNumber" db:"gitnumber"`
	CommitMessage string       `json:"commit_msg"                 db:"message"`
	Annotations   []Annotation `json:"annotations,omitempty"`
}

func NewCommit() *Commit {
	return &Commit{
		Annotations: []Annotation{},
	}
}

// readCommitsFromDB Gets commit information from SQL database.
// Returns map[Hash]->*Commit
// TODO(bensong): read in a range of commits instead of the whole history.
func readCommitsFromDB() (map[string]*Commit, error) {
	m := make(map[string]*Commit)
	rows, err := db.Query("SELECT ts, githash, gitnumber, message FROM githash")
	if err != nil {
		return m, fmt.Errorf("Failed to query githash table: %s", err)
	}

	for rows.Next() {
		var ts time.Time
		var githash string
		var gitnumber int64
		var message string
		if err := rows.Scan(&ts, &githash, &gitnumber, &message); err != nil {
			glog.Infoln("Commits row scan error: ", err)
			continue
		}
		commit := NewCommit()
		commit.CommitTime = ts.Unix()
		commit.Hash = githash
		commit.GitNumber = gitnumber
		commit.CommitMessage = message
		m[githash] = commit
	}

	// Gets annotations and puts them into corresponding commit struct.
	rows, err = db.Query(`SELECT
	    githashnotes.githash, githashnotes.id,
	    notes.type, notes.author, notes.notes
	    FROM githashnotes
	    INNER JOIN notes
	    ON githashnotes.id=notes.id
	    ORDER BY githashnotes.id`)
	if err != nil {
		return m, fmt.Errorf("Failed to read annotations: %s", err)
	}

	for rows.Next() {
		var githash string
		var id int
		var notetype int
		var author string
		var notes string
		if err := rows.Scan(&githash, &id, &notetype, &author, &notes); err != nil {
			glog.Infoln("Annotations row scan error: ", err)
			continue
		}
		if _, ok := m[githash]; ok {
			annotation := Annotation{id, notes, author, notetype}
			m[githash].Annotations = append(m[githash].Annotations, annotation)
		}
	}

	return m, nil
}

// ValueWeight is a weight proportional to the number of times the parameter
// Value appears in a cluster. Used in ClusterSummary.
type ValueWeight struct {
	Value  string
	Weight int
}

// ClusterSummary is a summary of a single cluster of traces.
type ClusterSummary struct {
	// Traces contains at most NUM_SAMPLE_TRACES_PER_CLUSTER sample traces, the first is the centroid.
	Traces [][][]float64

	// Keys of all the members of the Cluster.
	Keys []string

	// ParamSummaries is a summary of all the parameters in the cluster.
	ParamSummaries [][]ValueWeight
}

// ClusterSummaries is one summary for each cluster that the k-means clustering
// found.
type ClusterSummaries struct {
	Clusters []*ClusterSummary
}

// Choices is a list of possible values for a param. See Dataset.
type Choices []string

// Dataset is the top level struct we return via JSON to the UI.
//
// The length of the Commits array is the same length as all of the Values
// arrays in all of the Traces.
type Dataset struct {
	Traces           []*Trace           `json:"traces"`
	ParamSet         map[string]Choices `json:"param_set"`
	Commits          []*Commit          `json:"commits"`
	service          *bigquery.Service
	fullData         bool
	clusterSummaries *ClusterSummaries
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
	return i.Date() != config.BEGINNING_OF_TIME
}

// Date returns the day formatted as we use them on BigQuery table name suffixes.
func (i *DateIter) Date() string {
	return i.day.Format("20060102")
}

func tablePrefixFromDatasetName(name config.DatasetName) string {
	switch name {
	case config.DATASET_SKP:
		return "perf_skps_v2.skpbench"
	case config.DATASET_MICRO:
		return "perf_bench_v2.microbench"
	}
	return "perf_skps_v2.skpbench"
}

// gitCommitsWithTestData returns the list of commits that have perf data
// associated with them. Populates commit info from commitMap if possible.
//
// Returns a map of [dates of tables we had to query] to the list of git hashes
// that appear in those tables, and a list of commits ordered from oldest
// to newest.
//
// Not all commits will have perf data, the builders don't necessarily run for
// each commit.  Will limit itself to returning only the number of days that
// are needed to get MAX_COMMITS_IN_MEMORY.
func gitCommitsWithTestData(datasetName config.DatasetName, service *bigquery.Service, commitMap map[string]*Commit) (map[string][]string, []*Commit, error) {
	dateMap := make(map[string][]string)
	allCommits := make([]*Commit, 0)
	queryTemplate := `
SELECT
  gitHash, FIRST(timestamp) as timestamp, FIRST(gitNumber) as gitNumber
FROM
  %s%s
GROUP BY
  gitHash
ORDER BY
  timestamp DESC;
  `
	// Loop over table going backward until we find 30 commits, or hit the BEGINNING_OF_TIME.
	glog.Info("gitCommitsWithTestData: starting.")
	dates := NewDateIter()
	totalCommits := 0
	for dates.Next() {
		query := fmt.Sprintf(queryTemplate, tablePrefixFromDatasetName(datasetName), dates.Date())
		iter, err := NewRowIter(service, query)
		if err != nil {
			glog.Warningln("Tried to query a table that didn't exist", dates.Date(), err)
			continue
		}

		gitHashesForDay := []string{}
		for iter.Next() {
			c := NewCommit()
			if err := iter.Decode(c); err != nil {
				return nil, allCommits, fmt.Errorf("Failed reading hashes from BigQuery: %s", err)
			}
			gitHashesForDay = append(gitHashesForDay, c.Hash)
			// Populate commit info if available.
			if val, ok := commitMap[c.Hash]; ok {
				*c = *val
			}
			totalCommits++
			if len(allCommits) < config.MAX_COMMITS_IN_MEMORY {
				allCommits = append(allCommits, c)
			} else {
				break
			}
		}
		dateMap[dates.Date()] = gitHashesForDay
		glog.Infof("Finding hashes with data, finished day %s, total commits so far %d", dates.Date(), totalCommits)
		if totalCommits >= config.MAX_COMMITS_IN_MEMORY {
			break
		}
	}
	// Now reverse allCommits so that it is oldest first.
	reversedCommits := make([]*Commit, len(allCommits), len(allCommits))
	for i, c := range allCommits {
		reversedCommits[len(allCommits)-i-1] = c
	}
	return dateMap, reversedCommits, nil
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
// dates is a maps of table date suffixes that we will need to iterate over that map to the githashes
// that each of those days contain.
// earliestTimestamp is the timestamp of the earliest commit.
func (all *Dataset) populateTraces(datasetName config.DatasetName, dates map[string][]string, fullData bool) error {
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
	additionalPredicates := `
      AND (
        params.benchName="tabl_worldjournal.skp"
                   OR
        params.benchName="desk_amazon.skp"
        )
  `

	if datasetName == config.DATASET_MICRO {
		additionalPredicates = `
      AND (
        params.testName="draw_stroke_rrect_miter_640_480"
                   OR
        params.testName="shadermask_BW_FF_640_480"
        )
  `
	}
	if fullData {
		additionalPredicates = ""
	}
	datasetPredicates := `
     AND (
        params.measurementType="gpu" OR
        params.measurementType="wall"
        )
  `
	if datasetName == config.DATASET_MICRO {
		datasetPredicates = ""
	}

	// Query each table one day at a time. This protects us from schema changes.
	for date, gitHashes := range dates {
		for _, gitHash := range gitHashes {
			traceQueryParams := TraceQuery{
				TablePrefix:          tablePrefixFromDatasetName(datasetName),
				Date:                 date,
				DatasetPredicates:    datasetPredicates,
				AdditionalPredicates: additionalPredicates,
				GitHash:              gitHash,
				Timestamp:            earliestTimestamp,
			}
			query := &bytes.Buffer{}
			err := traceQueryTemplate.Execute(query, traceQueryParams)
			if err != nil {
				return fmt.Errorf("Failed to construct a query: %s", err)
			}
			glog.Infof("Query: %q", query)
			iter, err := NewRowIter(all.service, query.String())
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
	mutex sync.Mutex
	data  map[config.DatasetName]*Dataset
}

// allDataFromName returns the dataset for the given name. If no match is found
// it returns the SKP dataset.
func (d *Data) allDataFromName(name config.DatasetName) *Dataset {
	if data, ok := d.data[name]; ok {
		return data
	} else {
		return d.data[config.DATASET_SKP]
	}
}

func (d *Data) ClusterSummaries(name config.DatasetName) *ClusterSummaries {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.allDataFromName(name).clusterSummaries

}

// AsJSON serializes the data as JSON.
func (d *Data) AsJSON(name config.DatasetName, w io.Writer) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// TODO(jcgregorio) Keep a cache of the gzipped JSON around and serve that as long as it's fresh.
	return json.NewEncoder(w).Encode(d.allDataFromName(name))
}

// populateParamSet returns the set of all possible values for all the 'params'
// in Dataset.
func (all *Dataset) populateParamSet() {
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

// chooseK chooses a random sample of k observations. Used as the starting
// point for the k-means clustering.
func chooseK(observations []kmeans.Clusterable, k int) []kmeans.Clusterable {
	popN := len(observations)
	centroids := make([]kmeans.Clusterable, k)
	for i := 0; i < k; i++ {
		o := observations[rand.Intn(popN)].(*ctrace.ClusterableTrace)
		cp := &ctrace.ClusterableTrace{
			Key:    "I'm a centroid",
			Values: make([]float64, len(o.Values)),
		}
		copy(cp.Values, o.Values)
		centroids[i] = cp
	}
	return centroids
}

// traceToFlot converts the data into a format acceptable to the Flot plotting
// library.
//
// Flot expects data formatted as an array of [x, y] pairs.
func traceToFlot(t *ctrace.ClusterableTrace) [][]float64 {
	ret := make([][]float64, len(t.Values))
	for i, x := range t.Values {
		ret[i] = []float64{float64(i), x}
	}
	return ret
}

// blacklist of a list of Params that we don't want showing up in the
// clustering word cloud. Should need to go away as we clean up the data
// coming into BigQuery.
//
// The bool value doesn't matter, merely appearing in the map is enough to be
// blacklisted.
var blacklist = map[string]bool{
	"scale":         true,
	"mode":          true,
	"role":          true,
	"skpSize":       true,
	"viewport":      true,
	"configuration": true,
}

// ValueWeightSortable is a utility class for sorting the ValueWeight's by Weight.
type ValueWeightSortable []ValueWeight

func (p ValueWeightSortable) Len() int           { return len(p) }
func (p ValueWeightSortable) Less(i, j int) bool { return p[i].Weight > p[j].Weight } // Descending.
func (p ValueWeightSortable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// getParamSummaries summaries all the parameters for all observations in a cluster.
//
// The return value is an array of []ValueWeight's, one []ValueWeight per
// parameter. The members of each []ValueWeight are sorted by the Weight, with
// higher Weight's first.
func getParamSummaries(cluster []kmeans.Clusterable) [][]ValueWeight {
	// For each cluster member increment each parameters count.
	type ValueMap map[string]int
	counts := map[string]ValueMap{}
	clusterSize := float64(len(cluster))
	// First figure out what parameters and values appear in the cluster.
	for _, o := range cluster {
		for k, v := range o.(*ctrace.ClusterableTrace).Params {
			if _, ok := blacklist[k]; ok {
				continue
			}
			if v == "" {
				continue
			}
			if _, ok := counts[k]; !ok {
				counts[k] = ValueMap{}
				counts[k][v] = 0
			}
			counts[k][v] += 1
		}
	}
	// Now calculate the weights for each parameter value.  The weight of each
	// value is proportional to the number of times it appears on an observation
	// versus all other values for the same parameter.
	ret := make([][]ValueWeight, 0)
	for _, count := range counts {
		weights := []ValueWeight{}
		for value, weight := range count {
			weights = append(weights, ValueWeight{
				Value:  value,
				Weight: int(14*float64(weight)/clusterSize) + 12,
			})
		}
		sort.Sort(ValueWeightSortable(weights))
		ret = append(ret, weights)
	}

	return ret
}

// GetClusterSummaries returns a summaries for each cluster.
func (all *Dataset) GetClusterSummaries(observations, centroids []kmeans.Clusterable) *ClusterSummaries {
	ret := &ClusterSummaries{
		Clusters: make([]*ClusterSummary, len(centroids)),
	}
	allClusters, _ := kmeans.GetClusters(observations, centroids)
	for i, cluster := range allClusters {
		// cluster is just an array of the observations for a given cluster.
		numSampleTraces := len(cluster)
		if numSampleTraces > NUM_SAMPLE_TRACES_PER_CLUSTER {
			numSampleTraces = NUM_SAMPLE_TRACES_PER_CLUSTER
		}
		summary := &ClusterSummary{
			Keys:           make([]string, len(cluster)),
			Traces:         make([][][]float64, numSampleTraces),
			ParamSummaries: getParamSummaries(cluster),
		}
		for j, o := range cluster {
			summary.Keys[j] = o.(*ctrace.ClusterableTrace).Key
		}
		for j := 0; j < numSampleTraces; j++ {
			summary.Traces[j] = traceToFlot(cluster[j].(*ctrace.ClusterableTrace))
		}
		ret.Clusters[i] = summary
	}

	return ret
}

// populateClusters runs k-means clustering over the trace shapes and returns
// the clustering of those shapes via all.clusterSummaries.
func (all *Dataset) populateClusters() {
	begin := time.Now()
	observations := make([]kmeans.Clusterable, len(all.Traces))
	for i, t := range all.Traces {
		observations[i] = ctrace.NewFullTrace(t.Key, t.Values, t.Params)
	}

	// Create K starting centroids.
	centroids := chooseK(observations, K)
	for i := 0; i < KMEANS_ITERATIONS; i++ {
		centroids = kmeans.Do(observations, centroids, ctrace.CalculateCentroid)
		glog.Infof("Total Error: %f\n", kmeans.TotalError(observations, centroids))
	}
	all.clusterSummaries = all.GetClusterSummaries(observations, centroids)
	d := time.Since(begin)
	clusterCalculationLatency.Update(d)
	glog.Infof("Finished anomaly detection in %f s", d.Seconds())
}

// populate populates the Dataset struct with info from BigQuery.
func (all *Dataset) populate(datasetName config.DatasetName, commitMap map[string]*Commit) error {
	begin := time.Now()
	dates, commits, err := gitCommitsWithTestData(datasetName, all.service, commitMap)
	if err != nil {
		return fmt.Errorf("Failed to read hashes from BigQuery: %s", err)
	}
	glog.Info("Successfully read hashes from BigQuery")

	all.Commits = commits

	if err := all.populateTraces(datasetName, dates, all.fullData); err != nil {
		return fmt.Errorf("Failed to read traces from BigQuery: %s", err)
	}
	glog.Info("Successfully read traces from BigQuery")

	all.populateParamSet()

	all.populateClusters()

	lastSkpUpdate = time.Now()
	d := time.Since(begin)
	skpUpdateLatency.Update(d)
	glog.Infof("Finished loading skp data from BigQuery in %f s", d.Seconds())

	return nil
}

// NewDataset returns an new Dataset object ready to be filled with data via populate().
func NewDataset(service *bigquery.Service, fullData bool) *Dataset {
	return &Dataset{
		Traces:   make([]*Trace, 0),
		ParamSet: make(map[string]Choices),
		Commits:  make([]*Commit, 0),
		service:  service,
		fullData: fullData,
	}
}

// NewData loads the data the first time and then starts a go routine to
// preiodically refresh the data.
func NewData(doOauth bool, gitRepoDir string, fullData bool) (*Data, error) {
	var err error
	var client *http.Client
	if doOauth {
		client, err = runFlow(oauthConfig)
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
	d := &Data{
		data: make(map[config.DatasetName]*Dataset),
	}

	// Get a map of commit records
	commitMap, err := readCommitsFromDB()
	if err != nil {
		glog.Warningln("Did not get commit map: ", err)
	}

	for _, name := range config.ALL_DATASET_NAMES {
		data := NewDataset(service, fullData)
		if err := data.populate(name, commitMap); err != nil {
			glog.Fatal(err)
		}
		d.data[name] = data
	}

	go func() {
		for _ = range time.Tick(config.REFRESH_PERIOD) {
			// Get the latest map of commit records
			commitMap, err := readCommitsFromDB()
			if err != nil {
				glog.Warningln("Did not get new commit map: ", err)
			}

			for _, name := range config.ALL_DATASET_NAMES {
				data := NewDataset(service, fullData)
				if err := data.populate(name, commitMap); err != nil {
					glog.Errorln("Failed to refresh skp data from BigQuery: ", err)
					break
				}
				d.mutex.Lock()
				d.data[name] = data
				d.mutex.Unlock()
			}
		}
	}()

	return d, nil
}
