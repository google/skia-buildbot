// tilebuilder is an application that periodically loads new data from BigQuery
// for quick and easy consumption by the skiaperf UI.
//
//  The algorithm
//  -------------
//
//  Start at level 0.
//  Find the index of the last tile (0000.gob, 0001.gob, etc)
//  nextTile = (last tile)+1 or 0 if no tiles were found.
//  Find the last commit time from the last tile file, or use BEGINNING_OF_TIME if there were no tile files.
//  Get a list of all commits >= the last commit time (exclude the last git hash).
//
//  Start loading data from BigQuery, and for every time you hit (32) points write out a new Tile file
//  and increment nextTile.
//
//  If you get to the end and have <= 32 points left, write it out as nextTile.gob, we will pick it up
//  and try to fill it out the next time through the loop.
//
//  Do the following for each level, starting with 1, and incrementing until no new tiles are generated for a level.
//  {{
//    Find the index of the last tile (0000.gob, 0001.gob, etc)
//    nextTile = (last tile)+1 or 0 if no tiles were found.
//
//    Loop over the 4 subtiles that should make up this new tile and sub-sample to 32 points.
//    Make sure to pick non-missing points if possible. Also make sure to rollup commit messages
//    for all four of the points that get consolidated.
//
//    Write out as a new tile.
//
//    Keep looping until you run out of tiles at the lower level.
//  }}
//
package main

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"net/http"
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
	// TODO(jcgregorio) Move to skia.googlesource.com/...git... or something.
	"auth"
	"bqutil"
	"config"
	"db"
	"filetilestore"
	"types"
)

// flags
var (
	tileDir = flag.String("tile_dir", "/tmp/tileStore", "What directory to look for tiles in.")
	doOauth = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
)

var (
	// BigQuery query as a template.
	traceQueryTemplate *template.Template

	lastTileUpdate      = map[string]time.Time{}
	timeSinceLastUpdate = map[string]metrics.Gauge{}
	updateLatency       = map[string]metrics.Timer{}
)

const (
	SAMPLE_PERIOD = 30 * time.Minute
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
	// Initialize the metrics.
	for _, datasetName := range config.ALL_DATASET_NAMES {
		name := string(datasetName)
		lastTileUpdate[name] = time.Now()
		timeSinceLastUpdate[name] = metrics.NewRegisteredGauge(fmt.Sprintf("build.%s.time_since_last_update", name), metrics.DefaultRegistry)
		updateLatency[name] = metrics.NewRegisteredTimer(fmt.Sprintf("build.%s.latency", name), metrics.DefaultRegistry)
	}

	// Keep the timeSince* metrics up to date.
	go func() {
		for _ = range time.Tick(time.Minute) {

			for _, datasetName := range config.ALL_DATASET_NAMES {
				name := string(datasetName)
				timeSinceLastUpdate[name].Update(int64(time.Since(lastTileUpdate[name]).Seconds()))
			}
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

	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", "skia-monitoring-b:2003")
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "tilepipeline", addr)

	flag.Parse()
}

// startConditions returns the time from which queries should be made, the index
// of the next tile that needs to be written, and an error if any occurred.
func startConditions(store types.TileStore) (config.QuerySince, int, error) {
	// startTime is when to limit queries to in time.
	startTime := config.BEGINNING_OF_TIME

	// nextTile is the index of the next tile to write.
	nextTile := 0

	tile, err := store.Get(0, -1)
	if err != nil {
		return startTime, 0, fmt.Errorf("Failed to read tile looking for start conditions: %s", err)
	}
	if tile != nil {
		// We are always overwriting the last tile until it is full and we start on the next tile.
		nextTile = tile.TileIndex
		if tile.TileIndex > 0 {
			tile, err := store.Get(0, tile.TileIndex-1)
			if err != nil {
				return startTime, 0, fmt.Errorf("Failed to read previous tile looking for start conditions: %s", err)
			}
			// Start querying from the timestamp of the last commit in the last full tile.
			startTime = config.NewQuerySince(time.Unix(tile.Commits[len(tile.Commits)-1].CommitTime, 0))
		}
	}
	return startTime, nextTile, nil
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

// readCommitsFromDB Gets commit information from SQL database and returns a
// slice of *Commit in reverse timestamp order.
//
// TODO(bensong): read in a range of commits instead of the whole history.
func readCommitsFromDB() ([]*types.Commit, error) {
	glog.Infoln("readCommitsFromDB starting")
	sql := fmt.Sprintf(`SELECT
	     ts, githash, gitnumber, author, message
	     FROM githash
	     WHERE ts >= '%s'
	     ORDER BY ts DESC`, config.BEGINNING_OF_TIME.SqlTsColumn())
	s := make([]*types.Commit, 0)
	rows, err := db.DB.Query(sql)
	if err != nil {
		return nil, fmt.Errorf("Failed to query githash table: %s", err)
	} else {
		glog.Infoln("executed query")
	}

	for rows.Next() {
		var ts time.Time
		var githash string
		var gitnumber int64
		var author string
		var message string
		if err := rows.Scan(&ts, &githash, &gitnumber, &author, &message); err != nil {
			glog.Errorf("Commits row scan error: ", err)
			continue
		}
		glog.Infoln("readCommitsFromDB in row")
		commit := types.NewCommit()
		commit.CommitTime = ts.Unix()
		commit.Hash = githash
		commit.GitNumber = gitnumber
		commit.Author = author
		commit.CommitMessage = message
		s = append(s, commit)
	}

	return s, nil
}

// gitCommits returns all the Commits that have associated test data, going
// from now back to 'startTime'.
func gitCommits(service *bigquery.Service, datasetName config.DatasetName, startTime config.QuerySince) (map[string][]string, []*types.Commit, error) {
	dateMap := make(map[string][]string)
	allCommits := make([]*types.Commit, 0)

	commitHistory, err := readCommitsFromDB()
	if err != nil {
		return nil, nil, fmt.Errorf("gitCommits: Did not get the commits history from the database: ", err)
	}

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
	// Loop over table going backward until we hit startTime.
	glog.Info("gitCommits: starting.")
	// historyIdx keeps the current index of commitHistory.
	historyIdx := 0

	totalCommits := 0
	for dates := types.NewDateIter(); dates.Next() && (startTime.Date() <= dates.Date()); {
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
					// TODO(jcgregorio) Truncate the commit messages that go into the tail.
				}
			}
			totalCommits++
			allCommits = append(allCommits, c)
		}
		dateMap[dates.Date()] = gitHashesForDay
		glog.Infof("Finding hashes with data, finished day %s, total commits so far %d", dates.Date(), totalCommits)

	}
	// Now reverse allCommits so that it is oldest first.
	reversedCommits := make([]*types.Commit, len(allCommits), len(allCommits))
	for i, c := range allCommits {
		reversedCommits[len(allCommits)-i-1] = c
	}
	return dateMap, reversedCommits, nil
}

// populateParamSet returns the set of all possible values for all the 'params'
// in Dataset.
func populateParamSet(tile *types.Tile) {
	// First pull the data out into a map of sets.
	type ChoiceSet map[string]bool
	c := make(map[string]ChoiceSet)
	for _, t := range tile.Traces {
		for k, v := range t.Params {
			if set, ok := c[k]; !ok {
				c[k] = make(map[string]bool)
				c[k][v] = true
			} else {
				set[v] = true
			}
		}
	}
	// Now flatten the sets into []string and populate ParamsSet with that.
	for k, v := range c {
		allOptions := []string{}
		for option, _ := range v {
			allOptions = append(allOptions, option)
		}
		tile.ParamSet[k] = allOptions
	}
}

// populateTraces reads the data from BigQuery and populates the Traces.
//
// dates is a map of table date suffixes that we will need to iterate over that
// map to the git hashes that each of those days contain.
func populateTraces(tile *types.Tile, service *bigquery.Service, datasetName config.DatasetName, dates map[string][]string) error {
	// Keep a map of key to Trace.
	allTraces := map[string]*types.Trace{}

	numSamples := len(tile.Commits)

	earliestTimestamp := tile.Commits[0].CommitTime

	// A mapping of Git hashes to where they appear in the Commits array, also
	// the index at which a measurement gets stored in the Values array.
	hashToIndex := make(map[string]int)
	for i, commit := range tile.Commits {
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

	// Query each table one day at a time. This protects us from schema changes
	// and from trying to pull too much data from BigQuery at one time.
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
			iter, err := bqutil.NewRowIter(service, query.String())
			if err != nil {
				return fmt.Errorf("Failed to query data from BigQuery: %s", err)
			}
			var trace *types.Trace = nil
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
						trace = types.NewTrace(numSamples)
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
	// Flatten allTraces into Traces.
	for _, trace := range allTraces {
		tile.Traces = append(tile.Traces, trace)
	}

	return nil
}

// buildTile builds a Tile for the given set of commits.
//
// dates is the full set of days and hashes that appear in each day.
func buildTile(service *bigquery.Service, datasetName config.DatasetName, dates map[string][]string, commits []*types.Commit) (*types.Tile, error) {
	tile := types.NewTile()
	tile.Commits = commits

	// We need to filter down 'dates' to just include the hashes that appear in 'commits'.
	// First build a map of the hashes in 'commits'.
	commitHashes := map[string]bool{}
	for _, c := range commits {
		commitHashes[c.Hash] = true
	}
	filteredDates := map[string][]string{}
	for date, gitHashes := range dates {
		filteredHashes := []string{}
		for _, h := range gitHashes {
			if _, ok := commitHashes[h]; ok {
				filteredHashes = append(filteredHashes, h)
			}
		}
		if len(filteredHashes) > 0 {
			filteredDates[date] = filteredHashes
		}
	}

	glog.Infof("Building tile from: %#v\n", filteredDates)
	if err := populateTraces(tile, service, datasetName, filteredDates); err != nil {
		return nil, fmt.Errorf("Failed to read traces from BigQuery: %s", err)
	}
	glog.Info("Successfully read traces from BigQuery")
	populateParamSet(tile)

	return tile, nil
}

func updateAllTileSets(service *bigquery.Service) {
	glog.Infof("Starting to update all tile sets.")
	for _, datasetName := range config.ALL_DATASET_NAMES {
		glog.Infof("Starting to update tileset %s.", string(datasetName))
		begin := time.Now()

		store := filetilestore.NewFileTileStore(*tileDir, string(datasetName))

		startTime, nextTile, err := startConditions(store)
		glog.Infoln("Found startTime", startTime, "nextTile", nextTile)
		if err != nil {
			glog.Errorf("Failed to compute start conditions for dataset %s: %s", string(datasetName), err)
			continue
		}

		glog.Infoln("Getting commits")
		dates, commits, err := gitCommits(service, datasetName, startTime)
		glog.Infoln("Found commits", commits)
		if err != nil {
			glog.Errorf("Failed to read commits for dataset %s: %s", string(datasetName), err)
			continue
		}
		glog.Infof("Found %d new commits across %d days for dataset %s", len(commits), len(dates), string(datasetName))
		for i := 0; i < len(commits); i += config.TILE_SIZE {
			end := i + config.TILE_SIZE
			if end > len(commits) {
				end = len(commits)
			}
			tile, err := buildTile(service, datasetName, dates, commits[i:end])
			if err != nil {
				glog.Errorf("Failed to write tile: %d scale: 0: %s\n", nextTile, err)
				break
			}
			tile.Scale = 0
			tile.TileIndex = nextTile
			if err := store.Put(0, nextTile, tile); err != nil {
				glog.Errorf("Failed to write tile %s for dataset %s: %s", nextTile, string(datasetName), err)
				break
			}
			glog.Infof("Write tile: %d scale: %d\n", tile.TileIndex, tile.Scale)
			nextTile += 1
		}
		// TODO(jcgregorio) Now write out new tiles for scales 1,2,etc. Also make
		// sure to merge intermediate commits, but summarize the commit message.
		name := string(datasetName)
		lastTileUpdate[name] = time.Now()
		d := time.Since(begin)
		updateLatency[name].Update(d)
		glog.Infof("Finished loading Tile data for dataset %s from BigQuery in %f s", name, d.Seconds())
	}
}

func main() {
	var err error
	var client *http.Client
	if *doOauth {
		client, err = auth.RunFlow()
		if err != nil {
			glog.Fatalf("Failed to auth: %s", err)
		}
	} else {
		client, err = serviceaccount.NewClient(nil)
		if err != nil {
			glog.Fatalf("Failed to auth using a service account: %s", err)
		}
	}
	service, err := bigquery.New(client)
	if err != nil {
		glog.Fatalf("Failed to create a new BigQuery service object: %s", err)
	}

	updateAllTileSets(service)
	for _ = range time.Tick(SAMPLE_PERIOD) {
		updateAllTileSets(service)
	}
}
