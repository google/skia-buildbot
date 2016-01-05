package generator

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/aggregator"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/net/context"
	"google.golang.org/cloud/storage"
)

// VersionUpdater is a struct that will handle the updating from one version to fuzz to another
// for the backend. It expects to have UpdateToNewSkiaVersion called with the new hash to update.
type VersionUpdater struct {
	storageClient *storage.Client
	agg           *aggregator.BinaryAggregator
}

// NewVersionUpdater creates a VersionUpdater
func NewVersionUpdater(s *storage.Client, a *aggregator.BinaryAggregator) *VersionUpdater {
	return &VersionUpdater{
		storageClient: s,
		agg:           a,
	}
}

// UpdateToNewSkiaVersion runs a series of commands to update the fuzzer to a new Skia Version.
// It will stop the Generator, pause the Aggregator, update to the
// new version, re-scan all previous fuzzes and then start the Generator and the Aggregator
// again.  It re-uses the Aggregator pipeline to do the re-analysis.
func (v *VersionUpdater) UpdateToNewSkiaVersion(newHash string) (*vcsinfo.LongCommit, error) {
	oldHash := config.Generator.SkiaVersion.Hash
	// stop afl-fuzz
	StopBinaryGenerator()

	// sync skia to version, which sets config.Generator.SkiaVersion
	if err := common.DownloadSkia(newHash, config.Generator.SkiaRoot, &config.Generator); err != nil {
		return nil, fmt.Errorf("Could not sync skia to %s: %s", newHash, err)
	}

	// download all bad and grey fuzzes
	badFuzzNames, greyFuzzNames, err := v.downloadAllBadAndGreyFuzzes(oldHash, config.Aggregator.BinaryFuzzPath)
	if err != nil {
		return nil, fmt.Errorf("Problem downloading all previous fuzzes: %s", err)
	}
	glog.Infof("There are %d badFuzzNames and %d greyFuzzNames to rescan.", len(badFuzzNames), len(greyFuzzNames))
	// This is a soft shutdown, i.e. it waits for aggregator's queues to be empty
	if err := v.agg.RestartAnalysis(); err != nil {
		return nil, fmt.Errorf("Had problem restarting analysis/upload chain: %s", err)
	}
	// Reanalyze and reupload the fuzzes, making a bug on regressions.
	glog.Infof("Reanalyzing bad fuzzes")
	v.agg.MakeBugOnBadFuzz = false
	v.agg.UploadGreyFuzzes = true
	for _, name := range badFuzzNames {
		v.agg.ForceAnalysis(name)
	}
	v.agg.WaitForEmptyQueues()
	glog.Infof("Reanalyzing grey fuzzes")
	v.agg.MakeBugOnBadFuzz = true
	for _, name := range greyFuzzNames {
		v.agg.ForceAnalysis(name)
	}
	v.agg.WaitForEmptyQueues()
	v.agg.MakeBugOnBadFuzz = false
	v.agg.UploadGreyFuzzes = false
	glog.Infof("Done reanalyzing")

	// redownload samples (in case any are new)
	if err := DownloadBinarySeedFiles(v.storageClient); err != nil {
		return nil, fmt.Errorf("Could not download binary seed files: %s", err)
	}
	// change GCS version to have the current be up to date (fuzzer-fe will need to see that with its polling)
	if err := v.replaceCurrentSkiaVersionWith(oldHash, config.Generator.SkiaVersion.Hash); err != nil {
		return nil, fmt.Errorf("Could not update skia error: %s", err)
	}

	// restart afl-fuzz
	return config.Generator.SkiaVersion, StartBinaryGenerator()
}

// completedCounter is the number of fuzzes that have been downloaded from GCS, used for logging.
var completedCounter int32

// downloadAllBadAndGreyFuzzes downloads just the fuzzes from a commit in GCS.
// It uses multiple processes to do so and puts them in downloadPath.
func (v *VersionUpdater) downloadAllBadAndGreyFuzzes(commitHash, downloadPath string) (badFuzzNames []string, greyFuzzNames []string, err error) {
	toDownload := make(chan string, 100000)
	completedCounter = 0

	var wg sync.WaitGroup
	for i := 0; i < config.Generator.NumDownloadProcesses; i++ {
		wg.Add(1)
		go v.download(toDownload, downloadPath, &wg)
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

	if err := gs.AllFilesInDir(v.storageClient, config.GS.Bucket, fmt.Sprintf("binary_fuzzes/%s/bad/", commitHash), badFilter); err != nil {
		return nil, nil, fmt.Errorf("Problem getting bad fuzzes: %s", err)
	}

	if err := gs.AllFilesInDir(v.storageClient, config.GS.Bucket, fmt.Sprintf("binary_fuzzes/%s/grey/", commitHash), greyFilter); err != nil {
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
func (v *VersionUpdater) download(toDownload <-chan string, downloadPath string, wg *sync.WaitGroup) {
	defer wg.Done()
	for file := range toDownload {
		contents, err := gs.FileContentsFromGS(v.storageClient, config.GS.Bucket, file)
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
func (v *VersionUpdater) replaceCurrentSkiaVersionWith(oldHash, newHash string) error {
	// delete all pending requests
	if err := gs.DeleteAllFilesInDir(v.storageClient, config.GS.Bucket, "skia_version/pending/"); err != nil {
		return err
	}
	if err := gs.DeleteAllFilesInDir(v.storageClient, config.GS.Bucket, "skia_version/current/"); err != nil {
		return err
	}
	if err := v.touch(fmt.Sprintf("skia_version/current/%s", newHash)); err != nil {
		return err
	}
	return v.touch(fmt.Sprintf("skia_version/old/%s", oldHash))
}

// touch creates an empty file in Google Storage of the given name.
func (v *VersionUpdater) touch(file string) error {
	w := v.storageClient.Bucket(config.GS.Bucket).Object(file).NewWriter(context.Background())
	if err := w.Close(); err != nil {
		return fmt.Errorf("Could not touch version file %s : %s", file, err)
	}
	return nil
}
