package common

import (
	"fmt"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"golang.org/x/net/context"
	"google.golang.org/cloud/storage"
)

// DownloadSkiaVersionForFuzzing downloads the version of Skia specified in Google Storage
// to the given path.  It returns an error on failure.
func DownloadSkiaVersionForFuzzing(storageClient *storage.Client, path string) error {
	skiaVersion, err := getSkiaVersionFromGCS(storageClient)
	if err != nil {
		return fmt.Errorf("Could not get Skia version from GCS: %s", err)
	}

	return downloadSkia(skiaVersion, path)
}

// getSkiaVersionFromGCS checks the skia_version folder in the fuzzer bucket for a single file
// that has the version to be used for fuzzing (typically a dep roll).  It returns the version
// or an error if there was a failure.
func getSkiaVersionFromGCS(storageClient *storage.Client) (string, error) {
	if storageClient == nil {
		return "", fmt.Errorf("Storage service cannot be nil!")
	}
	q := &storage.Query{Prefix: "skia_version/current"}
	contents, err := storageClient.Bucket(config.GS.Bucket).List(context.Background(), q)
	if err != nil {
		return "", err
	}
	if len(contents.Results) < 2 {
		return "", fmt.Errorf("version file not found")
	}
	// item[0] is the folder skia_version/, the file name of item[1] is the current version to fuzz
	file := contents.Results[1].Name
	return strings.SplitAfter(file, "skia_version/current/")[1], nil
}

// downloadSkia uses git to clone Skia from googlesource.com and check it out to the specified version
// It returns an error on failure.
func downloadSkia(version, path string) error {
	glog.Infof("Cloning Skia version %s to %s", version, path)

	repo, err := gitinfo.CloneOrUpdate("https://skia.googlesource.com/skia", path, false)
	if err != nil {
		return fmt.Errorf("Failed cloning Skia: %s", err)
	}

	if err = repo.SetToCommit(version); err != nil {
		return fmt.Errorf("Problem setting Skia to version %s: %s", version, err)
	}

	syncCmd := &exec.Command{
		Name: "bin/sync-and-gyp",
		Dir:  path,
	}

	if err := exec.Run(syncCmd); err != nil {
		return fmt.Errorf("Failed syncing and setting up gyp: %s", err)
	}

	if lc, err := repo.Details(version); err != nil {
		glog.Errorf("Could not get git details for skia version %s: %s", version, err)
	} else {
		config.SetSkiaVersion(lc)
	}

	return nil
}
