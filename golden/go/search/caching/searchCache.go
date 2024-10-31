package caching

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/skerr"
)

type SearchCacheType int

const (
	// ByBlame_Corpus denotes the cache type for untriaged images by commits for a given corpus.
	ByBlame_Corpus SearchCacheType = iota
	// Unignored_Corpus denotes the cache type for traces without ignore rules.
	Unignored_Corpus
)

// SearchCacheManager provides a struct to handle the cache operations for gold search.
type SearchCacheManager struct {
	cacheClient   cache.Cache
	db            *pgxpool.Pool
	corpora       []string
	commitWindow  int
	dataProviders map[SearchCacheType]cacheDataProvider
}

// New returns a new instance of the SearchCacheManager.
func New(cacheClient cache.Cache, db *pgxpool.Pool, corpora []string, commitWindow int) *SearchCacheManager {
	return &SearchCacheManager{
		cacheClient:  cacheClient,
		db:           db,
		corpora:      corpora,
		commitWindow: commitWindow,
		dataProviders: map[SearchCacheType]cacheDataProvider{
			ByBlame_Corpus:   NewCacheDataProvider(db, corpora, commitWindow, ByBlameQuery),
			Unignored_Corpus: NewCacheDataProvider(db, corpora, commitWindow, UnignoredQuery),
		},
	}
}

// RunCachePopulation gets the cache data from the providers and stores it in the cache instance.
func (s SearchCacheManager) RunCachePopulation(ctx context.Context) error {
	for _, prov := range s.dataProviders {
		data, err := prov.GetCacheData(ctx)
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
func (s SearchCacheManager) GetByBlameData(ctx context.Context, corpus string) ([]SearchCacheData, error) {
	cacheKey := ByBlameKey(corpus)
	return s.getDataFromCache(ctx, corpus, cacheKey, ByBlame_Corpus)
}

// GetUnignoredTracesData returns the unignored traces data for the given corpus from cache.
func (s SearchCacheManager) GetUnignoredTracesData(ctx context.Context, corpus string) ([]SearchCacheData, error) {
	cacheKey := UnignoredKey(corpus)
	return s.getDataFromCache(ctx, corpus, cacheKey, Unignored_Corpus)
}

// getDataFromCache returns cached data for the given parameters from the configured cache. If there is a cache miss,
// it will return the data from the database instead.
func (s SearchCacheManager) getDataFromCache(ctx context.Context, corpus string, cacheKey string, cacheType SearchCacheType) ([]SearchCacheData, error) {
	data := []SearchCacheData{}
	jsonStr, err := s.cacheClient.GetValue(ctx, cacheKey)
	if err != nil {
		return data, skerr.Wrapf(err, "Error retrieving data from cache for key %s corpus %s", cacheKey, corpus)
	}

	// This is the case when there is a cache miss.
	if jsonStr == "" {
		var provider cacheDataProvider
		var ok bool
		if provider, ok = s.dataProviders[cacheType]; !ok {
			return nil, skerr.Fmt("Invalid cache type %d specified.", cacheType)
		}
		return provider.GetDataForCorpus(ctx, corpus)
	}

	err = json.Unmarshal([]byte(jsonStr), &data)
	return data, err
}
