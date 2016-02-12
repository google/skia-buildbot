package gsloader

import (
	"fmt"
	"sort"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/fuzzer/go/fuzzcache"
	fstorage "go.skia.org/infra/fuzzer/go/storage"
	"google.golang.org/cloud/storage"
)

// LoadFromBoltDB loads the data.FuzzReport from FuzzReportCache associated with the given hash.
// The FuzzReport is first put into the staging fuzz cache, and then into the current.
// If a cache for the commit does not exist, or there are other problems with the retrieval,
// an error is returned.  We do not need to deduplicate on extraction because
// the fuzzes were deduplicated on storage.
func LoadFromBoltDB(cache *fuzzcache.FuzzReportCache) error {
	glog.Infof("Looking into cache for revision %s", config.FrontEnd.SkiaVersion.Hash)
	for _, category := range common.FUZZ_CATEGORIES {
		if staging, err := cache.LoadTree(category, config.FrontEnd.SkiaVersion.Hash); err != nil {
			return fmt.Errorf("Problem decoding existing from bolt db: %s", err)
		} else {
			data.SetStaging(category, *staging)
			glog.Infof("Successfully loaded %s fuzzes from bolt db cache with %d files", category, len(*staging))
		}
	}
	data.StagingToCurrent()
	return nil
}

// GSLoader is a struct that handles downloading fuzzes from Google Storage.
type GSLoader struct {
	storageClient *storage.Client
	Cache         *fuzzcache.FuzzReportCache
}

// New creates a GSLoader and returns it.
func New(s *storage.Client, c *fuzzcache.FuzzReportCache) *GSLoader {
	return &GSLoader{
		storageClient: s,
		Cache:         c,
	}
}

// LoadFreshFromGoogleStorage pulls all fuzzes out of GCS and loads them into memory.
// The "fresh" in the name refers to the fact that all other loaded fuzzes (if any)
// are written over, including in the cache.
// Upon completion, the full results are cached to a boltDB instance and moved from staging
// to the current copy.
func (g *GSLoader) LoadFreshFromGoogleStorage() error {
	revision := config.FrontEnd.SkiaVersion.Hash
	data.ClearStaging()
	fuzzNames := make([]string, 0, 100)

	for _, cat := range common.FUZZ_CATEGORIES {
		badPath := fmt.Sprintf("%s/%s/bad", cat, revision)
		reports, err := fstorage.GetReportsFromGS(g.storageClient, badPath, cat, nil, config.FrontEnd.NumDownloadProcesses)
		if err != nil {
			return err
		}
		b := 0
		for report := range reports {
			fuzzNames = append(fuzzNames, report.FuzzName)
			data.NewFuzzFound(cat, report)
			b++
		}
		glog.Infof("%d bad fuzzes freshly loaded from gs://%s/%s", b, config.GS.Bucket, badPath)
	}
	// We must wait until after all the fuzzes are in staging, otherwise, we'll only have a partial update
	data.StagingToCurrent()

	for _, category := range common.FUZZ_CATEGORIES {
		if err := g.Cache.StoreTree(data.StagingCopy(category), category, revision); err != nil {
			glog.Errorf("Problem storing category %s to boltDB: %s", category, err)
		}
	}
	return g.Cache.StoreFuzzNames(fuzzNames, revision)
}

// LoadFuzzesFromGoogleStorage pulls all fuzzes out of GCS that are on the given whitelist
// and loads them into memory (as staging).  After loading them, it updates the cache
// and moves them from staging to the current copy.
func (g *GSLoader) LoadFuzzesFromGoogleStorage(whitelist []string) error {
	revision := config.FrontEnd.SkiaVersion.Hash
	data.StagingFromCurrent()
	sort.Strings(whitelist)

	fuzzNames := make([]string, 0, 100)
	for _, cat := range common.FUZZ_CATEGORIES {
		badPath := fmt.Sprintf("%s/%s/bad", cat, revision)
		reports, err := fstorage.GetReportsFromGS(g.storageClient, badPath, cat, whitelist, config.FrontEnd.NumDownloadProcesses)
		if err != nil {
			return err
		}
		b := 0
		for report := range reports {
			fuzzNames = append(fuzzNames, report.FuzzName)
			data.NewFuzzFound(cat, report)
			b++
		}
		glog.Infof("%d bad fuzzes incrementally loaded from gs://%s/%s", b, config.GS.Bucket, badPath)
	}
	// We must wait until after all the fuzzes are in staging, otherwise, we'll only have a partial update
	data.StagingToCurrent()

	oldBinaryFuzzNames, err := g.Cache.LoadFuzzNames(revision)
	if err != nil {
		glog.Warningf("Could not read old binary fuzz names from cache.  Continuing...", err)
		oldBinaryFuzzNames = []string{}
	}
	for _, category := range common.FUZZ_CATEGORIES {
		if err := g.Cache.StoreTree(data.StagingCopy(category), category, revision); err != nil {
			glog.Errorf("Problem storing category %s to boltDB: %s", category, err)
		}
	}
	return g.Cache.StoreFuzzNames(append(oldBinaryFuzzNames, whitelist...), revision)
}
