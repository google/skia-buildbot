package download_skia

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/config"
	fstorage "go.skia.org/infra/fuzzer/go/storage"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/sklog"
)

// AtGCSRevision downloads the revision of Skia specified in Google Storage
// to the given path. On sucess, the given VersionSetter is set to be the current revision.
// It returns the revision it found in GCS and any errors.
func AtGCSRevision(ctx context.Context, storageClient fstorage.FuzzerGCSClient, path string, v config.VersionSetter, clean bool) error {
	skiaVersion, _, err := GetCurrentSkiaVersionFromGCS(storageClient)
	if err != nil {
		return fmt.Errorf("Could not get Skia revision from GCS: %s", err)
	}
	if err := AtRevision(ctx, skiaVersion, path, v, clean); err != nil {
		return fmt.Errorf("Problem downloading skia: %s", err)
	}
	// Always clean out the build directory, to mitigate potential build
	// problems
	buildDir := filepath.Join(path, "out")
	if err := os.RemoveAll(buildDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to clean out Skia build dir %s: %s", buildDir, err)
	}
	return nil
}

// GetCurrentSkiaVersionFromGCS checks the skia_version folder in the fuzzer bucket for a single
// file that has the current revision to be used for fuzzing (typically a dep roll). It returns the
// revision, the last time the revision was set, and any error.
func GetCurrentSkiaVersionFromGCS(storageClient fstorage.FuzzerGCSClient) (string, time.Time, error) {
	return revisionHelper(storageClient, "skia_version/current/")
}

// GetPendingSkiaVersionFromGCS checks the skia_version folder in the fuzzer bucket for a single
// file that has the pending revision to be used for fuzzing (typically a dep roll). It returns the
// revision, the last time the revision was set, and any error. If there is no pending revision,
// empty string, zero time.Time and nil error are returned.
func GetPendingSkiaVersionFromGCS(storageClient fstorage.FuzzerGCSClient) (string, time.Time, error) {
	// We ignore errors about not finding any pending revisions
	if revision, date, err := revisionHelper(storageClient, "skia_version/pending/"); err == nil || strings.HasPrefix(err.Error(), "Could not find specified revision") {
		return revision, date, nil
	} else {
		return revision, date, err
	}
}

var gitRevision = regexp.MustCompile("[0-9a-f]{40}")

// revisionHelper actually goes and gets the revision files from GCS and parses them. It returns the
// revision, the last time the revision was set, and any error.
func revisionHelper(storageClient fstorage.FuzzerGCSClient, prefix string) (string, time.Time, error) {
	if storageClient == nil {
		return "", time.Time{}, fmt.Errorf("Storage service cannot be nil!")
	}
	rev := ""
	ts := time.Time{}
	if err := storageClient.AllFilesInDirectory(context.Background(), prefix, func(item *storage.ObjectAttrs) {
		name := strings.SplitAfter(item.Name, prefix)[1]
		if rev == "" && gitRevision.MatchString(name) {
			rev = name
			ts = item.Updated
		} else if gitRevision.MatchString(item.Name) {
			sklog.Warningf("Found two (or more) potential git revisions in %s. newly saw %s, but sticking with %s", prefix, name, rev)
		}
	}); err != nil {
		return "", time.Time{}, err
	}

	if rev == "" {
		return "", time.Time{}, fmt.Errorf("Could not find specified revision in %q", prefix)
	}
	return rev, ts, nil
}

// AtRevision uses git to clone Skia from googlesource.com and check it out to the specified
// revision. Upon sucess, the SkiaVersion in config is set to be the current revision and any
// dependencies needed to compile Skia have been installed (e.g. the latest revision of gyp).
// It returns an error on failure.
func AtRevision(ctx context.Context, revision, path string, v config.VersionSetter, clean bool) error {
	if lc, err := buildskia.GNDownloadSkia(ctx, "master", revision, path, config.Common.DepotToolsPath, clean, false); err != nil {
		return fmt.Errorf("Could not buildskia.GNDownloadSkia for skia revision %s: %s", revision, err)
	} else {
		v.SetSkiaVersion(lc)
		return nil
	}
}
