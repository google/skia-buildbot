package syncer

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/frontend/gcsloader"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// FuzzSyncer is a struct that will fetch the bad/grey fuzzes from GCS at discrete intervals.
// Once started, it will occasionally wake up and download any new fuzzes from GCS using the
// supplied gcsloader. It looks at both the current revision and the previous revision to count
//regressions.  Clients should look at LastCount for the most recent result of the counts.
type FuzzSyncer struct {
	countMutex    sync.Mutex
	storageClient *storage.Client
	gcsLoader     *gcsloader.GCSLoader
	lastCount     map[string]FuzzCount      // maps category->FuzzCount
	fuzzNameCache map[string]util.StringSet // maps key->FuzzNames
}

// FuzzCount is a struct that holds the counts of fuzzes across all architectures.
type FuzzCount struct {
	HighPriority int `json:"highPriorityCount"`
	MedPriority  int `json:"mediumPriorityCount"`
	LowPriority  int `json:"lowPriorityCount"`
}

var HIGH_PRIORITY_FLAGS = util.NewStringSet([]string{
	"ASAN_global-buffer-overflow",
	"ASAN_heap-buffer-overflow",
	"ASAN_stack-buffer-overflow",
	"ASAN_heap-use-after-free",
})

var MEDIUM_PRIORITY_FLAGS = util.NewStringSet([]string{
	"ClangCrashed",
	"ASANCrashed",
	"Other",
})

// LOW_PRIORITY is everything else

// NewFuzzSyncer creates a FuzzSyncer and returns it.
func New(s *storage.Client) *FuzzSyncer {
	return &FuzzSyncer{
		storageClient: s,
		lastCount:     make(map[string]FuzzCount),
		fuzzNameCache: make(map[string]util.StringSet),
	}
}

// SetGCSLoader sets this objects GCSLoader, allowing it to fetch the new fuzzes it finds
// and update the fuzzes displayed to users.
func (f *FuzzSyncer) SetGCSLoader(g *gcsloader.GCSLoader) {
	f.gcsLoader = g
}

// Start updates the LastCount and starts a timer with a period of config.FrontEnd.FuzzSyncPeriod.
// Any errors are logged.
func (f *FuzzSyncer) Start() {
	go func() {
		t := time.Tick(config.FrontEnd.FuzzSyncPeriod)
		for {
			// Refresh first, so it happens when start is called.
			f.Refresh()
			<-t
		}
	}()
}

// Refresh updates the LastCount with fresh data.  Any errors are logged.
func (f *FuzzSyncer) Refresh() {
	sklog.Info("Counting fuzzes (high, medium, low severity)")
	currRevision := config.Common.SkiaVersion.Hash

	allBadFuzzes := util.NewStringSet()

	for _, cat := range common.FUZZ_CATEGORIES {
		currentBadNames := util.NewStringSet()
		for _, a := range common.ARCHITECTURES {
			// always fetch current counts
			currentBadNames = currentBadNames.Union(f.getFuzzNames("bad", cat, a, currRevision))
		}

		allBadFuzzes = allBadFuzzes.Union(currentBadNames)
	}

	if err := f.updateLoadedBinaryFuzzes(allBadFuzzes.Keys()); err != nil {
		sklog.Errorf("Problem updating loaded binary fuzzes: %s", err)
	}

	if f.gcsLoader == nil || f.gcsLoader.Pool == nil {
		sklog.Infof("Skipping summary updates because pool not ready")
		return
	}
	f.countMutex.Lock()
	defer f.countMutex.Unlock()
	counts := map[string]FuzzCount{}
	for _, f := range f.gcsLoader.Pool.Reports() {
		if f.IsGrey {
			continue
		}
		c, ok := counts[f.FuzzCategory]
		if !ok {
			c = FuzzCount{}
		}
		hi, med := false, false
		for _, flags := range f.Flags {
			xf := util.NewStringSet(flags)
			if len(HIGH_PRIORITY_FLAGS.Intersect(xf)) > 0 {
				hi = true
			} else if len(MEDIUM_PRIORITY_FLAGS.Intersect(xf)) > 0 {
				med = true
			}
		}

		if hi {
			c.HighPriority++
		} else if med {
			c.MedPriority++
		} else {
			c.LowPriority++
		}
		counts[f.FuzzCategory] = c
	}
	f.lastCount = counts
}

// getMostRecentOldRevision finds the most recently updated revision used.
// It searches the GCS bucket under skia_version/old/  An error is returned if there is one.
func (f *FuzzSyncer) getMostRecentOldRevision() (string, error) {
	var newestTime time.Time
	newestHash := ""
	findNewest := func(item *storage.ObjectAttrs) {
		sklog.Infof("%s: %s", item.Name, item.Updated)
		if newestTime.Before(item.Updated) {
			newestTime = item.Updated
			newestHash = item.Name[strings.LastIndex(item.Name, "/")+1:]
		}
	}
	if err := gcs.AllFilesInDir(f.storageClient, config.GCS.Bucket, "skia_version/old/", findNewest); err != nil {
		return "", err
	}
	sklog.Infof("Most recent old version found to be %s", newestHash)
	return newestHash, nil
}

// getOrLookUpFuzzNames first checks the cache for a StringSet of fuzz names linked to a given
// tuple of fuzzType (bad or grey), category, revision.  If such a thing is not in the cache, it
// fetches it via getFuzzNames() and caches it for next time. The cache occasionally empties
// itself to avoid staleness (for example, after a version update).
func (f *FuzzSyncer) getOrLookUpFuzzNames(fuzzType, category, architecture, revision string) util.StringSet {
	key := strings.Join([]string{fuzzType, category, architecture, revision}, "|")
	// 1% of the time, we purge the cache
	cachePurge := rand.Float32() > 0.99
	if cachePurge {
		sklog.Info("Purging the cached fuzz names")
		f.fuzzNameCache = make(map[string]util.StringSet)
	}
	if s, has := f.fuzzNameCache[key]; has {
		return s
	}
	s := f.getFuzzNames(fuzzType, category, architecture, revision)
	// cache it
	f.fuzzNameCache[key] = s
	return s
}

// getFuzzNames gets all the fuzz names belonging to a fuzzType, category, revision tuple from
// Google Storage.  It tries two different ways to do this, first by reading a
// (bad|grey)_fuzz_names file, which exists in previous revisions.  Second, it manually counts
// all fuzzes in the given GCS folder.
func (f *FuzzSyncer) getFuzzNames(fuzzType, category, architecture, revision string) util.StringSet {
	// Sometimes fuzzes with the name "" show up, and we don't want that.
	emptyString := util.NewStringSet([]string{""})
	// The file stored, if it exists, is a pipe separated list.
	if names, err := gcs.FileContentsFromGCS(f.storageClient, config.GCS.Bucket, fmt.Sprintf("%s/%s/%s/%s_fuzz_names.txt", category, revision, architecture, fuzzType)); err == nil {
		return util.NewStringSet(strings.Split(string(names), "|")).Complement(emptyString)
	} else {
		sklog.Infof("Could not find cached names, downloading them the long way, instead: %s", err)
	}

	if names, err := common.GetAllFuzzNamesInFolder(f.storageClient, fmt.Sprintf("%s/%s/%s/%s", category, revision, architecture, fuzzType)); err != nil {
		sklog.Errorf("Problem fetching %s %s %s fuzzes at revision %s: %s", fuzzType, architecture, category, revision, err)
		return nil
	} else {
		return util.NewStringSet(names).Complement(emptyString)
	}
}

// updateLoadedBinaryFuzzes uses gcsLoader to download the fuzzes that are currently not
// in the fuzz report tree / cache.
func (f *FuzzSyncer) updateLoadedBinaryFuzzes(currentBadFuzzHashes []string) error {
	if f.gcsLoader == nil {
		sklog.Info("Skipping update because the cache hasn't been set yet")
		return nil
	}
	prevBadFuzzNames, err := f.gcsLoader.Cache.LoadFuzzNames(config.Common.SkiaVersion.Hash)
	if err != nil {
		return fmt.Errorf("Could not load previous fuzz hashes from cache at revision %s: %s", config.Common.SkiaVersion.Hash, err)
	}
	sort.Strings(currentBadFuzzHashes)
	sort.Strings(prevBadFuzzNames)

	newBinaryFuzzNames := make([]string, 0, 10)
	for _, h := range currentBadFuzzHashes {
		if i := sort.SearchStrings(prevBadFuzzNames, h); i < len(prevBadFuzzNames) && prevBadFuzzNames[i] == h {
			continue
		}
		newBinaryFuzzNames = append(newBinaryFuzzNames, h)
	}

	sklog.Infof("%d newly found fuzzes from Google Storage.  Going to load them.", len(newBinaryFuzzNames))
	if len(newBinaryFuzzNames) > 0 {
		return f.gcsLoader.LoadFuzzesFromGoogleStorage(newBinaryFuzzNames)
	}
	return nil
}

func (f *FuzzSyncer) LastCount(category string) FuzzCount {
	return f.lastCount[category]
}
