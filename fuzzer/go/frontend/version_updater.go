package frontend

import (
	"fmt"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/frontend/gsloader"
	"go.skia.org/infra/fuzzer/go/frontend/syncer"
	"golang.org/x/net/context"
	"google.golang.org/cloud/storage"
)

// VersionUpdater is a struct that will handle the updating from one version to fuzz to another
// for the frontend.
// It will handle both a pending change and a current change.
type VersionUpdater struct {
	gsLoader *gsloader.GSLoader
	syncer   *syncer.FuzzSyncer
}

// NewVersionUpdater returns a VersionUpdater.
func NewVersionUpdater(g *gsloader.GSLoader, syncer *syncer.FuzzSyncer) *VersionUpdater {
	return &VersionUpdater{
		gsLoader: g,
		syncer:   syncer,
	}
}

// HandleCurrentVersion sets the current version of Skia to be the specified value and calls
// LoadFreshFromGoogleStorage.
func (v *VersionUpdater) HandleCurrentVersion(currentHash string) error {
	// Make sure skia version is at the proper version.  This also sets config.Common.SkiaVersion.
	if err := common.DownloadSkia(currentHash, config.Common.SkiaRoot, &config.Common, false); err != nil {
		return fmt.Errorf("Could not update Skia to current version %s: %s", currentHash, err)
	}
	if err := v.gsLoader.LoadFreshFromGoogleStorage(); err != nil {
		return fmt.Errorf("Had problems fetching new fuzzes from GCS: %s", err)
	}
	v.syncer.Refresh()
	return nil
}

func UpdateVersionToFuzz(storageClient *storage.Client, bucket, version string) error {
	newVersionFile := fmt.Sprintf("skia_version/pending/%s", version)
	w := storageClient.Bucket(bucket).Object(newVersionFile).NewWriter(context.Background())
	if err := w.Close(); err != nil {
		return fmt.Errorf("Could not create version file %s : %s", newVersionFile, err)
	}
	glog.Infof("%s has been made.  The backend and frontend will eventually pick up this change (in that order).\n", newVersionFile)
	return nil
}
