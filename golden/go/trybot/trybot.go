// trybot is for ingesting Gold trybot results.
package trybot

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/ingester"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/goldingester"
)

// Init registers trybot ingester. The supplied database connection is where
// trybot results are stored.
func Init(vdb *database.VersionedDB) {
	ingester.Register(config.CONSTRUCTOR_GOLD_TRYBOT, func() ingester.ResultIngester {
		return NewTrybotResultIngester(NewTrybotResultStorage(vdb))
	})
}

type TrybotResultStorage struct {
	vdb *database.VersionedDB
}

func NewTrybotResultStorage(vdb *database.VersionedDB) *TrybotResultStorage {
	return &TrybotResultStorage{
		vdb: vdb,
	}
}

// Write writes trybot results to the SQL database connected to vdb that was the
// the argument to Init(..).
func (t *TrybotResultStorage) Write(issue string, trybotResults *TryBotResults) error {
	trybotResults.indexDigests()

	b, err := json.Marshal(trybotResults)
	if err != nil {
		return fmt.Errorf("Failed to encode to JSON: %s", err)
	}

	// Find the latest timestamp in the data.
	var timeStamp int64 = 0
	for _, entry := range trybotResults.Bots {
		if entry.TS > timeStamp {
			timeStamp = entry.TS
		}
	}

	_, err = t.vdb.DB.Exec("REPLACE INTO tries (issue, results, last_updated) VALUES (?, ?, ?)", issue, b, timeStamp)
	if err != nil {
		return fmt.Errorf("Failed to write trybot data to database: %s", err)
	}
	return nil
}

// Get returns the trybot results for the given issue from the datastore.
func (t *TrybotResultStorage) Get(issue string) (*TryBotResults, error) {
	var results string
	err := t.vdb.DB.QueryRow("SELECT results FROM tries WHERE issue=?", issue).Scan(&results)
	if err == sql.ErrNoRows {
		return NewTryBotResults(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to load try data with id %s: %s", issue, err)
	}

	try := &TryBotResults{}
	if err := json.Unmarshal([]byte(results), try); err != nil {
		return nil, fmt.Errorf("Failed to decode try data for issue %s. Error: %s", issue, err)
	}

	try.expandDigests()
	return try, nil
}

// List returns the last N Rietveld issue IDs that have been ingested.
func (t *TrybotResultStorage) List(offset, size int) ([]string, int, error) {
	var total int
	if err := t.vdb.DB.QueryRow("SELECT count(*) FROM tries").Scan(&total); err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []string{}, 0, nil
	}

	rows, err := t.vdb.DB.Query("SELECT issue FROM tries ORDER BY last_updated DESC LIMIT ?,?", offset, size)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to read try data from database: %s", err)
	}
	defer util.Close(rows)

	ret := make([]string, 0, size)
	for rows.Next() {
		var issue string
		if err := rows.Scan(&issue); err != nil {
			return nil, 0, fmt.Errorf("List: Failed to read issus from row: %s", err)
		}
		ret = append(ret, issue)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ret)))
	return ret, total, nil
}

// TrybotResultIngester implements the ingester.ResultIngester interface.
type TrybotResultIngester struct {
	tbrStorage     *TrybotResultStorage
	resultsByIssue map[string]*TryBotResults
}

func NewTrybotResultIngester(tbrStorage *TrybotResultStorage) ingester.ResultIngester {
	return &TrybotResultIngester{
		tbrStorage:     tbrStorage,
		resultsByIssue: map[string]*TryBotResults{},
	}
}

// See the ingester.ResultIngester interface.
func (t *TrybotResultIngester) Ingest(_ *ingester.TileTracker, opener ingester.Opener, fileInfo *ingester.ResultsFileLocation, counter metrics.Counter) error {
	r, err := opener()
	if err != nil {
		return fmt.Errorf("Unable to open reader: %s", err)
	}
	dmResults, err := goldingester.ParseDMResultsFromReader(r)
	if err != nil {
		return err
	}

	if _, ok := t.resultsByIssue[dmResults.Issue]; !ok {
		t.resultsByIssue[dmResults.Issue] = NewTryBotResults()
	}

	// Add the entire file to our current knowledge about this issue.
	t.resultsByIssue[dmResults.Issue].update(dmResults.Key, dmResults.Results, fileInfo.LastUpdated)
	counter.Inc(1)
	glog.Infof("Finished processing file %s.", fileInfo.Name)
	return nil
}

// See the ingester.ResultIngester interface.
func (t *TrybotResultIngester) BatchFinished(_ metrics.Counter) error {
	// Reset this instance regardless of the outcome of this call.
	defer func() {
		t.resultsByIssue = map[string]*TryBotResults{}
	}()

	for issue, tries := range t.resultsByIssue {
		// Get the current results.
		pastTries, err := t.tbrStorage.Get(issue)
		if err != nil {
			return err
		}

		needsUpdating := pastTries.updateIfNewer(tries)
		if needsUpdating {
			if err := t.tbrStorage.Write(issue, pastTries); err != nil {
				return err
			}
		}
	}

	glog.Info("Finished processing ingestion batch.")
	return nil
}

// TryBotResults stores the results of one entire issue.
type TryBotResults struct {
	// Constains a list of all digests contained in the issue.
	Digests []string

	// Results for specific bots.
	Bots map[string]*BotResults
}

// BotResults contains the results of one bot run.
type BotResults struct {
	BotParams   map[string]string
	TestResults []*TestResult
	TS          int64
}

// TestResult stores a digest and the params that are specific to one test.
type TestResult struct {
	Params    map[string]string
	DigestIdx int
	digest    string
}

// TryBotResults maps trace ids to trybot results.
// type TryBotResults map[string]*TBResult
func NewTryBotResults() *TryBotResults {
	return &TryBotResults{
		Bots: map[string]*BotResults{},
	}
}

// update incorporates the given restuls into the current results for this
// issue.
func (t *TryBotResults) update(botParams map[string]string, testResults []*goldingester.Result, timeStamp int64) {
	botId, err := util.MD5Params(botParams)
	if err != nil {
		glog.Errorf("Unable to hash bot parameters \n\n%v\n\n. Error: %s", botParams, err)
		return
	}

	current, ok := t.Bots[botId]
	if !ok || (current.TS < timeStamp) {
		// Replace the current entry for this bot.
		current = &BotResults{
			BotParams: botParams,
		}

		botTestResults := []*TestResult{}
		for _, result := range testResults {
			params := util.AddParams(result.Key, result.Options)
			if !goldingester.IgnoreResult(params) {
				botTestResults = append(botTestResults, &TestResult{
					Params: params,
					digest: result.Digest,
				})
			}
		}

		current.TestResults = botTestResults
		current.TS = timeStamp
		t.Bots[botId] = current
	}
}

// updateIfNewer incorporates the results of trybot runs into this results
// if they are newer.
func (t *TryBotResults) updateIfNewer(tries *TryBotResults) bool {
	updated := false
	for key, entry := range tries.Bots {
		found, ok := t.Bots[key]
		if !ok || (found.TS < entry.TS) {
			t.Bots[key] = entry
			updated = true
		}
	}
	return updated
}

func (t *TryBotResults) indexDigests() {
	digestIdx := map[string]int{}
	digestList := []string{}
	for _, bot := range t.Bots {
		for _, result := range bot.TestResults {
			if _, ok := digestIdx[result.digest]; !ok {
				digestIdx[result.digest] = len(digestList)
				digestList = append(digestList, result.digest)
			}
			result.DigestIdx = digestIdx[result.digest]
		}
	}
	t.Digests = digestList
}

func (t *TryBotResults) expandDigests() {
	indexDigestMap := make(map[int]string, len(t.Digests))
	for idx, digest := range t.Digests {
		indexDigestMap[idx] = digest
	}
	for _, bot := range t.Bots {
		for _, result := range bot.TestResults {
			result.digest = indexDigestMap[result.DigestIdx]
		}
	}
}
