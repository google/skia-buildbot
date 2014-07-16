package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

import (
	"code.google.com/p/goauth2/compute/serviceaccount"
	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/google-api-go-client/bigquery/v2"
	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/golang/glog"
	"github.com/oxtoacart/webbrowser"

	"github.com/rcrowley/go-metrics"
)

var (
	bqOauthConfig = &oauth.Config{
		ClientId:     "470362608618-nlbqngfl87f4b3mhqqe9ojgaoe11vrld.apps.googleusercontent.com",
		ClientSecret: "J4YCkfMXFJISGyuBuVEiH60T",
		Scope: bigquery.BigqueryInsertdataScope + " " +
			bigquery.BigqueryScope,
		AuthURL:     "https://accounts.google.com/o/oauth2/auth",
		TokenURL:    "https://accounts.google.com/o/oauth2/token",
		RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
		TokenCache:  oauth.CacheFile("ingestbqtoken.data"),
	}

	datasetsWithMetrics         = []string{"perf_skps_gotest", "perf_bench_gotest"}
	bytesAddedGauge             = map[string]metrics.Gauge{}
	bytesSuccessfullyAddedGauge = map[string]metrics.Gauge{}
	ingestionLatency            metrics.Timer
)

const (
	_BQ_PROJECT_NAME   = "google.com:chrome-skia"
	BEGINNING_OF_TIME  = 1401840000
	_CS_PROJECT_BUCKET = "chromium-skia-gm"
)

func Init() {
	for _, dataset := range datasetsWithMetrics {
		bytesAddedGauge[dataset] = metrics.NewRegisteredGauge(fmt.Sprintf("ingest.%s.bytes_in", dataset), metrics.DefaultRegistry)
		bytesSuccessfullyAddedGauge[dataset] = metrics.NewRegisteredGauge(fmt.Sprintf("ingest.%s.bytes_out", dataset), metrics.DefaultRegistry)
	}

	ingestionLatency = metrics.NewRegisteredTimer("ingest.time_elapsed", metrics.DefaultRegistry)

	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", "jcgregorio.cnc:2003")
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "ingester", addr)
}

// authBigQuery authenticates and returns a usable bigquery Service object.
func authBigQuery(useOauth bool) (*bigquery.Service, error) {
	if useOauth {
		glog.Infoln("Using OAuth for BQ authentication")
		t := &oauth.Transport{Config: bqOauthConfig}
		_, err := bqOauthConfig.TokenCache.Token()
		if err != nil {
			// Reflow
			url := bqOauthConfig.AuthCodeURL("dunno what goes here")
			webbrowser.Open(url)
			fmt.Println("Please enter the code you get from the webpage that just opened: ")
			var code string
			fmt.Scan(&code)
			if _, err := t.Exchange(code); err != nil {
				return nil, err
			}
		}
		return bigquery.New(t.Client())
	} else {
		glog.Infoln("Using service account for BQ authentication")
		client, err := serviceaccount.NewClient(nil)
		if err != nil {
			return nil, fmt.Errorf("Unable to auth using service account: %s", err)
		}
		return bigquery.New(client)
	}
}

// getValidTables returns a slice of table names it found
// in the dataset in BigQuery.
func getValidTables(service *bigquery.Service, datasetName string) ([]string, error) {
	req := service.Tables.List(_BQ_PROJECT_NAME, datasetName)
	tables := make([]string, 0)

	for req != nil {
		curResp, err := req.Do()
		if err == nil {
			for _, table := range curResp.Tables {
				tables = append(tables, table.TableReference.TableId)
			}
			if len(curResp.NextPageToken) > 0 {
				req.PageToken(curResp.NextPageToken)
			} else {
				req = nil
			}
		} else {
			return nil, fmt.Errorf("Failed to get valid tables: %s", err)
		}
	}
	return tables, nil
}

const (
	BACKTRACK_TABLES = 5
)

// getLatestBQTimestamp returns the last time the dataset was updated, as well
// as it is capable of determining.
func getLatestBQTimestamp(service *bigquery.Service, datasetName string, tablePrefix string) int64 {
	result, err := service.Datasets.Get(_BQ_PROJECT_NAME, datasetName).Do()
	if err != nil {
		return BEGINNING_OF_TIME
	}
	results := result.LastModifiedTime / 1000
	tables, err := getValidTables(service, datasetName)
	if err != nil {
		glog.Warningf("Error in retrieving tables: %s\n", err)
		return BEGINNING_OF_TIME
	}
	for _, table := range tables {
		if strings.TrimPrefix(table, tablePrefix) != table {
			tables = append(tables, table)
		}
	}
	if len(tables) <= 0 || err != nil {
		glog.Infoln("No tables detected, using earliest possible timestamp")
		return BEGINNING_OF_TIME
	}
	sort.Strings(tables)
	// Check the last five tables, as these are likely to be among the last
	// tables updated
	if len(tables) > BACKTRACK_TABLES {
		tables = tables[len(tables)-BACKTRACK_TABLES:]
	}
	glog.Infoln("Querying tables for lastModifiedTime")
	for _, table := range tables {
		result, err := service.Tables.Get(_BQ_PROJECT_NAME, datasetName, table).Do()
		if err != nil {
			glog.Errorf("Failed to retrieve table: %s\n", err)
			return BEGINNING_OF_TIME
		}
		if result.LastModifiedTime/1000 > results {
			results = result.LastModifiedTime / 1000
		}
	}
	return results
}

// getStorageService returns a Cloud Storage service.
func getStorageService() (*storage.Service, error) {
	return storage.New(http.DefaultClient)
}

type GSObject struct {
	name string
	day  time.Time
}

// parseTimestamp returns the timestamp stored in the url
func parseTimestamp(url string) (int64, error) {
	dirParts := strings.Split(url, "/")
	fileName := dirParts[len(dirParts)-1]
	splitName := strings.Split(fileName, "_")
	lastPart := splitName[len(splitName)-1]
	timestampStr := strings.Split(lastPart, ".")[0]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return -1, err
	} else {
		return timestamp, nil
	}
}

// roundDate converts a timestamp to a date, rounding to the nearest day.
func roundDate(timestamp int64) time.Time {
	d := time.Unix(timestamp, 0)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

// getFilesFromGSDir returns a list of GSObjects representing the URI of each
// file in that directory that was added after the given timestamp.
func getFilesFromGSDir(service *storage.Service, directory string, bucket string, lowestTimestamp int64) []*GSObject {
	results := make([]*GSObject, 0)
	glog.Infoln("Opening directory", directory, "of bucket", bucket)
	req := service.Objects.List(bucket).Prefix(directory)

	for req != nil {
		resp, err := req.Do()
		if err != nil {
			glog.Errorln("Error occurred while getting files: ", err)
			break
		}
		for _, result := range resp.Items {
			updateDate, _ := time.Parse(time.RFC3339, result.Updated)
			updateTimestamp := updateDate.Unix()
			if updateTimestamp > lowestTimestamp {
				newParsedTime, err := parseTimestamp(result.Name)
				if err != nil {
					glog.Errorf("Failed to parse timestamp for %s: %s\n", result.Name, err)
					continue
				}
				results = append(results,
					&GSObject{
						name: "gs://" + bucket + "/" + result.Name,
						day:  roundDate(newParsedTime),
					})
			}
		}
		if len(resp.NextPageToken) > 0 {
			req.PageToken(resp.NextPageToken)
		} else {
			req = nil
		}
	}
	return results
}

// getLatestGSDirs gets the appropriate directory names in which data
// would be stored between the given timestamp and now.
func getLatestGSDirs(timestamp int64, bsSubdir string) []string {
	oldTime := time.Unix(timestamp, 0).UTC()
	glog.Infoln("Old time: ", oldTime)
	newTime := time.Now().UTC()
	lastAddedTime := oldTime
	results := make([]string, 0)
	newYear, newMonth, newDay := newTime.Date()
	newHour := newTime.Hour()
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

// getSchema returns the schema dictionary for the given dataset
func getSchema(name string) (*bigquery.TableSchema, error) {

	rawByteString, err := ioutil.ReadFile(name + ".json")
	if err != nil {
		return nil, fmt.Errorf("Unable to open schema: %s", err)
	}
	result := new(bigquery.TableSchema)
	err = json.Unmarshal(rawByteString, result)
	return result, err
}

type Job struct {
	jobId string
	name  string
}

// smartCopy loads the new files from Cloud Storage into the BigQuery, returning
// a list of load jobs that it initated in the process.
func smartCopy(bq *bigquery.Service, cs *storage.Service, jobs []Job, dataset, prefix, sourceBucketSubdir string, timestamp int64) (int, []Job, error) {
	glog.Infoln("Start of smart copy, subdir = ", sourceBucketSubdir)
	glog.Infoln("Finding timestamp...")
	var realTimestamp int64
	if timestamp > 0 {
		realTimestamp = timestamp
	} else {
		realTimestamp = getLatestBQTimestamp(bq, dataset, prefix)
	}
	glog.Infoln("Using timestamp ", realTimestamp)
	// Get all the JSON files with a timestamp after that
	dirs := getLatestGSDirs(realTimestamp, sourceBucketSubdir)
	glog.Infoln("Looking in dirs: ", dirs)
	tables, _ := getValidTables(bq, dataset)
	glog.Infoln("Found existing tables: ", tables)

	toDateSuffix := func(day time.Time) string {
		return fmt.Sprintf("%04d%02d%02d", day.Year(), day.Month(), day.Day())
	}

	jsonUris := make(map[string][]string)
	for _, dir := range dirs {
		files := getFilesFromGSDir(cs, dir, _CS_PROJECT_BUCKET, realTimestamp)
		for _, file := range files {
			dateSuffix := toDateSuffix(file.day)
			if _, ok := jsonUris[dateSuffix]; !ok {
				jsonUris[dateSuffix] = make([]string, 0, 1)
			}
			glog.Infoln("Adding", file.name, "to insert list for", dateSuffix)
			jsonUris[dateSuffix] = append(jsonUris[dateSuffix], file.name)
		}
	}

	isValidTable := func(maybeTable string) bool {
		for _, name := range tables {
			if name == maybeTable {
				return true
			}
		}
		return false
	}
	count := 0
	for suffix, uris := range jsonUris {
		if !isValidTable(prefix + suffix) {
			// Insert a blank table
			glog.Infoln("Making new table", prefix+suffix)
			tableSchema, err := getSchema(prefix)
			if err != nil {
				return count, jobs, fmt.Errorf("Unable to retrieve schema: %s", err)
			}
			if !*logOnly {
				resp, err := bq.Tables.Insert(_BQ_PROJECT_NAME, dataset,
					&bigquery.Table{
						Schema: tableSchema,
						TableReference: &bigquery.TableReference{
							DatasetId: dataset,
							ProjectId: _BQ_PROJECT_NAME,
							TableId:   prefix + suffix,
						},
					}).Do()
				glog.Infoln(resp)
				if err != nil {
					return count, jobs, fmt.Errorf("Table insertion unsuccesful: %s", err)
				}
			}
		}
		glog.Infoln("Inserting ", uris, "into ", prefix+suffix)
		if *logOnly {
			continue
		}
		resp, err := bq.Jobs.Insert(_BQ_PROJECT_NAME, &bigquery.Job{
			Configuration: &bigquery.JobConfiguration{
				Load: &bigquery.JobConfigurationLoad{
					DestinationTable: &bigquery.TableReference{
						DatasetId: dataset,
						ProjectId: _BQ_PROJECT_NAME,
						TableId:   prefix + suffix,
					},
					MaxBadRecords: 1000000000,
					SourceFormat:  "NEWLINE_DELIMITED_JSON",
					SourceUris:    uris,
				},
			},
		}).Do()
		if err != nil {
			return count, jobs, fmt.Errorf("Error occurred while inserting data: %s", err)
		}
		jobs = append(jobs, Job{
			jobId: resp.JobReference.JobId,
			name:  prefix + suffix,
		})
		count += len(uris)
	}
	return count, jobs, nil
}

// runAndRepeat calls smartCopy repeatedly to make sure it uploads all the
// data to BigQuery by having it check where files may have been uploaded while
// smartCopy was running.
func runAndRepeat(bq *bigquery.Service, cs *storage.Service, jobs []Job, dataset, prefix, sourceBucketSubdir string) (int, []Job) {
	newtimestamp := time.Now().Unix()
	timestamp := int64(-1)
	totalSuccess := 0
	numSuccess, jobs, err := smartCopy(bq, cs, jobs, dataset, prefix, sourceBucketSubdir, timestamp)
	if err != nil {
		glog.Warningf("Error occurred during smartCopy: %s\n", err)
	}
	totalSuccess += numSuccess
	for numSuccess > 0 {
		timestamp, newtimestamp = newtimestamp, time.Now().Unix()
		numSuccess, jobs, err = smartCopy(bq, cs, jobs, dataset, prefix, sourceBucketSubdir, timestamp)
		if err != nil {
			glog.Warningf("Error occurred during smartCopy: %s\n", err)
		}
		totalSuccess += numSuccess
	}
	return totalSuccess, jobs
}

// isFinished determines whether a job is in the the finished array slice.
func isFinished(job *Job, finished []*Job) bool {
	for _, finishedJob := range finished {
		if job == finishedJob {
			return true
		}
	}
	return false
}

// getDatasetName extracts the dataset name from a job, returning nil if it can't find it.
func getDatasetName(job *bigquery.Job) string {
	if job.Configuration == nil {
		return ""
	}
	if job.Configuration.Load == nil {
		return ""
	}
	// Is this one required?
	if job.Configuration.Load.DestinationTable == nil {
		return ""
	}
	return job.Configuration.Load.DestinationTable.DatasetId
}

// isMetricked determines whether a job has an associated gauge to update.
func isMetricked(job *bigquery.Job) bool {
	jobDataset := getDatasetName(job)
	for _, datasetName := range datasetsWithMetrics {
		if datasetName == jobDataset {
			return true
		}
	}
	return false
}

// waitForJobs blocks until all the BigQuery jobs return as completed, and prints
// out each one's statuses.
func waitForJobs(bq *bigquery.Service, jobs []Job) {
	finished := make([]*Job, 0)
	begin := time.Now()
	bytesAdded := make(map[string]int64)
	bytesSuccess := make(map[string]int64)

	for _, dataset := range datasetsWithMetrics {
		bytesAdded[dataset] = 0
		bytesSuccess[dataset] = 0
	}

	glog.Infof("Waiting on %d jobs.", len(jobs))

	for len(finished) < len(jobs) && time.Since(begin) < 10*time.Minute {
		for i, _ := range jobs {
			if !isFinished(&jobs[i], finished) {
				resp, err := bq.Jobs.Get(_BQ_PROJECT_NAME, jobs[i].jobId).Do()
				if err != nil {
					glog.Errorln("Error while polling job:", err)
					continue
				}
				if resp.Status.State == "DONE" {
					glog.Infoln("Job", jobs[i].name, "finished")
					formatOut, _ := json.Marshal(resp)
					glog.Infoln(string(formatOut))
					finished = append(finished, &jobs[i])

					if isMetricked(resp) {
						datasetName := getDatasetName(resp)
						bytesAdded[datasetName] += resp.Statistics.Load.InputFileBytes
						bytesSuccess[datasetName] += resp.Statistics.Load.OutputBytes
					}
				}
			}
		}
		time.Sleep(15 * time.Second)
	}

	for _, dataset := range datasetsWithMetrics {
		glog.Infoln("Jobs for ", dataset)
		glog.Infoln("bytes in: ", bytesAdded[dataset])
		glog.Infoln("bytes out: ", bytesSuccess[dataset])
		bytesAddedGauge[dataset].Update(bytesAdded[dataset])
		bytesSuccessfullyAddedGauge[dataset].Update(bytesSuccess[dataset])
	}
}

type CustomRequest struct {
	// Cloud Storage directory to pull from
	SourceCSDirectory string
	// Target BigQuery dataset
	DestinationBQDataset string
	// Target BigQuery table
	DestinationBQTable string
	// Schema to use
	Schema string
	// Timestamp; set to -1 to use default
	Timestamp int64
	// Whether to run once or run until all new files have been added
	OneShot bool
}

type CommitInfo struct {
	// Name of the builder to add
	BuilderName string
	// Name of the commit to add
	BuildCommit string
	// Time of the build
	BuildTime time.Time
}

type IngestService struct {
	useOAuth bool
}

func NewIngestService(useOAuth bool) *IngestService {
	return &IngestService{useOAuth: useOAuth}
}

// CustomUpdate runs a custom job using the details it is passed on the
// ingest service
func (is *IngestService) CustomUpdate(request *CustomRequest) {
	glog.Infoln("Running custom job:")
	glog.Infoln("Loading from", request.SourceCSDirectory)
	glog.Infoln("Uploading to", request.DestinationBQTable, "in", request.DestinationBQDataset)
	glog.Infoln("Using schema", request.Schema)
	glog.Infoln("Using timestamp", request.Timestamp)
	if request.OneShot {
		glog.Infoln("One shot job")
	}

	bq, err := authBigQuery(is.useOAuth)
	if err != nil {
		glog.Errorln("Unable to get BigQuery service:", err)
		glog.Errorln("Ending request early")
		return
	}
	cs, err := getStorageService()
	if err != nil {
		glog.Errorln("Unable to get Cloud Storage service:", err)
		glog.Errorln("Ending request early")
		return
	}
	if request.OneShot {
		numSuccesses, jobs, err := smartCopy(
			bq,
			cs,
			make([]Job, 0),
			request.DestinationBQDataset,
			request.DestinationBQTable,
			request.SourceCSDirectory,
			request.Timestamp)
		if err != nil {
			glog.Errorln("Error occurred during run: ", err)
		}
		glog.Infoln(numSuccesses, "datasets submitted")
		waitForJobs(bq, jobs)
	} else {
		numSuccesses, jobs := runAndRepeat(
			bq,
			cs,
			make([]Job, 0),
			request.DestinationBQDataset,
			request.DestinationBQTable,
			request.SourceCSDirectory)
		glog.Infoln(numSuccesses, "datasets submitted")
		waitForJobs(bq, jobs)
	}
}

// NormalUpdate runs a normal ingest update, looking in the default Cloud Service
// directories for new files, and uploading them to the default BigQuery datasets
func (is *IngestService) NormalUpdate() error {
	glog.Infoln("ingest: Update request received")
	begin := time.Now()
	bq, err := authBigQuery(is.useOAuth)
	if err != nil {
		return fmt.Errorf("Unable to get BigQuery service: %s", err)
	}
	cs, err := getStorageService()
	if err != nil {
		return fmt.Errorf("Unable to get Cloud Storage service: %s", err)
	}
	jobs := make([]Job, 0)
	// TODO: Move these strings into config
	microSuccess, jobs := runAndRepeat(bq, cs, jobs, "perf_bench_gotest", "microbench", "stats-json-v2")
	skpSuccess, jobs := runAndRepeat(bq, cs, jobs, "perf_skps_gotest", "skpbench", "pics-json-v2")
	glog.Infoln("Waiting for jobs to finish..")
	waitForJobs(bq, jobs)
	success := microSuccess + skpSuccess
	glog.Infoln(success, "datasets possibly added")
	glog.Infoln("ingest: request completed")

	timeElapsed := time.Since(begin)
	ingestionLatency.Update(timeElapsed)
	return nil
}
