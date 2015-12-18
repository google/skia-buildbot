package generator

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/aggregator"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/gs"
	"golang.org/x/net/context"
	"google.golang.org/cloud/storage"
)

var storageClient *storage.Client
var agg *aggregator.BinaryAggregator

// StartVersionWatcher starts a background routine that will occasionally
// wake up and check the stored version in GCS to see if there is any pending versions.
// If there is, it calls UpdateToNewSkiaVersion
func StartVersionWatcher(s *storage.Client, a *aggregator.BinaryAggregator) <-chan error {
	storageClient = s
	agg = a
	status := make(chan error)
	go func() {
		t := time.Tick(config.Generator.VersionCheckPeriod)
		for {
			_ = <-t
			glog.Infof("Woke up to check the Skia version, currently at %s", config.Generator.SkiaVersion.Hash)
			pending, err := common.GetPendingSkiaVersionFromGCS(storageClient)
			if err != nil {
				glog.Errorf("Failed getting pending Skia version from GCS.  Going to try again: %s", err)
				continue
			}
			glog.Infof("Pending version found to be %q", pending)
			if pending != "" {
				if err := UpdateToNewSkiaVersion(pending); err != nil {
					status <- fmt.Errorf("Failed partway through switching Skia Versions.  We are likely in a broken state.  %s", err)
					break
				}
				glog.Infof("Succesfully switched to skia version %s", pending)
			}
		}
	}()
	return status
}

// UpdateToNewSkiaVersion runs a series of commands to update the fuzzer to a new Skia Version.
// It will stop the Generator, pause the Aggregator, update to the
// new version, re-scan all previous fuzzes and then start the Generator and the Aggregator
// again.  It re-uses the Aggregator pipeline to do the re-analysis.
func UpdateToNewSkiaVersion(newHash string) error {
	oldHash := config.Generator.SkiaVersion.Hash
	// stop afl-fuzz
	StopBinaryGenerator()

	// sync skia to version, which sets config.Generator.SkiaVersion
	if err := common.DownloadSkia(newHash, config.Generator.SkiaRoot, &config.Generator); err != nil {
		return fmt.Errorf("Could not sync skia to %s: %s", newHash, err)
	}

	// download all bad and grey fuzzes
	badFuzzNames, greyFuzzNames, err := downloadAllBadAndGreyFuzzes(oldHash, config.Aggregator.BinaryFuzzPath)
	if err != nil {
		return fmt.Errorf("Problem downloading all previous fuzzes: %s", err)
	}
	glog.Infof("There are %d badFuzzNames and %d greyFuzzNames to rescan.", len(badFuzzNames), len(greyFuzzNames))
	// This is a soft shutdown, i.e. it waits for aggregator's queues to be empty
	if err := agg.RestartAnalysis(); err != nil {
		return fmt.Errorf("Had problem restarting analysis/upload chain: %s", err)
	}
	// Reanalyze and reupload the fuzzes, making a bug on regressions.
	glog.Infof("Reanalyzing bad fuzzes")
	agg.MakeBugOnBadFuzz = false
	for _, name := range badFuzzNames {
		agg.ForceAnalysis(name)
	}
	agg.WaitForEmptyQueues()
	glog.Infof("Reanalyzing grey fuzzes")
	agg.MakeBugOnBadFuzz = true
	for _, name := range greyFuzzNames {
		agg.ForceAnalysis(name)
	}
	agg.WaitForEmptyQueues()
	agg.MakeBugOnBadFuzz = false
	glog.Infof("Done reanalyzing")

	// redownload samples (in case any are new)
	if err := DownloadBinarySeedFiles(storageClient); err != nil {
		return fmt.Errorf("Could not download binary seed files: %s", err)
	}
	// change GCS version to have the current be up to date (fuzzer-fe will need to see that with its polling)
	if err := replaceCurrentSkiaVersionWith(oldHash, config.Generator.SkiaVersion.Hash); err != nil {
		return fmt.Errorf("Could not update skia error: %s", err)
	}

	// restart afl-fuzz
	return StartBinaryGenerator()
}

// completedCounter is the number of fuzzes that have been downloaded from GCS, used for logging.
var completedCounter int32

// downloadAllBadAndGreyFuzzes downloads just the fuzzes from a commit in GCS.
// It uses multiple processes to do so and puts them in downloadPath.
func downloadAllBadAndGreyFuzzes(commitHash, downloadPath string) (badFuzzNames []string, greyFuzzNames []string, err error) {
	toDownload := make(chan string, 100000)
	completedCounter = 0

	var wg sync.WaitGroup
	for i := 0; i < config.Generator.NumDownloadProcesses; i++ {
		wg.Add(1)
		go download(toDownload, downloadPath, &wg)
	}

	badFilter := func(item *storage.ObjectAttrs) {
		name := item.Name
		if strings.HasSuffix(name, ".dump") || strings.HasSuffix(name, ".err") {
			return
		}
		fuzzHash := name[strings.LastIndex(name, "/")+1:]
		badFuzzNames = append(badFuzzNames, filepath.Join(config.Aggregator.BinaryFuzzPath, fuzzHash))
		toDownload <- item.Name
	}

	greyFilter := func(item *storage.ObjectAttrs) {
		name := item.Name
		if strings.HasSuffix(item.Name, ".dump") || strings.HasSuffix(item.Name, ".err") {
			return
		}
		fuzzHash := name[strings.LastIndex(name, "/")+1:]
		greyFuzzNames = append(greyFuzzNames, filepath.Join(config.Aggregator.BinaryFuzzPath, fuzzHash))
		toDownload <- item.Name
	}

	if err := gs.AllFilesInDir(storageClient, config.GS.Bucket, fmt.Sprintf("binary_fuzzes/%s/bad/", commitHash), badFilter); err != nil {
		return nil, nil, fmt.Errorf("Problem getting bad fuzzes: %s", err)
	}

	if err := gs.AllFilesInDir(storageClient, config.GS.Bucket, fmt.Sprintf("binary_fuzzes/%s/grey/", commitHash), greyFilter); err != nil {
		return nil, nil, fmt.Errorf("Problem getting grey fuzzes: %s", err)
	}

	close(toDownload)
	wg.Wait()
	return badFuzzNames, greyFuzzNames, nil
}

// download starts a go routine that waits for files to download from Google Storage
// and downloads them to downloadPath.  When it is done (on error or when the channel
// is closed), it signals to the WaitGroup that it is done.
// It also logs the progress on downloading the fuzzes
func download(toDownload <-chan string, downloadPath string, wg *sync.WaitGroup) {
	defer wg.Done()
	for file := range toDownload {
		contents, err := gs.FileContentsFromGS(storageClient, config.GS.Bucket, file)
		if err != nil {
			glog.Warningf("Problem downloading fuzz %s, continuing anyway: %s", file, err)
		}
		hash := file[strings.LastIndex(file, "/")+1:]
		onDisk := filepath.Join(downloadPath, hash)
		if err = ioutil.WriteFile(onDisk, contents, 0644); err != nil && !os.IsExist(err) {
			glog.Warningf("Problem writing fuzz to %s, continuing anyway: %s", onDisk, err)
		}
		atomic.AddInt32(&completedCounter, 1)
		if completedCounter%100 == 0 {
			glog.Infof("%d fuzzes downloaded", completedCounter)
		}
	}
}

// replaceCurrentSkiaVersionWith puts the oldHash in skia_version/old and
// the newHash in skia_version/current.  It also removes all pending versions.
func replaceCurrentSkiaVersionWith(oldHash, newHash string) error {
	// delete all pending requests
	if err := gs.DeleteAllFilesInDir(storageClient, config.GS.Bucket, "skia_version/pending/"); err != nil {
		return err
	}
	if err := gs.DeleteAllFilesInDir(storageClient, config.GS.Bucket, "skia_version/current/"); err != nil {
		return err
	}
	if err := touch(fmt.Sprintf("skia_version/current/%s", newHash)); err != nil {
		return err
	}
	return touch(fmt.Sprintf("skia_version/old/%s", oldHash))
}

// touch creates an empty file in Google Storage of the given name.
func touch(file string) error {
	w := storageClient.Bucket(config.GS.Bucket).Object(file).NewWriter(context.Background())
	if err := w.Close(); err != nil {
		return fmt.Errorf("Could not touch version file %s : %s", file, err)
	}
	return nil
}
