package caching

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/search/common"
)

// cacheDataProvider provides a struct for reading data for caching purposes.
type cacheDataProvider struct {
	db           *pgxpool.Pool
	corpora      []string
	commitWindow int
	query        string
	cacheKeyFunc func(string) string
}

// NewCacheDataProvider returns a new instance of the cacheDataProvider struct.
func NewCacheDataProvider(db *pgxpool.Pool, corpora []string, commitWindow int, sqlQuery string, cacheKeyFunc func(string) string) cacheDataProvider {
	return cacheDataProvider{
		db:           db,
		corpora:      corpora,
		commitWindow: commitWindow,
		query:        sqlQuery,
		cacheKeyFunc: cacheKeyFunc,
	}
}

// GetDataForCorpus returns the byblame data for the given corpus.
func (prov cacheDataProvider) GetDataForCorpus(ctx context.Context, firstCommitId string, corpus string) ([]SearchCacheData, error) {
	var cacheData []SearchCacheData
	rows, err := prov.db.Query(ctx, prov.query, firstCommitId, corpus)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		cacheDataObj := SearchCacheData{}
		if err := rows.Scan(&cacheDataObj.TraceID, &cacheDataObj.GroupingID, &cacheDataObj.Digest); err != nil {
			return nil, skerr.Wrap(err)
		}
		cacheData = append(cacheData, cacheDataObj)
	}

	return cacheData, nil
}

// GetCacheData implements cacheDataProvider.
func (prov cacheDataProvider) GetCacheData(ctx context.Context, firstCommitId string) (map[string]string, error) {
	cacheMap := map[string]string{}

	// For each of the corpora, execute the sql query and add the results to the map.
	for _, corpus := range prov.corpora {
		cacheData, err := prov.GetDataForCorpus(ctx, firstCommitId, corpus)
		if err != nil {
			return nil, err
		}
		if len(cacheData) > 0 {
			key := prov.cacheKeyFunc(corpus)
			cacheDataStr, err := common.ToJSON(cacheData)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			cacheMap[key] = cacheDataStr
		}
	}

	return cacheMap, nil
}
