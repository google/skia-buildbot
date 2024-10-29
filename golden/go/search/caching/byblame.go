package caching

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/skerr"
)

// This query collects untriaged image digests within the specified commit window for the given
// corpus where an ignore rule is not applied. This data is used when the user wants to see
// a list of untriaged digests for the specific corpus in the UI.
const (
	ByBlameQuery = `WITH
BeginningOfWindow AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC
	OFFSET $1 LIMIT 1
),
UntriagedDigests AS (
	SELECT grouping_id, digest FROM Expectations
	WHERE label = 'u'
),
UnignoredDataAtHead AS (
	SELECT trace_id, grouping_id, digest FROM ValuesAtHead
	JOIN BeginningOfWindow ON ValuesAtHead.most_recent_commit_id >= BeginningOfWindow.commit_id
	WHERE matches_any_ignore_rule = FALSE AND corpus = $2
)
SELECT UnignoredDataAtHead.trace_id, UnignoredDataAtHead.grouping_id, UnignoredDataAtHead.digest FROM
UntriagedDigests
JOIN UnignoredDataAtHead ON UntriagedDigests.grouping_id = UnignoredDataAtHead.grouping_id AND
	 UntriagedDigests.digest = UnignoredDataAtHead.digest`
)

// ByBlameDataProvider implements cacheDataProvider.
type ByBlameDataProvider struct {
	db           *pgxpool.Pool
	corpora      []string
	commitWindow int
}

func NewByBlameDataProvider(db *pgxpool.Pool, corpora []string, commitWindow int) ByBlameDataProvider {
	return ByBlameDataProvider{
		db:           db,
		corpora:      corpora,
		commitWindow: commitWindow,
	}
}

// GetDataForCorpus returns the byblame data for the given corpus.
func (prov ByBlameDataProvider) GetDataForCorpus(ctx context.Context, corpus string) ([]ByBlameData, error) {
	cacheData := []ByBlameData{}
	rows, err := prov.db.Query(ctx, ByBlameQuery, prov.commitWindow, corpus)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		byBlameData := ByBlameData{}
		if err := rows.Scan(&byBlameData.TraceID, &byBlameData.GroupingID, &byBlameData.Digest); err != nil {
			return nil, skerr.Wrap(err)
		}
		cacheData = append(cacheData, byBlameData)
	}

	return cacheData, nil
}

// GetCacheData implements cacheDataProvider.
func (prov ByBlameDataProvider) GetCacheData(ctx context.Context) (map[string]string, error) {
	cacheMap := map[string]string{}

	// For each of the corpora, execute the sql query and add the results to the map.
	for _, corpus := range prov.corpora {
		cacheData, err := prov.GetDataForCorpus(ctx, corpus)
		if err != nil {
			return nil, err
		}
		if len(cacheData) > 0 {
			key := ByBlameKey(corpus)
			cacheDataStr, err := toJSON(cacheData)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			cacheMap[key] = cacheDataStr
		}
	}

	return cacheMap, nil
}
