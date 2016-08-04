package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/buildskia"
	"golang.org/x/net/context"
)

// DownloadSkiaVersionForFuzzing downloads the revision of Skia specified in Google Storage
// to the given path. On sucess, the given VersionSetter is set to be the current revision.
// It returns the revision it found in GCS and any errors.
func DownloadSkiaVersionForFuzzing(storageClient *storage.Client, path string, v config.VersionSetter, clean bool) error {
	skiaVersion, _, err := GetCurrentSkiaVersionFromGCS(storageClient)
	if err != nil {
		return fmt.Errorf("Could not get Skia revision from GCS: %s", err)
	}
	if err := DownloadSkia(skiaVersion, path, v, clean); err != nil {
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
func GetCurrentSkiaVersionFromGCS(storageClient *storage.Client) (string, time.Time, error) {
	return revisionHelper(storageClient, "skia_version/current/")
}

// GetPendingSkiaVersionFromGCS checks the skia_version folder in the fuzzer bucket for a single
// file that has the pending revision to be used for fuzzing (typically a dep roll). It returns the
// revision, the last time the revision was set, and any error. If there is no pending revision,
// empty string, zero time.Time and nil error are returned.
func GetPendingSkiaVersionFromGCS(storageClient *storage.Client) (string, time.Time, error) {
	// We ignore errors about not finding any pending revisions
	if revision, date, err := revisionHelper(storageClient, "skia_version/pending/"); err == nil || strings.HasPrefix(err.Error(), "Could not find specified revision") {
		return revision, date, nil
	} else {
		return revision, date, err
	}
}

// revisionHelper actually goes and gets the revision files from GCS and parses them. It returns the
// revision, the last time the revision was set, and any error.
func revisionHelper(storageClient *storage.Client, prefix string) (string, time.Time, error) {
	if storageClient == nil {
		return "", time.Time{}, fmt.Errorf("Storage service cannot be nil!")
	}
	q := &storage.Query{Prefix: prefix}
	contents, err := storageClient.Bucket(config.GS.Bucket).List(context.Background(), q)
	if err != nil {
		return "", time.Time{}, err
	}
	for _, r := range contents.Results {
		if r.Name != prefix {
			return strings.SplitAfter(r.Name, prefix)[1], r.Updated, nil
		}
	}
	return "", time.Time{}, fmt.Errorf("Could not find specified revision in %q", prefix)
}

// downloadSkia uses git to clone Skia from googlesource.com and check it out to the specified
// revision. Upon sucess, the SkiaVersion in config is set to be the current revision and any
// dependencies needed to compile Skia have been installed (e.g. the latest revision of gyp).
// It returns an error on failure.
func DownloadSkia(revision, path string, v config.VersionSetter, clean bool) error {
	if lc, err := buildskia.DownloadSkia("master", revision, path, config.Common.DepotToolsPath, clean, false); err != nil {
		return fmt.Errorf("Could not get git details for skia revision %s: %s", revision, err)
	} else {
		v.SetSkiaVersion(lc)
		return nil
	}
}
