package gcsloader

import (
	"fmt"
	"sort"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/frontend/fuzzcache"
	"go.skia.org/infra/fuzzer/go/frontend/fuzzpool"
	fstorage "go.skia.org/infra/fuzzer/go/storage"
	"go.skia.org/infra/go/sklog"
)

// LoadFromBoltDB fills the given *fuzzpool.FuzzPool from FuzzReportCache associated with the given
//  hash. The data.FuzzReports will all be in the current part of the pool.
// If a cache for the commit does not exist, or there are other problems with the retrieval,
// an error is returned.  We do not need to deduplicate on extraction because
// the fuzzes were deduplicated on storage.
func LoadFromBoltDB(pool *fuzzpool.FuzzPool, cache *fuzzcache.FuzzReportCache) error {
	sklog.Infof("Looking into cache for revision %s", config.Common.SkiaVersion.Hash)
	if err := cache.LoadPool(pool, config.Common.SkiaVersion.Hash); err != nil {
		return fmt.Errorf("Could not load from cache: %s", err)
	}
	sklog.Infof("Loaded a fuzz pool of size %d", len(pool.Reports()))
	return nil
}

// GCSLoader is a struct that handles downloading fuzzes from Google Storage.
type GCSLoader struct {
	storageClient *storage.Client
	Cache         *fuzzcache.FuzzReportCache
	Pool          *fuzzpool.FuzzPool
}

// New creates a GCSLoader and returns it.
func New(s *storage.Client, c *fuzzcache.FuzzReportCache, p *fuzzpool.FuzzPool) *GCSLoader {
	return &GCSLoader{
		storageClient: s,
		Cache:         c,
		Pool:          p,
	}
}

// LoadFreshFromGoogleStorage pulls all bad and grey fuzzes out of GCS and loads them into memory.
// The "fresh" in the name refers to the fact that all other loaded fuzzes (if any)
// are written over, including in the cache.
// Upon completion, the full results are cached to a boltDB instance and moved from staging
// to the current copy.
func (g *GCSLoader) LoadFreshFromGoogleStorage() error {
	revision := config.Common.SkiaVersion.Hash
	g.Pool.ClearStaging()
	fuzzNames := make([]string, 0, 100)

	for _, arch := range common.ARCHITECTURES {
		for _, cat := range common.FUZZ_CATEGORIES {
			badPath := fmt.Sprintf("%s/%s/%s/bad", cat, revision, arch)
			reports, err := fstorage.GetReportsFromGCS(g.storageClient, badPath, cat, arch, nil, config.FrontEnd.NumDownloadProcesses)
			if err != nil {
				return err
			}
			b := 0
			for report := range reports {
				report.IsGrey = false
				fuzzNames = append(fuzzNames, report.FuzzName)
				g.Pool.AddFuzzReport(report)
				b++
			}
			sklog.Infof("%d bad fuzzes freshly loaded from gs://%s/%s", b, config.GCS.Bucket, badPath)

			greyPath := fmt.Sprintf("%s/%s/%s/grey", cat, revision, arch)
			reports, err = fstorage.GetReportsFromGCS(g.storageClient, greyPath, cat, arch, nil, config.FrontEnd.NumDownloadProcesses)
			if err != nil {
				return err
			}
			b = 0
			for report := range reports {
				report.IsGrey = true
				fuzzNames = append(fuzzNames, report.FuzzName)
				g.Pool.AddFuzzReport(report)
				b++
			}
			sklog.Infof("%d grey fuzzes freshly loaded from gs://%s/%s", b, config.GCS.Bucket, greyPath)
		}
	}
	g.Pool.CurrentFromStaging()
	g.Pool.ClearStaging()
	if err := g.Cache.StorePool(g.Pool, revision); err != nil {
		return fmt.Errorf("Problem storing fuzz pool to boltDB: %s", err)
	}
	return g.Cache.StoreFuzzNames(fuzzNames, revision)
}

// LoadFuzzesFromGoogleStorage pulls all fuzzes out of GCS that are on the given whitelist
// and loads them into memory (as staging). These fuzzes represent the newly found bad fuzzes
// that have been uploaded by the various generators/aggregators. After loading them, it
// updates the cache and moves them from staging to the current copy.
func (g *GCSLoader) LoadFuzzesFromGoogleStorage(whitelist []string) error {
	revision := config.Common.SkiaVersion.Hash
	g.Pool.StagingFromCurrent()
	sort.Strings(whitelist)

	fuzzNames := make([]string, 0, 100)
	for _, arch := range common.ARCHITECTURES {
		for _, cat := range common.FUZZ_CATEGORIES {
			badPath := fmt.Sprintf("%s/%s/%s/bad", cat, revision, arch)
			reports, err := fstorage.GetReportsFromGCS(g.storageClient, badPath, cat, arch, whitelist, config.FrontEnd.NumDownloadProcesses)
			if err != nil {
				return err
			}
			b := 0
			for report := range reports {
				report.IsGrey = false
				fuzzNames = append(fuzzNames, report.FuzzName)
				g.Pool.AddFuzzReport(report)
				b++
			}
			sklog.Infof("%d bad fuzzes incrementally loaded from gs://%s/%s", b, config.GCS.Bucket, badPath)
		}
	}
	// We must wait until after all the fuzzes are in staging, otherwise, we'll only have a partial update
	g.Pool.CurrentFromStaging()
	g.Pool.ClearStaging()
	oldBinaryFuzzNames, err := g.Cache.LoadFuzzNames(revision)
	if err != nil {
		sklog.Warningf("Could not read old binary fuzz names from cache: %s\n  Continuing...", err)
		oldBinaryFuzzNames = []string{}
	}
	if err := g.Cache.StorePool(g.Pool, revision); err != nil {
		return fmt.Errorf("Problem storing fuzz pool to boltDB: %s", err)
	}
	return g.Cache.StoreFuzzNames(append(oldBinaryFuzzNames, whitelist...), revision)
}
