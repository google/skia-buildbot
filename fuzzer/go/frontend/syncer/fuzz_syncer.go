package syncer

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/frontend/gsloader"
	"go.skia.org/infra/go/gs"
	"google.golang.org/cloud/storage"
)

// FuzzSyncer is a struct that will handle the syncing of bad/grey fuzzes.
// Once started, it will occasionally wake up and download any new fuzzes
// Clients should look at LastCount for the last count of fuzzes data.
type FuzzSyncer struct {
	countMutex    sync.Mutex
	storageClient *storage.Client
	gsLoader      *gsloader.GSLoader
	lastCount     map[string]FuzzCount
}

// FuzzCount is a struct that holds the counts of fuzzes.
type FuzzCount struct {
	TotalBad  int `json:"totalBadCount"`
	TotalGrey int `json:"totalGreyCount"`
	// "This" means "newly introduced/fixed in this revision"
	ThisBad  int `json:"thisBadCount"`
	ThisGrey int `json:"thisGreyCount"`
}

// NewFuzzSyncer creates a FuzzSyncer and returns it.
func New(s *storage.Client) *FuzzSyncer {
	return &FuzzSyncer{
		storageClient: s,
		lastCount:     make(map[string]FuzzCount),
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
	glog.Info("Counting bad and grey fuzzes")
	currRevision := config.FrontEnd.SkiaVersion.Hash
	prevRevision, err := f.getMostRecentOldRevision()
	if err != nil {
		glog.Infof("Problem getting most recent old version: %s", err)
		return
	}
	f.countMutex.Lock()
	defer f.countMutex.Unlock()
	allCurrentNames := []string{}

	for _, cat := range common.FUZZ_CATEGORIES {
		lastCount := FuzzCount{}
		currentBadNames, err := common.GetAllFuzzNamesInFolder(f.storageClient, fmt.Sprintf("%s/%s/bad", cat, currRevision))
		if err != nil {
			glog.Errorf("Problem getting total bad fuzz counts: %s", err)
		} else {
			lastCount.TotalBad = len(currentBadNames)
			allCurrentNames = append(allCurrentNames, currentBadNames...)
		}

		if previousBadNames, err := common.GetAllFuzzNamesInFolder(f.storageClient, fmt.Sprintf("%s/%s/bad", cat, prevRevision)); err != nil {
			glog.Errorf("Problem getting this bad fuzz counts: %s", err)
		} else {
			lastCount.ThisBad = lastCount.TotalBad - len(previousBadNames)
		}

		if currentGreyNames, err := common.GetAllFuzzNamesInFolder(f.storageClient, fmt.Sprintf("%s/%s/grey", cat, currRevision)); err != nil {
			glog.Errorf("Problem getting total grey fuzz counts: %s", err)
		} else {
			lastCount.TotalGrey = len(currentGreyNames)
		}

		if previousGreyNames, err := common.GetAllFuzzNamesInFolder(f.storageClient, fmt.Sprintf("%s/%s/grey", cat, prevRevision)); err != nil {
			glog.Errorf("Problem getting this grey fuzz counts: %s", err)
		} else {
			lastCount.ThisGrey = lastCount.TotalGrey - len(previousGreyNames)
		}
		f.lastCount[cat] = lastCount
	}

	if err = f.updateLoadedBinaryFuzzes(allCurrentNames); err != nil {
		glog.Errorf("Problem updating loaded binary fuzzes: %s", err)
	}
}

// getMostRecentOldRevision finds the most recently updated revision used.
// It searches the GS bucket under skia_version/old/  An error is returned if there is one.
func (f *FuzzSyncer) getMostRecentOldRevision() (string, error) {
	var newestTime time.Time
	newestHash := ""
	findNewest := func(item *storage.ObjectAttrs) {
		glog.Infof("%s: %s", item.Name, item.Updated)
		if newestTime.Before(item.Updated) {
			newestTime = item.Updated
			newestHash = item.Name[strings.LastIndex(item.Name, "/")+1:]
		}
	}
	if err := gs.AllFilesInDir(f.storageClient, config.GS.Bucket, "skia_version/old/", findNewest); err != nil {
		return "", err
	}
	glog.Infof("Most recent old version found to be %s", newestHash)
	return newestHash, nil
}

// updateLoadedBinaryFuzzes uses gsLoader to download the fuzzes that are currently not
// in the fuzz report tree / cache.
func (f *FuzzSyncer) updateLoadedBinaryFuzzes(currentBadFuzzHashes []string) error {
	if f.gsLoader == nil {
		glog.Info("Skipping update because the cache hasn't been set yet")
		return nil
	}
	prevBadFuzzNames, err := f.gsLoader.Cache.LoadFuzzNames(config.FrontEnd.SkiaVersion.Hash)
	if err != nil {
		return fmt.Errorf("Could not load previous fuzz hashes from cache at revision %s: %s", config.FrontEnd.SkiaVersion.Hash, err)
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

	glog.Infof("%d newly found fuzzes from Google Storage.  Going to load them.", len(newBinaryFuzzNames))
	if len(newBinaryFuzzNames) > 0 {
		return f.gsLoader.LoadBinaryFuzzesFromGoogleStorage(newBinaryFuzzNames)
	}
	return nil
}

func (f *FuzzSyncer) LastCount(category string) FuzzCount {
	return f.lastCount[category]
}
