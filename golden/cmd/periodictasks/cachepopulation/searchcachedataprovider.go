package cachepopulation

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/search"
	search_query "go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/types"
)

type searchCacheDataProvider struct {
	db                *pgxpool.Pool
	searchCacheConfig config.SearchCacheConfig
}

// newSearchCacheDataProvider returns a new instance of searchCacheDataProvider.
func newSearchCacheDataProvider(db *pgxpool.Pool, searchCacheConfig config.SearchCacheConfig) searchCacheDataProvider {
	return searchCacheDataProvider{
		db:                db,
		searchCacheConfig: searchCacheConfig,
	}
}

// GetData returns the data to be cached for search api.
func (s searchCacheDataProvider) GetData(ctx context.Context) (string, string, error) {
	ctx, span := trace.StartSpan(ctx, "searchcachedataprovider_run")
	defer span.End()

	searchApi := search.New(s.db, 10)

	// Create the default search query.
	query := search_query.Search{
		Limit: 50,
		TraceValues: map[string][]string{
			types.CorpusField: s.searchCacheConfig.Corpora,
		},
		RightTraceValues: map[string][]string{
			types.CorpusField: s.searchCacheConfig.Corpora,
		},
		Offset:                  0,
		Sort:                    "desc",
		IncludeUntriagedDigests: true,
	}
	results, err := searchApi.Search(ctx, &query)
	if err != nil {
		return "", "", err
	}

	resultJSON, err := toJSON(results)
	if err != nil {
		return "", "", err
	}
	return s.searchCacheConfig.DefaultPageCacheKey, resultJSON, nil
}

var _ CacheDataProvider = searchCacheDataProvider{}
