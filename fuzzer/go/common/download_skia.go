package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
	"google.golang.org/cloud/storage"
)

// DownloadSkiaVersionForFuzzing downloads the version of Skia specified in Google Storage
// to the given path. On sucess, the given VersionSetter is set to be the current version.
// It returns the version it found in GCS and any errors.
func DownloadSkiaVersionForFuzzing(storageClient *storage.Client, path string, v config.VersionSetter) error {
	skiaVersion, err := GetCurrentSkiaVersionFromGCS(storageClient)
	if err != nil {
		return fmt.Errorf("Could not get Skia version from GCS: %s", err)
	}
	if err := DownloadSkia(skiaVersion, path, v, true); err != nil {
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

// GetCurrentSkiaVersionFromGCS checks the skia_version folder in the fuzzer bucket for a single file
// that has the current version to be used for fuzzing (typically a dep roll).  It returns the version
// or an error if there was a failure.
func GetCurrentSkiaVersionFromGCS(storageClient *storage.Client) (string, error) {
	return versionHelper(storageClient, "skia_version/current/")
}

// GetPendingSkiaVersionFromGCS checks the skia_version folder in the fuzzer bucket for a single file
// that has the pending version to be used for fuzzing (typically a dep roll).  It returns the version
// or an error if there was a failure.  If there is no pending version, empty string and error
// are returned.
func GetPendingSkiaVersionFromGCS(storageClient *storage.Client) (string, error) {
	// We ignore errors about not finding any pending versions
	if version, err := versionHelper(storageClient, "skia_version/pending/"); err == nil || strings.HasPrefix(err.Error(), "Could not find specified version") {
		return version, nil
	} else {
		return version, err
	}
}

// versionHelper actually goes and gets the version files from GCS and parses them
func versionHelper(storageClient *storage.Client, prefix string) (string, error) {
	if storageClient == nil {
		return "", fmt.Errorf("Storage service cannot be nil!")
	}
	q := &storage.Query{Prefix: prefix}
	contents, err := storageClient.Bucket(config.GS.Bucket).List(context.Background(), q)
	if err != nil {
		return "", err
	}
	for _, r := range contents.Results {
		if r.Name != prefix {
			return strings.SplitAfter(r.Name, prefix)[1], nil
		}
	}
	return "", fmt.Errorf("Could not find specified version in %q", prefix)
}

// downloadSkia uses git to clone Skia from googlesource.com and check it out to the specified version.
// Upon sucess, the SkiaVersion in config is set to be the current version and any dependencies
// needed to compile Skia have been installed (e.g. the latest version of gyp).
// It returns an error on failure.
func DownloadSkia(version, path string, v config.VersionSetter, clean bool) error {
	glog.Infof("Cloning Skia version %s to %s, clean: %t", version, path, clean)

	if clean {
		// The third_party folder can cause bin/sync-and-gyp to fail.  Clean builds
		// delete everything, just to make sure.
		util.RemoveAll(filepath.Join(path))
	}

	repo, err := gitinfo.CloneOrUpdate("https://skia.googlesource.com/skia", path, false)
	if err != nil {
		return fmt.Errorf("Failed cloning Skia: %s", err)
	}

	if err = repo.SetToCommit(version); err != nil {
		return fmt.Errorf("Problem setting Skia to version %s: %s", version, err)
	}

	//  as of skia@2362c476ef4, we need gclient on the path to run sync-and-gyp
	syncCmd := &exec.Command{
		Name: "bin/sync-and-gyp",
		Dir:  path,
		// This is a bit of a hack because we need to expand the os path (which has python on it)
		Env: []string{"PATH=" + config.Common.DepotToolsPath + ":" + os.Getenv("PATH")},
	}

	if err := exec.Run(syncCmd); err != nil {
		return fmt.Errorf("Failed syncing and setting up gyp: %s", err)
	}

	if v != nil {
		if lc, err := repo.Details(version, false); err != nil {
			glog.Errorf("Could not get git details for skia version %s: %s", version, err)
		} else {
			v.SetSkiaVersion(lc)
		}
	}
	return nil
}
