package caching

import (
	"context"
	"encoding/hex"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/web/frontend"
)

// testDigestsProvider is a struct used for getting caching data
// for digest details grouped by tests.
type testDigestsProvider struct {
	db           *pgxpool.Pool
	corpora      []string
	commitWindow int
	dbType       config.DatabaseType
}

// NewTestDigestsProvider returns a new instance of the testDigestsProvider.
func NewTestDigestsProvider(db *pgxpool.Pool, corpora []string, commitWindow int) *testDigestsProvider {
	return &testDigestsProvider{
		db:           db,
		corpora:      corpora,
		commitWindow: commitWindow,
	}
}

// SetDatabaseType sets the database type for the current configuration.
func (prov *testDigestsProvider) SetDatabaseType(dbType config.DatabaseType) {
	prov.dbType = dbType
}

// SetPublicTraces sets the given traces as the publicly visible ones.
func (prov testDigestsProvider) SetPublicTraces(traces map[schema.MD5Hash]struct{}) {
	// No op.
}

// GetDataForCorpus returns the testDigests data for the given corpus.
func (prov testDigestsProvider) GetDataForCorpus(ctx context.Context, corpus string, excludeIgnoredTraces bool, filterParamset paramtools.ParamSet) ([]frontend.TestSummary, error) {
	summaries, err := prov.getDigestsByTestsFromDB(ctx, corpus, excludeIgnoredTraces, filterParamset)
	if err != nil {
		return nil, err
	}
	withTotals := make([]frontend.TestSummary, 0, len(summaries))
	for _, s := range summaries {
		s.TotalDigests = s.UntriagedDigests + s.PositiveDigests + s.NegativeDigests
		withTotals = append(withTotals, *s)
	}
	return withTotals, nil
}

// GetCacheData returns the testDigests data to be cached.
func (prov testDigestsProvider) GetCacheData(ctx context.Context, firstCommitId string) (map[string]string, error) {
	cacheMap := map[string]string{}

	// For each of the corpora, execute the sql query and add the results to the map.
	for _, corpus := range prov.corpora {
		summaryForCorpus, err := prov.GetDataForCorpus(ctx, corpus, true, nil)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		if len(summaryForCorpus) > 0 {
			key := DigestsByTestKey(corpus)
			cacheDataStr, err := common.ToJSON(summaryForCorpus)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			cacheMap[key] = cacheDataStr
		}
	}

	return cacheMap, nil
}

// getDigestsByTestsFromDB returns the testDigests data from the database.
func (prov testDigestsProvider) getDigestsByTestsFromDB(ctx context.Context, corpus string, excludeIgnoredTraces bool, filterParamset paramtools.ParamSet) ([]*frontend.TestSummary, error) {
	ctx, span := trace.StartSpan(ctx, "getDigestsByTestsDB")
	defer span.End()

	statement := `WITH
CommitsInWindow AS (
	SELECT commit_id, tile_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
),
OldestCommitInWindow AS (
	SELECT commit_id, tile_id FROM CommitsInWindow
	ORDER BY commit_id ASC LIMIT 1
),`
	digestsStatement, digestsArgs, err := digestCountTracesStatement(corpus, excludeIgnoredTraces, filterParamset)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	statement += digestsStatement
	statement += `DigestsWithLabels AS (
	SELECT Groupings.grouping_id, Groupings.keys AS grouping, label, DigestsOfInterest.digest
	FROM DigestsOfInterest
	JOIN Expectations ON DigestsOfInterest.grouping_id = Expectations.grouping_id
	                     AND DigestsOfInterest.digest = Expectations.digest
	JOIN Groupings ON DigestsOfInterest.grouping_id = Groupings.grouping_id
)
`
	selectStmt := `SELECT encode(grouping_id, 'hex'), grouping, label, COUNT(digest) FROM DigestsWithLabels
GROUP BY grouping_id, grouping, label ORDER BY grouping->>'name'`
	if prov.dbType == config.Spanner {
		// Spanner does not support grouping based on JSONB fields. Hence we select all the
		// data first and then do the grouping later in memory.
		selectStmt = `SELECT grouping_id, grouping, label, digest FROM DigestsWithLabels`
	}
	statement += selectStmt

	arguments := []interface{}{prov.commitWindow}
	arguments = append(arguments, digestsArgs...)
	sklog.Infof("statement: %s", statement)
	sklog.Infof("arguments: %s", arguments)
	rows, err := prov.db.Query(ctx, statement, arguments...)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var summaries []*frontend.TestSummary
	if prov.dbType == config.Spanner {
		summaries, err = getTestSummariesFromSpanner(rows)
		if err != nil {
			sklog.Errorf("Error retrieving test summaries from Spanner: %v", err)
			return nil, err
		}
	} else {
		var currentSummary *frontend.TestSummary
		var currentSummaryGroupingID string
		for rows.Next() {
			var groupingID string
			var grouping paramtools.Params
			var label schema.ExpectationLabel
			var count int
			if err := rows.Scan(&groupingID, &grouping, &label, &count); err != nil {
				return nil, skerr.Wrap(err)
			}
			if currentSummary == nil || currentSummaryGroupingID != groupingID {
				currentSummary = &frontend.TestSummary{Grouping: grouping}
				currentSummaryGroupingID = groupingID
				summaries = append(summaries, currentSummary)
			}
			if label == schema.LabelNegative {
				currentSummary.NegativeDigests = count
			} else if label == schema.LabelPositive {
				currentSummary.PositiveDigests = count
			} else {
				currentSummary.UntriagedDigests = count
			}
		}
	}

	return summaries, nil
}

// getTestSummariesFromSpanner returns a list of test summaries from the rows returned from
// querying against the spanner database.
func getTestSummariesFromSpanner(rows pgx.Rows) ([]*frontend.TestSummary, error) {
	// We need to get the number of digests per label (untriaged, positive, negative) for
	// each grouping. We do this by creating a map that uses the groupingID as the key and
	// the corresponding summary as the value. The query returns a separate row for each label
	// so we use this map to ensure that the same summary object is updated for the grouping
	// for each label that's corresponding to the groupingID.
	summaryMap := map[string]*frontend.TestSummary{}
	for rows.Next() {
		var grouping paramtools.Params
		var label schema.ExpectationLabel
		var groupingIdBytes []byte
		var digest []byte
		if err := rows.Scan(&groupingIdBytes, &grouping, &label, &digest); err != nil {
			return nil, skerr.Wrap(err)
		}
		groupingID := hex.EncodeToString(groupingIdBytes)

		if _, ok := summaryMap[groupingID]; !ok {
			// If an existing summary is not present, let's create a new one.
			summaryMap[groupingID] = &frontend.TestSummary{Grouping: grouping}
		}

		if label == schema.LabelNegative {
			summaryMap[groupingID].NegativeDigests++
		} else if label == schema.LabelPositive {
			summaryMap[groupingID].PositiveDigests++
		} else {
			summaryMap[groupingID].UntriagedDigests++
		}
	}

	// Now let's extract all the summary objects from the map and return
	// the resulting array.
	summaries := []*frontend.TestSummary{}
	for _, summary := range summaryMap {
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// digestCountTracesStatementForBatch returns a statement and arguments that will return all tests,
// digests and their grouping ids. The results will be in a table called DigestsWithLabels.
func digestCountTracesStatement(corpus string, excludeIgnoredTraces bool, filterParamset paramtools.ParamSet) (string, []interface{}, error) {
	arguments := []interface{}{corpus}
	statement := `DigestsOfInterest AS (
	SELECT DISTINCT keys->>'name' AS test_name, digest, grouping_id FROM ValuesAtHead
	JOIN OldestCommitInWindow ON ValuesAtHead.most_recent_commit_id >= OldestCommitInWindow.commit_id
	WHERE corpus = $2`
	if excludeIgnoredTraces {
		statement += ` AND matches_any_ignore_rule = FALSE`
	}
	if len(filterParamset) > 0 {
		jObj := map[string]string{}
		for key, values := range filterParamset {
			if len(values) != 1 {
				return "", nil, skerr.Fmt("not implemented: we only support one value per key")
			}
			jObj[key] = values[0]
		}
		statement += ` AND keys @> $3`
		arguments = append(arguments, jObj)
	}
	statement += "),"
	return statement, arguments, nil
}
