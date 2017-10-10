package frontend

import (
	"context"
	"fmt"

	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/download_skia"
	"go.skia.org/infra/fuzzer/go/frontend/gcsloader"
	"go.skia.org/infra/fuzzer/go/frontend/syncer"
	"go.skia.org/infra/fuzzer/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
)

// VersionUpdater is a struct that will handle the updating from one version to fuzz to another
// for the frontend.
// It will handle both a pending change and a current change.
type VersionUpdater struct {
	gcsLoader *gcsloader.GCSLoader
	syncer    *syncer.FuzzSyncer
}

// NewVersionUpdater returns a VersionUpdater.
func NewVersionUpdater(g *gcsloader.GCSLoader, syncer *syncer.FuzzSyncer) *VersionUpdater {
	return &VersionUpdater{
		gcsLoader: g,
		syncer:    syncer,
	}
}

// HandleCurrentVersion sets the current version of Skia to be the specified value and calls
// LoadFreshFromGoogleStorage.
func (v *VersionUpdater) HandleCurrentVersion(currentHash string) error {
	// Make sure skia version is at the proper version.  This also sets config.Common.SkiaVersion.
	if err := download_skia.AtRevision(currentHash, config.Common.SkiaRoot, &config.Common, false); err != nil {
		return fmt.Errorf("Could not update Skia to current version %s: %s", currentHash, err)
	}
	if err := v.gcsLoader.LoadFreshFromGoogleStorage(); err != nil {
		return fmt.Errorf("Had problems fetching new fuzzes from GCS: %s", err)
	}
	v.syncer.Refresh()
	return nil
}

// UpdateVersionToFuzz creates a pending version file and then a work files for each of the
// backends. When the fuzzer backends finish their roll duties, they will remove their
// respective "working" files, indicating they are done.
func UpdateVersionToFuzz(storageClient storage.FuzzerGCSClient, backendWorkers []string, version string) error {
	newVersionFile := fmt.Sprintf("skia_version/pending/%s", version)
	if err := storageClient.SetFileContents(context.Background(), newVersionFile, gcs.FILE_WRITE_OPTS_TEXT, []byte(version)); err != nil {
		return fmt.Errorf("Could not set pending version: %s", err)
	}
	for _, bw := range backendWorkers {
		workFile := fmt.Sprintf("skia_version/pending/working_%s", bw)
		if err := storageClient.SetFileContents(context.Background(), workFile, gcs.FILE_WRITE_OPTS_TEXT, []byte(workFile)); err != nil {
			sklog.Warningf("Error writing to %s to signal worker %s. Continuing anyway. %s", workFile, bw, err)
		}
	}

	sklog.Infof("%s has been made and %d backend workers notified. They will pick up the change and start working.\n", newVersionFile, len(backendWorkers))
	return nil
}
