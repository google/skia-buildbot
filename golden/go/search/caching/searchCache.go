package caching

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/sql/schema"
)

type SearchCacheType int

const (
	// ByBlame_Corpus denotes the cache type for untriaged images by commits for a given corpus.
	ByBlame_Corpus SearchCacheType = iota
	// MatchingTraces denotes the cache type for search results.
	MatchingTraces
)

// SearchCacheData provides a struct to hold data for the entry in by blame cache.
type SearchCacheData struct {
	TraceID    schema.TraceID     `json:"traceID"`
	GroupingID schema.GroupingID  `json:"groupingID"`
	Digest     schema.DigestBytes `json:"digest"`
}

// SearchCacheManager provides a struct to handle the cache operations for gold search.
type SearchCacheManager struct {
	cacheClient   cache.Cache
	db            *pgxpool.Pool
	corpora       []string
	commitWindow  int
	dataProviders map[SearchCacheType]cacheDataProvider

	publiclyVisibleTraces map[schema.MD5Hash]struct{}
	isPublicView          bool
	dbType                config.DatabaseType
}

// New returns a new instance of the SearchCacheManager.
func New(cacheClient cache.Cache, db *pgxpool.Pool, corpora []string, commitWindow int) *SearchCacheManager {
	return &SearchCacheManager{
		cacheClient:  cacheClient,
		db:           db,
		corpora:      corpora,
		commitWindow: commitWindow,
		dataProviders: map[SearchCacheType]cacheDataProvider{
			ByBlame_Corpus: NewByBlameCacheDataProvider(db, corpora, commitWindow, ByBlameQuery, ByBlameKey),
			MatchingTraces: NewMatchingTracesCacheDataProvider(db, corpora, commitWindow),
		},
	}
}

// SetDatabaseType sets the database type for the current configuration.
func (s *SearchCacheManager) SetDatabaseType(dbType config.DatabaseType) {
	s.dbType = dbType
	for _, prov := range s.dataProviders {
		prov.SetDatabaseType(dbType)
	}
}

// SetPublicTraces sets the given traces as the publicly visible ones.
func (s *SearchCacheManager) SetPublicTraces(traces map[schema.MD5Hash]struct{}) {
	s.publiclyVisibleTraces = traces
	s.isPublicView = true
	for _, prov := range s.dataProviders {
		prov.SetPublicTraces(traces)
	}
}

// RunCachePopulation gets the cache data from the providers and stores it in the cache instance.
func (s SearchCacheManager) RunCachePopulation(ctx context.Context) error {
	ctx, err := common.AddCommitsData(ctx, s.db, s.commitWindow)
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, prov := range s.dataProviders {
		data, err := prov.GetCacheData(ctx, string(common.GetFirstCommitID(ctx)))
		if err != nil {
			return skerr.Wrapf(err, "Error while running cache population with provider %v", prov)
		}

		for key, val := range data {
			err := s.cacheClient.SetValue(ctx, key, val)
			if err != nil {
				return skerr.Wrapf(err, "Error while setting value in cache.")
			}
		}
	}

	return nil
}

// GetByBlameData returns the by blame data for the given corpus from cache.
func (s SearchCacheManager) GetByBlameData(ctx context.Context, firstCommitId string, corpus string) ([]SearchCacheData, error) {
	cacheKey := ByBlameKey(corpus)
	return s.getByBlameDataFromCache(ctx, firstCommitId, corpus, cacheKey)
}

// getByBlameDataFromCache returns cached data for the given parameters from the configured cache. If there is a cache miss,
// it will return the data from the database instead.
func (s SearchCacheManager) getByBlameDataFromCache(ctx context.Context, firstCommitId, corpus string, cacheKey string) ([]SearchCacheData, error) {
	data := []SearchCacheData{}
	jsonStr, err := s.cacheClient.GetValue(ctx, cacheKey)
	if err != nil {
		return data, skerr.Wrapf(err, "Error retrieving data from cache for key %s corpus %s", cacheKey, corpus)
	}

	// This is the case when there is a cache miss.
	if jsonStr == "" {
		var provider byBlameCacheDataProvider
		var ok bool
		if provider, ok = s.dataProviders[ByBlame_Corpus].(byBlameCacheDataProvider); !ok {
			return nil, skerr.Fmt("ByBlame cache data provider is not initialized.")
		}
		return provider.GetDataForCorpus(ctx, firstCommitId, corpus)
	}

	err = json.Unmarshal([]byte(jsonStr), &data)
	return data, err
}

// GetMatchingDigestsAndTraces returns the digests and traces for the given search query from the cache.
//
// Note: On a cache miss it attempts to perform a database search.
func (s SearchCacheManager) GetMatchingDigestsAndTraces(ctx context.Context, searchQueryContext MatchingTracesQueryContext) ([]common.DigestWithTraceAndGrouping, error) {
	cacheKeys := []string{}
	contextMap := map[string]MatchingTracesQueryContext{}
	if searchQueryContext.IncludeUntriaged {
		key := MatchingUntriagedTracesKey(searchQueryContext.Corpus)
		cacheKeys = append(cacheKeys, key)
		contextMap[key] = MatchingTracesQueryContext{
			IncludeUntriaged:                 true,
			OnlyIncludeDigestsProducedAtHead: searchQueryContext.OnlyIncludeDigestsProducedAtHead,
			TraceValues:                      searchQueryContext.TraceValues,
			Corpus:                           searchQueryContext.Corpus,
		}
	}
	if searchQueryContext.IncludePositive {
		key := MatchingPositiveTracesKey(searchQueryContext.Corpus)
		cacheKeys = append(cacheKeys, key)
		contextMap[key] = MatchingTracesQueryContext{
			IncludePositive:                  true,
			OnlyIncludeDigestsProducedAtHead: searchQueryContext.OnlyIncludeDigestsProducedAtHead,
			TraceValues:                      searchQueryContext.TraceValues,
			Corpus:                           searchQueryContext.Corpus,
		}
	}
	if searchQueryContext.IncludeNegative {
		key := MatchingNegativeTracesKey(searchQueryContext.Corpus)
		cacheKeys = append(cacheKeys, key)
		contextMap[key] = MatchingTracesQueryContext{
			IncludeNegative:                  true,
			OnlyIncludeDigestsProducedAtHead: searchQueryContext.OnlyIncludeDigestsProducedAtHead,
			TraceValues:                      searchQueryContext.TraceValues,
			Corpus:                           searchQueryContext.Corpus,
		}
	}

	// We have one cache key per selected option. Let's get the cache data per key and then
	// combine the result.
	matchingTracesAndDigestsMap := map[string][]common.DigestWithTraceAndGrouping{}
	for _, cacheKey := range cacheKeys {
		jsonStr, err := s.cacheClient.GetValue(ctx, cacheKey)
		if err != nil {
			return nil, skerr.Wrapf(err, "Error retrieving data from cache for key %s queryContext %v", cacheKey, searchQueryContext)
		}

		var digests []common.DigestWithTraceAndGrouping
		// This is the case when there is a cache miss.
		if jsonStr == "" {
			sklog.Infof("No data found in cache for key %s. Attempting db search.", cacheKey)
			prov := NewMatchingTracesCacheDataProvider(s.db, s.corpora, s.commitWindow)
			if s.isPublicView {
				prov.SetPublicTraces(s.publiclyVisibleTraces)
			}
			prov.SetDatabaseType(s.dbType)
			digests, err = prov.getMatchingDigestsAndTracesFromDB(ctx, contextMap[cacheKey])
		} else {
			err = json.Unmarshal([]byte(jsonStr), &digests)
		}

		if err != nil {
			sklog.Errorf("Error during retrieving data from cache: %v", err)
			return nil, err
		}

		matchingTracesAndDigestsMap[cacheKey] = digests
	}

	var matchingTracesAndDigests []common.DigestWithTraceAndGrouping
	for _, digests := range matchingTracesAndDigestsMap {
		matchingTracesAndDigests = append(matchingTracesAndDigests, digests...)
	}
	sklog.Infof("Returning %d digests.", len(matchingTracesAndDigests))
	return matchingTracesAndDigests, nil
}

// GetDigestsForGrouping retrieves the digests for a given grouping and traceKey set from the cache.
func (s SearchCacheManager) GetDigestsForGrouping(ctx context.Context, groupingID schema.GroupingID, traceKeys paramtools.ParamSet) ([]schema.DigestBytes, error) {
	traces, err := traceKeys.Freeze().ToString()
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	cacheKey := DigestsForGroupingKey(groupingID, traces)
	jsonStr, err := s.cacheClient.GetValue(ctx, cacheKey)
	if err != nil {
		return nil, skerr.Wrapf(err, "Error retrieving digest for group from cache for key %s", cacheKey)
	}

	// This is the case when there is a cache miss.
	if jsonStr == "" {
		return nil, nil
	}

	var digests []schema.DigestBytes
	err = json.Unmarshal([]byte(jsonStr), &digests)

	return digests, skerr.Wrap(err)
}

// SetDigestsForGrouping sets the digests for a given grouping and traceKey set into the cache.
func (s SearchCacheManager) SetDigestsForGrouping(ctx context.Context, groupingID schema.GroupingID, traceKeys paramtools.ParamSet, digests []schema.DigestBytes) error {
	traces, err := traceKeys.Freeze().ToString()
	if err != nil {
		return skerr.Wrap(err)
	}

	cacheKey := DigestsForGroupingKey(groupingID, traces)
	jsonStr, err := common.ToJSON(digests)
	if err != nil {
		return skerr.Wrapf(err, "Error converting digests into a json string.")
	}

	return s.cacheClient.SetValue(ctx, cacheKey, jsonStr)
}
