// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path"
	"sort"
	"sync"
	"text/template"
	"time"
)

import (
	"code.google.com/p/goauth2/compute/serviceaccount"
	"code.google.com/p/google-api-go-client/bigquery/v2"
	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"
)

import (
	"auth"
	"bqutil"
	"config"
	"ctrace"
	"db"
	"kmeans"
	"types"
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
	TablePrefix       string
	Date              string
	DatasetPredicates string
	GitHash           string
	Timestamp         int64
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
     AND (gitHash = "{{.GitHash}}")
     {{.DatasetPredicates}}
      AND
        timestamp >= {{.Timestamp}}
   ORDER BY
     key DESC,
     timestamp DESC;
	     `))
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

// ValueWeight is a weight proportional to the number of times the parameter
// Value appears in a cluster. Used in ClusterSummary.
type ValueWeight struct {
	Value  string
	Weight int
}

// StepFit stores information on the best Step Function fit on a trace.
// Deviation is the Least Absolute Deviation divided by the step size.
// TurningPoint is the point index from where the Step Function changes value.
type StepFit struct {
	Deviation float64
	StepSize  float64
}

// ClusterSummary is a summary of a single cluster of traces.
type ClusterSummary struct {
	// Traces contains at most NUM_SAMPLE_TRACES_PER_CLUSTER sample traces, the first is the centroid.
	Traces [][][]float64

	// Keys of all the members of the Cluster.
	Keys []string

	// ParamSummaries is a summary of all the parameters in the cluster.
	ParamSummaries [][]ValueWeight

	// StepFit is info on the best Step Function fit of the centroid.
	StepFit StepFit
}

// ClusterSummaries is one summary for each cluster that the k-means clustering
// found.
type ClusterSummaries struct {
	Clusters         []*ClusterSummary
	StdDevThreshhold float64
	K                int
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
	Commits          []*types.Commit          `json:"commits"`
	service          *bigquery.Service
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
	return i.Date() != config.BEGINNING_OF_TIME.BqTableSuffix()
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

// gitCommits returns lists of commits that have perf data associated with them.
// Populates commit info and tail_commits from commitHistory if possible.
//
// Returns a map of [dates of tables we had to query] to the list of git hashes
// that appear in those tables, and a list of commits with data ordered from
// oldest to newest.
//
// Not all commits will have perf data, the builders don't necessarily run for
// each commit.  Will limit itself to returning only the number of days that
// are needed to get MAX_COMMITS_IN_MEMORY.
func gitCommits(datasetName config.DatasetName, service *bigquery.Service, commitHistory []*types.Commit) (map[string][]string, []*types.Commit, error) {
	dateMap := make(map[string][]string)
	allCommits := make([]*types.Commit, 0)
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
	// Loop over table going backward until we find MAX_COMMITS_IN_MEMORY+1 commits, or hit the BEGINNING_OF_TIME.
	glog.Info("gitCommits: starting.")
	// historyIdx keeps the current index of commitHistory.
	historyIdx := 0
	dates := NewDateIter()
	totalCommits := 0
	for dates.Next() {
		query := fmt.Sprintf(queryTemplate, tablePrefixFromDatasetName(datasetName), dates.Date())
		iter, err := bqutil.NewRowIter(service, query)
		if err != nil {
			glog.Warningln("Tried to query a table that didn't exist", dates.Date(), err)
			continue
		}

		gitHashesForDay := []string{}
		for iter.Next() {
			c := types.NewCommit()
			if err := iter.Decode(c); err != nil {
				return nil, allCommits, fmt.Errorf("Failed reading hashes from BigQuery: %s", err)
			}
			gitHashesForDay = append(gitHashesForDay, c.Hash)
			// Scan commitHistory and populate commit info if available.
			for ; historyIdx < len(commitHistory) && commitHistory[historyIdx].CommitTime >= c.CommitTime; historyIdx++ {
				if commitHistory[historyIdx].Hash == c.Hash {
					*c = *commitHistory[historyIdx]
				} else if len(allCommits) > 0 {
					// Append to tail_commit
					tailCommits := &allCommits[len(allCommits)-1].TailCommits
					*tailCommits = append(*tailCommits, commitHistory[historyIdx])
				}
			}
			totalCommits++
			if len(allCommits) <= config.MAX_COMMITS_IN_MEMORY {
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
	// Removes the extra allCommits if applicable.
	if len(allCommits) == config.MAX_COMMITS_IN_MEMORY {
		allCommits = allCommits[:len(allCommits)-1]
	}
	// Now reverse allCommits so that it is oldest first.
	reversedCommits := make([]*types.Commit, len(allCommits), len(allCommits))
	for i, c := range allCommits {
		reversedCommits[len(allCommits)-i-1] = c
	}
	return dateMap, reversedCommits, nil
}

// populateTraces reads the measurement data from BigQuery and populates the Traces.
//
// dates is a maps of table date suffixes that we will need to iterate over that map to the githashes
// that each of those days contain.
// earliestTimestamp is the timestamp of the earliest commit.
func (all *Dataset) populateTraces(datasetName config.DatasetName, dates map[string][]string) error {
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
				TablePrefix:       tablePrefixFromDatasetName(datasetName),
				Date:              date,
				DatasetPredicates: datasetPredicates,
				GitHash:           gitHash,
				Timestamp:         earliestTimestamp,
			}
			query := &bytes.Buffer{}
			err := traceQueryTemplate.Execute(query, traceQueryParams)
			if err != nil {
				return fmt.Errorf("Failed to construct a query: %s", err)
			}
			glog.Infof("Query: %q", query)
			iter, err := bqutil.NewRowIter(all.service, query.String())
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

func (d *Data) ClusterSummariesFor(name config.DatasetName, k int, stddevThreshhold float64) *ClusterSummaries {
	return d.allDataFromName(name).calculateClusterSummaries(k, stddevThreshhold)
}

// AsGzippedJSON returns the Dataset as gzipped JSON.
//
// The Dataset is already available in the form of a gzipped JSON file on disk,
// so use that instead of re-serializing and gzipping it again here.
func (Data) AsGzippedJSON(tileDir string, name config.DatasetName, w io.Writer) error {
	filename := path.Join(tileDir, string(name)+".gz")
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Couldn't open zipped tile for reading: %s", err)
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("Failed writing tile back to the UI: %s", err)
	}
	return nil
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

// average calculates and returns the average value of the given []float64.
func average(xs[]float64)float64 {
	total := 0.0
	for _,v := range xs {
		total += v
	}
	return total / float64(len(xs))
}

// sse calculates and returns the sum squared error from the given base of []float64.
func sse(xs[]float64, base float64)float64 {
	total := 0.0
	for _,v := range xs {
		total += math.Pow(v - base, 2)
	}
	return total
}

// getStepFit takes one []float64 trace and calculates and returns its StepFit.
func getStepFit(trace []float64) StepFit {
	deviation := math.MaxFloat64
	stepSize := -1.0
	for i := range trace {
		if i == 0 {
			continue
		}
		y0 := average(trace[:i])
		y1 := average(trace[i:])
		if y0 == y1 {
			continue
		}
		d := math.Sqrt(sse(trace[:i], y0) + sse(trace[i:], y1)) / float64(len(trace))
		if d < deviation {
			deviation = d
			stepSize = math.Abs(y0 - y1)
		}
	}
	return StepFit{deviation, stepSize}
}

// GetClusterSummaries returns a summaries for each cluster.
func GetClusterSummaries(observations, centroids []kmeans.Clusterable) *ClusterSummaries {
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
			// Try fit on the centroid.
			StepFit:        getStepFit(cluster[0].(*ctrace.ClusterableTrace).Values),
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

// calculateClusterSummaries runs k-means clustering over the trace shapes.
func (all *Dataset) calculateClusterSummaries(k int, stddevThreshhold float64) *ClusterSummaries {
	observations := make([]kmeans.Clusterable, len(all.Traces))
	for i, t := range all.Traces {
		observations[i] = ctrace.NewFullTrace(t.Key, t.Values, t.Params, stddevThreshhold)
	}

	// Create K starting centroids.
	centroids := chooseK(observations, k)
	// TODO(jcgregorio) Keep iterating until the total error stops changing.
	for i := 0; i < KMEANS_ITERATIONS; i++ {
		centroids = kmeans.Do(observations, centroids, ctrace.CalculateCentroid)
		glog.Infof("Total Error: %f\n", kmeans.TotalError(observations, centroids))
	}
	clusterSummaries := GetClusterSummaries(observations, centroids)
	clusterSummaries.K = k
	clusterSummaries.StdDevThreshhold = stddevThreshhold
	return clusterSummaries
}

// populateClusters runs k-means clustering over the trace shapes and returns
// the clustering of those shapes via all.clusterSummaries.
func (all *Dataset) populateClusters() {
	begin := time.Now()
	all.clusterSummaries = all.calculateClusterSummaries(K, ctrace.MIN_STDDEV)
	d := time.Since(begin)
	clusterCalculationLatency.Update(d)
	glog.Infof("Finished clustering in %f s", d.Seconds())
}

// populate populates the Dataset struct with info from BigQuery or the tileDir.
//
// Data will only be loaded from tileDir if firstLoad is true. Data will always
// be written back to tileDir after loading from BigQuery.
func (all *Dataset) populate(datasetName config.DatasetName, commitHistory []*types.Commit, tileDir string, firstLoad bool) error {
	begin := time.Now()
	dates, commits, err := gitCommits(datasetName, all.service, commitHistory)
	if err != nil {
		return fmt.Errorf("Failed to read hashes from BigQuery: %s", err)
	}
	glog.Info("Successfully read hashes from BigQuery")
	all.Commits = commits

	filename := path.Join(tileDir, string(datasetName)+".gz")
	_, err = os.Stat(filename)
	fileExists := !os.IsNotExist(err)
	if firstLoad && fileExists {
		f, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("Couldn't open saved dataset: %s", err)
		}
		defer f.Close()

		r, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("Failed reading gzipped tile data: %s", err)
		}
		defer r.Close()

		if err := json.NewDecoder(r).Decode(all); err != nil {
			return fmt.Errorf("Error decoding saved dataset: %s", err)
		}
		glog.Info("Successfully read traces from disk.")
	} else {
		if err := all.populateTraces(datasetName, dates); err != nil {
			return fmt.Errorf("Failed to read traces from BigQuery: %s", err)
		}
		glog.Info("Successfully read traces from BigQuery")
		all.populateParamSet()

		os.MkdirAll(tileDir, 0755)

		f, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("Couldn't create file for writing dataset: %s", err)
		}
		defer f.Close()

		w := gzip.NewWriter(f)
		defer w.Close()

		if err := json.NewEncoder(w).Encode(all); err != nil {
			return fmt.Errorf("Couldn't write dataset: %s", err)
		}
	}

	all.populateClusters()

	lastSkpUpdate = time.Now()
	d := time.Since(begin)
	skpUpdateLatency.Update(d)
	glog.Infof("Finished loading skp data from BigQuery in %f s", d.Seconds())

	return nil
}

// NewDataset returns an new Dataset object ready to be filled with data via populate().
func NewDataset(service *bigquery.Service) *Dataset {
	return &Dataset{
		Traces:   make([]*Trace, 0),
		ParamSet: make(map[string]Choices),
		Commits:  make([]*types.Commit, 0),
		service:  service,
	}
}

// NewData loads the data the first time and then starts a go routine to
// preiodically refresh the data.
//
// If there are existing gzip files in tileDir then those will be used instead
// of trying to load data from BigQuery at startup. Regardless of where the
// data comes from at startup, after each time data is pulled from BigQuery the
// data will be written to tileDir.
func NewData(doOauth bool, gitRepoDir string, tileDir string) (*Data, error) {
	var err error
	var client *http.Client
	if doOauth {
		client, err = auth.RunFlow()
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
	commitHistory, err := db.ReadCommitsFromDB()
	if err != nil {
		glog.Warningln("Did not get commit slice: ", err)
	}

	for _, name := range config.ALL_DATASET_NAMES {
		data := NewDataset(service)
		if err := data.populate(name, commitHistory, tileDir, true); err != nil {
			glog.Fatal(err)
		}
		d.data[name] = data
	}

	go func() {
		for _ = range time.Tick(config.REFRESH_PERIOD) {
			// Get the latest map of commit records
			commitHistory, err := db.ReadCommitsFromDB()
			if err != nil {
				glog.Warningln("Did not get new commit slice: ", err)
			}

			for _, name := range config.ALL_DATASET_NAMES {
				data := NewDataset(service)
				if err := data.populate(name, commitHistory, tileDir, false); err != nil {
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
