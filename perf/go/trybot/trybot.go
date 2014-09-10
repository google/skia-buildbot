/** trybot is for loading and serving trybot performance results.

  Implementation notes:

    Regular files are in:

    gs://chromium-skia-gm/nano-json-v1/2014/08/07/01/Perf-...-Release/nanobench_da7a94...8d35a_1407357280.json

    while trybots are in:

    gs://chromium-skia-gm/trybot/nano-json-v1/2014/08/07/05/Perf-...-Release-Trybot/85/448043002/nanobench_da7a944...d35a_1407357280.json

    Note the 'trybot' dir prefix and the addition of the build number and codereview issue number in the directory.
    The Rietveld issue id appears before the file name.  Note that some of the tries aren't associated with an issue, we will ignore those.


    Notes: I tried using both GOB and JSON as the serialization format and got the following numbers:

       GOB:  40s 12MB  49s (second run)
       JSON: 43s 14MB  43s (second run)

    Since there isn't a material difference between the two let's go with JSON
    as that's a little easier to debug.
*/

package trybot

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"

	"code.google.com/p/google-api-go-client/storage/v1"

	"skia.googlesource.com/buildbot.git/perf/go/db"
	"skia.googlesource.com/buildbot.git/perf/go/ingester"
	"skia.googlesource.com/buildbot.git/perf/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/util"
)

var (
	// nameRegex is the regexp that a trybot filename must match. This enforces the need for a Rietveld issue number.
	//
	// REPL here: http://play.golang.org/p/uGmexyFxEr
	nameRegex = regexp.MustCompile(`trybot/nano-json-v1/\d{4}/\d{2}/\d{2}/\d{2}/[^/]+/\d+/(\d+)/(.*)`)

	st *storage.Service = nil

	elapsedTimePerUpdate metrics.Timer
	metricsProcessed     metrics.Counter
	numSuccessUpdates    metrics.Counter
)

// Write the TryBotResults to the datastore.
func Write(issue string, try *types.TryBotResults) error {
	b, err := json.Marshal(try)
	if err != nil {
		return fmt.Errorf("Failed to encode to JSON: %s", err)
	}
	glog.Infof("Writing: %s", issue)
	_, err = db.DB.Exec("REPLACE INTO tries (issue, results, lastUpdated) VALUES (?, ?, ?)", issue, b, time.Now())
	if err != nil {
		return fmt.Errorf("Failed to write to database: %s", err)
	}
	return nil
}

// Get the TryBotResults from the datastore.
func Get(issue string) (*types.TryBotResults, error) {
	var results string
	err := db.DB.QueryRow("SELECT results FROM tries WHERE issue=?", issue).Scan(&results)
	if err == sql.ErrNoRows {
		return types.NewTryBotResults(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to load try data with id %s: %s", issue, err)
	}
	try := &types.TryBotResults{}
	if err := json.Unmarshal([]byte(results), try); err != nil {
		return nil, fmt.Errorf("Failed to decode try data with id: %s", issue)
	}
	return try, nil
}

// List returns the last N Rietveld issue IDs.
func List(n int) ([]string, error) {
	rows, err := db.DB.Query("SELECT issue FROM tries ORDER BY lastUpdated DESC LIMIT ?", n)
	if err != nil {
		return nil, fmt.Errorf("Failed to read try data from database: %s", err)
	}
	defer rows.Close()

	ret := []string{}
	for rows.Next() {
		var issue string
		if err := rows.Scan(&issue); err != nil {
			return nil, fmt.Errorf("List: Failed to read issus from row: %s", err)
		}
		ret = append(ret, issue)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))
	return ret, nil
}

// TileWithTryData will add all the trybot data for the given issue to the
// given Tile. A new Tile that is a copy of the original Tile will be returned,
// so we aren't modifying the underlying Tile.
func TileWithTryData(tile *types.Tile, issue string) (*types.Tile, error) {
	ret := tile.Copy()
	lastCommitIndex := tile.LastCommitIndex()
	// The way we handle Tiles there is always empty space at the end of the
	// Tile of index -1. Use that space to inject the trybot results.
	ret.Commits[lastCommitIndex+1].CommitTime = time.Now().Unix()
	lastCommitIndex = tile.LastCommitIndex()

	tryResults, err := Get(issue)
	if err != nil {
		return nil, fmt.Errorf("AppendToTile: Failed to retreive trybot results: %s", err)
	}
	// Copy in the trybot data.
	for k, v := range tryResults.Values {
		if tr, ok := ret.Traces[k]; !ok {
			continue
		} else {
			tr.Values[lastCommitIndex] = v
		}
	}
	return ret, nil
}

// addTryData copies the data from the BenchFile into the TryBotResults.
func addTryData(res *types.TryBotResults, b *ingester.BenchFile) {
	glog.Infof("addTryData: %s", b.Name)
	benchData, err := b.FetchAndParse()
	if err != nil {
		// Don't fall over for a single corrupt file.
		return
	}

	keyPrefix := benchData.KeyPrefix()
	for testName, allConfigs := range benchData.Results {
		for configName, result := range *allConfigs {
			key := fmt.Sprintf("%s:%s:%s", keyPrefix, testName, configName)
			res.Values[key] = result.Min
			metricsProcessed.Inc(1)
		}
	}
}

// BenchByIssue allows sorting BenchFile's by the Rietveld issue id.
//
// We sort on issue id so that we aren't doing excessive writes to the
// database.
type BenchByIssue struct {
	BenchFile *ingester.BenchFile
	IssueName string
}

type BenchByIssueSlice []*BenchByIssue

func (p BenchByIssueSlice) Len() int           { return len(p) }
func (p BenchByIssueSlice) Less(i, j int) bool { return p[i].IssueName < p[j].IssueName }
func (p BenchByIssueSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func Init() {
	db.Init()
	var err error
	st, err = storage.New(util.NewTimeoutClient())
	if err != nil {
		panic("Can't construct HTTP client")
	}
	elapsedTimePerUpdate = metrics.NewRegisteredTimer("ingester.trybot.nano.update", metrics.DefaultRegistry)
	metricsProcessed = metrics.NewRegisteredCounter("ingester.trybot.nano.processed", metrics.DefaultRegistry)
	numSuccessUpdates = metrics.NewRegisteredCounter("ingester.trybot.nano.updates", metrics.DefaultRegistry)
}

// Udpate does a single round of ingestion of trybot data of all the data that
// has appeared since lastIngestTime.
func Update(lastIngestTime int64) error {
	begin := time.Now()
	glog.Infof("Starting to query Google Storage for new files.")
	benchFiles, err := ingester.GetBenchFiles(lastIngestTime, st, "trybot/nano-json-v1")
	if err != nil {
		return fmt.Errorf("Failed to get trybot files: %s", err)
	}

	benchFilesByIssue := []*BenchByIssue{}
	for _, b := range benchFiles {
		match := nameRegex.FindStringSubmatch(b.Name)
		if match != nil {
			issue := match[1]
			benchFilesByIssue = append(benchFilesByIssue, &BenchByIssue{
				BenchFile: b,
				IssueName: issue,
			})
		}
	}
	// Resort by issue id.
	sort.Sort(BenchByIssueSlice(benchFilesByIssue))

	lastIssue := ""
	var cur *types.TryBotResults = nil
	for _, b := range benchFilesByIssue {
		if b.IssueName != lastIssue {
			// Write out the current TryBotResults to the datastore and create a fresh new TryBotResults.
			if cur != nil {
				if err := Write(lastIssue, cur); err != nil {
					return fmt.Errorf("Update failed to write trybot results: %s", err)
				}
			}
			if cur, err = Get(b.IssueName); err != nil {
				return fmt.Errorf("Failed to load existing trybot data for issue %s: %s", b.IssueName, err)
				continue
			}
			lastIssue = b.IssueName
			glog.Infof("Switched to issue: %s", lastIssue)
		}
		addTryData(cur, b.BenchFile)
	}
	if cur != nil {
		if err := Write(lastIssue, cur); err != nil {
			return fmt.Errorf("Update failed to write trybot results: %s", err)
		}
	}
	numSuccessUpdates.Inc(1)
	elapsedTimePerUpdate.UpdateSince(begin)
	glog.Infof("Finished trybot ingestion.")

	return nil
}
