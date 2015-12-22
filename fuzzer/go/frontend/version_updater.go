package frontend

import (
	"fmt"
	"sync"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/functionnamefinder"
	"go.skia.org/infra/fuzzer/go/fuzzcache"
	"go.skia.org/infra/go/vcsinfo"
	"google.golang.org/cloud/storage"
)

// VersionUpdater is a struct that will handle the updating from one version to fuzz to another
// for the frontend.
// It will handle both a pending change and a current change.
type VersionUpdater struct {
	storageClient  *storage.Client
	finders        map[string]functionnamefinder.Finder
	finderBuilding sync.Mutex
	cache          fuzzcache.FuzzReportCache
}

// NewVersionUpdater returns a VersionUpdater.
func NewVersionUpdater(s *storage.Client, c fuzzcache.FuzzReportCache) *VersionUpdater {
	return &VersionUpdater{
		storageClient: s,
		finders:       make(map[string]functionnamefinder.Finder),
		cache:         c,
	}
}

// versionHolder is a small struct to be passed to DownloadSkia to get the
// version details.
type versionHolder struct {
	version *vcsinfo.LongCommit
}

// SetSkiaVersion stores the long commit given.
func (v *versionHolder) SetSkiaVersion(lc *vcsinfo.LongCommit) {
	v.version = lc
}

// HandlePendingVersion updates the frontend's copy of Skia to the specified pending version
// and begins building the AST for the pending version on a background goroutine.
func (u *VersionUpdater) HandlePendingVersion(pendingHash string) (*vcsinfo.LongCommit, error) {
	pending := versionHolder{}
	if err := common.DownloadSkia(pendingHash, config.FrontEnd.SkiaRoot, &pending); err != nil {
		return nil, fmt.Errorf("Could not update Skia to pending version %s: %s", pendingHash, err)
	}

	// start generating AST in the background.
	go func() {
		u.finderBuilding.Lock()
		defer u.finderBuilding.Unlock()
		if finder, err := functionnamefinder.New(); err != nil {
			glog.Errorf("Error building FunctionNameFinder at version %s: %s", pendingHash, err)
			return
		} else {
			glog.Infof("Successfully rebuilt AST for Skia version %s", pendingHash)
			u.finders[pendingHash] = finder
		}
	}()

	return pending.version, nil
}

// HandleCurrentVersion sets the current version of Skia to be the specified value and calls
// LoadFromGoogleStorage.  If there is an ast still being generated, it will block until
// that completes.
func (u *VersionUpdater) HandleCurrentVersion(currentHash string) (*vcsinfo.LongCommit, error) {
	// Make sure skia version is at the proper version.  This also sets config.Frontend.SkiaVersion.
	if err := common.DownloadSkia(currentHash, config.FrontEnd.SkiaRoot, &config.FrontEnd); err != nil {
		return nil, fmt.Errorf("Could not update Skia to current version %s: %s", currentHash, err)
	}
	// Block until finder is built.
	u.finderBuilding.Lock()
	defer u.finderBuilding.Unlock()
	if err := LoadFromGoogleStorage(u.storageClient, u.finders[currentHash], u.cache); err != nil {
		return nil, fmt.Errorf("Had problems fetching new fuzzes from GCS: %s", err)
	}
	return config.FrontEnd.SkiaVersion, nil
}
