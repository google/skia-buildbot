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
	LastCount FuzzCount

	countMutex    sync.Mutex
	storageClient *storage.Client
	gsLoader      *gsloader.GSLoader
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
	currentBadFuzzHashes, err := common.GetAllFuzzNamesInFolder(f.storageClient, fmt.Sprintf("binary_fuzzes/%s/bad/", currRevision))
	if err != nil {
		glog.Errorf("Problem getting total bad fuzz counts: %s", err)
	} else {
		f.LastCount.TotalBad = len(currentBadFuzzHashes)
	}

	if prevBadFuzzHashes, err := common.GetAllFuzzNamesInFolder(f.storageClient, fmt.Sprintf("binary_fuzzes/%s/bad/", prevRevision)); err != nil {
		glog.Errorf("Problem getting this bad fuzz counts: %s", err)
	} else {
		f.LastCount.ThisBad = f.LastCount.TotalBad - len(prevBadFuzzHashes)
	}

	// We don't need to mess with grey fuzzes here because those only change when the Skia
	// revision changes.
	if greyFuzzHashes, err := common.GetAllFuzzNamesInFolder(f.storageClient, fmt.Sprintf("binary_fuzzes/%s/grey/", currRevision)); err != nil {
		glog.Errorf("Problem getting total grey fuzz counts: %s", err)
	} else {
		f.LastCount.TotalGrey = len(greyFuzzHashes)
	}

	if greyFuzzHashes, err := common.GetAllFuzzNamesInFolder(f.storageClient, fmt.Sprintf("binary_fuzzes/%s/grey/", prevRevision)); err != nil {
		glog.Errorf("Problem getting this grey fuzz counts: %s", err)
	} else {
		f.LastCount.ThisGrey = f.LastCount.TotalGrey - len(greyFuzzHashes)
	}
	if err = f.updateLoadedBinaryFuzzes(currentBadFuzzHashes); err != nil {
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
// in the
func (f *FuzzSyncer) updateLoadedBinaryFuzzes(currentBadFuzzHashes []string) error {
	if f.gsLoader == nil {
		glog.Info("Skipping update because the finder/cache hasn't been set yet")
		return nil
	}
	_, prevBadFuzzHashes, err := f.gsLoader.Cache.Load(config.FrontEnd.SkiaVersion.Hash)
	if err != nil {
		return fmt.Errorf("Could not load previous fuzz hashes from cache at revision %s: %s", config.FrontEnd.SkiaVersion.Hash, err)
	}
	sort.Strings(currentBadFuzzHashes)
	sort.Strings(prevBadFuzzHashes)

	newBinaryFuzzNames := make([]string, 0, len(currentBadFuzzHashes)-len(prevBadFuzzHashes))
	for _, h := range currentBadFuzzHashes {
		if i := sort.SearchStrings(prevBadFuzzHashes, h); i < len(prevBadFuzzHashes) && prevBadFuzzHashes[i] == h {
			continue
		}
		newBinaryFuzzNames = append(newBinaryFuzzNames, h)
	}

	glog.Infof("%d newly found fuzzes from Google Storage.  Going to load them.", len(newBinaryFuzzNames))
	return f.gsLoader.LoadBinaryFuzzesFromGoogleStorage(newBinaryFuzzNames)
}
