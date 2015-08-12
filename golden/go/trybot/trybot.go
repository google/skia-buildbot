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
	"go.skia.org/infra/golden/go/types"
)

// Init registers trybot ingester. The supplied database connection is where
// trybot results are stored.
func Init(vdb *database.VersionedDB) {
	ingester.Register(config.CONSTRUCTOR_GOLD_TRYBOT, func() ingester.ResultIngester {
		return NewTrybotResultIngester(vdb)
	})
}

// Write writes trybot results to the SQL database connected to vdb that was the
// the argument to Init(..).
func Write(vdb *database.VersionedDB, issue string, trybotResults types.TryBotResults) error {
	b, err := json.Marshal(trybotResults)
	if err != nil {
		return fmt.Errorf("Failed to encode to JSON: %s", err)
	}

	// Find the latest timestamp in the data.
	var timeStamp int64 = 0
	for _, entry := range trybotResults {
		if entry.TS > timeStamp {
			timeStamp = entry.TS
		}
	}

	_, err = vdb.DB.Exec("REPLACE INTO tries (issue, results, last_updated) VALUES (?, ?, ?)", issue, b, timeStamp)
	if err != nil {
		return fmt.Errorf("Failed to write trybot data to database: %s", err)
	}
	return nil
}

// Get returns the trybot results for the given issue from the datastore.
func Get(vdb *database.VersionedDB, issue string) (types.TryBotResults, error) {
	var results string
	err := vdb.DB.QueryRow("SELECT results FROM tries WHERE issue=?", issue).Scan(&results)
	if err == sql.ErrNoRows {
		return types.NewTryBotResults(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to load try data with id %s: %s", issue, err)
	}

	try := types.TryBotResults{}
	if err := json.Unmarshal([]byte(results), &try); err != nil {
		return nil, fmt.Errorf("Failed to decode try data for issue %s. Error: %s", issue, err)
	}
	return try, nil
}

// List returns the last N Rietveld issue IDs that have been ingested.
func List(vdb *database.VersionedDB, n int) ([]string, error) {
	rows, err := vdb.DB.Query("SELECT issue FROM tries ORDER BY last_updated DESC LIMIT ?", n)
	if err != nil {
		return nil, fmt.Errorf("Failed to read try data from database: %s", err)
	}
	defer util.Close(rows)

	ret := make([]string, 0, n)
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

// TrybotResultIngester implements the ingester.ResultIngester interface.
type TrybotResultIngester struct {
	vdb            *database.VersionedDB
	resultsByIssue map[string]types.TryBotResults
}

func NewTrybotResultIngester(vdb *database.VersionedDB) ingester.ResultIngester {
	return &TrybotResultIngester{
		vdb:            vdb,
		resultsByIssue: map[string]types.TryBotResults{},
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

	dmResults.ForEach(func(key, value string, params map[string]string) {
		if _, ok := t.resultsByIssue[dmResults.Issue]; !ok {
			t.resultsByIssue[dmResults.Issue] = types.NewTryBotResults()
		}
		t.resultsByIssue[dmResults.Issue].Update(key, value, fileInfo.LastUpdated)
	})

	counter.Inc(1)
	glog.Infof("Finished processing file %s.", fileInfo.Name)
	return nil
}

// See the ingester.ResultIngester interface.
func (t *TrybotResultIngester) BatchFinished(_ metrics.Counter) error {
	// Reset this instance regardless of the outcome of this call.
	defer func() {
		t.resultsByIssue = map[string]types.TryBotResults{}
	}()

	for issue, tries := range t.resultsByIssue {
		// Get the current results.
		pastTries, err := Get(t.vdb, issue)
		if err != nil {
			return err
		}

		// Update the results with the results of this batch.
		needsUpdating := false
		for key, newTry := range tries {
			if found, ok := pastTries[key]; !ok || (ok && (found.TS < newTry.TS)) {
				pastTries[key] = newTry
				needsUpdating = true
			}
		}

		if needsUpdating {
			if err := Write(t.vdb, issue, pastTries); err != nil {
				return err
			}
		}
	}

	glog.Info("Finished processing ingestion batch.")
	return nil
}
