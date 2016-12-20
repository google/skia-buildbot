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
	"go.skia.org/infra/fuzzer/go/frontend/gsloader"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// FuzzSyncer is a struct that will fetch the bad/grey fuzzes from GCS at discrete intervals.
// Once started, it will occasionally wake up and download any new fuzzes from GCS using the
// supplied gsloader. It looks at both the current revision and the previous revision to count
//regressions.  Clients should look at LastCount for the most recent result of the counts.
type FuzzSyncer struct {
	countMutex    sync.Mutex
	storageClient *storage.Client
	gsLoader      *gsloader.GSLoader
	lastCount     map[string]FuzzCount      // maps category->FuzzCount
	fuzzNameCache map[string]util.StringSet // maps key->FuzzNames
}

// FuzzCount is a struct that holds the counts of fuzzes across all architectures.
type FuzzCount struct {
	TotalBad  int `json:"totalBadCount"`
	TotalGrey int `json:"totalGreyCount"`
	// "This" means "newly introduced/fixed in this revision"
	ThisBad        int `json:"thisBadCount"`
	ThisRegression int `json:"thisRegressionCount"`
}

// NewFuzzSyncer creates a FuzzSyncer and returns it.
func New(s *storage.Client) *FuzzSyncer {
	return &FuzzSyncer{
		storageClient: s,
		lastCount:     make(map[string]FuzzCount),
		fuzzNameCache: make(map[string]util.StringSet),
	}
}

// SetGSLoader sets this objects GSLoader, allowing it to fetch the new fuzzes it finds
// and update the fuzzes displayed to users.
func (f *FuzzSyncer) SetGSLoader(g *gsloader.GSLoader) {
	f.gsLoader = g
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
	sklog.Info("Counting bad and grey fuzzes")
	currRevision := config.Common.SkiaVersion.Hash
	prevRevision, err := f.getMostRecentOldRevision()
	if err != nil {
		sklog.Infof("Problem getting most recent old version: %s", err)
		return
	}
	f.countMutex.Lock()
	defer f.countMutex.Unlock()
	allBadFuzzes := util.NewStringSet()

	for _, cat := range common.FUZZ_CATEGORIES {
		lastCount := FuzzCount{}

		previousGreyNames := util.NewStringSet()
		previousBadNames := util.NewStringSet()
		currentGreyNames := util.NewStringSet()
		currentBadNames := util.NewStringSet()
		for _, a := range common.ARCHITECTURES {
			// Previous fuzzes and current grey fuzzes can be drawn from the cache, if they aren't there.
			previousGreyNames = previousGreyNames.Union(f.getOrLookUpFuzzNames("grey", cat, a, prevRevision))
			previousBadNames = previousBadNames.Union(f.getOrLookUpFuzzNames("bad", cat, a, prevRevision))
			currentGreyNames = currentGreyNames.Union(f.getOrLookUpFuzzNames("grey", cat, a, currRevision))
			// always fetch current counts
			currentBadNames = currentBadNames.Union(f.getFuzzNames("bad", cat, a, currRevision))
		}

		lastCount.TotalBad = len(currentBadNames)
		lastCount.TotalGrey = len(currentGreyNames)
		lastCount.ThisBad = len(currentBadNames.Complement(previousBadNames).Complement(previousGreyNames))
		lastCount.ThisRegression = len(previousGreyNames.Intersect(currentBadNames))
		allBadFuzzes = allBadFuzzes.Union(currentBadNames)

		f.lastCount[cat] = lastCount
	}

	if err = f.updateLoadedBinaryFuzzes(allBadFuzzes.Keys()); err != nil {
		sklog.Errorf("Problem updating loaded binary fuzzes: %s", err)
	}
}

// getMostRecentOldRevision finds the most recently updated revision used.
// It searches the GS bucket under skia_version/old/  An error is returned if there is one.
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
	if err := gs.AllFilesInDir(f.storageClient, config.GS.Bucket, "skia_version/old/", findNewest); err != nil {
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
	// 5% of the time, we purge the cache
	cachePurge := rand.Float32() > 0.95
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
	if names, err := gs.FileContentsFromGS(f.storageClient, config.GS.Bucket, fmt.Sprintf("%s/%s/%s/%s_fuzz_names.txt", category, revision, architecture, fuzzType)); err == nil {
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

// updateLoadedBinaryFuzzes uses gsLoader to download the fuzzes that are currently not
// in the fuzz report tree / cache.
func (f *FuzzSyncer) updateLoadedBinaryFuzzes(currentBadFuzzHashes []string) error {
	if f.gsLoader == nil {
		sklog.Info("Skipping update because the cache hasn't been set yet")
		return nil
	}
	prevBadFuzzNames, err := f.gsLoader.Cache.LoadFuzzNames(config.Common.SkiaVersion.Hash)
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
		return f.gsLoader.LoadFuzzesFromGoogleStorage(newBinaryFuzzNames)
	}
	return nil
}

func (f *FuzzSyncer) LastCount(category string) FuzzCount {
	return f.lastCount[category]
}
