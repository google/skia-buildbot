package backend

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/aggregator"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/generator"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/net/context"
	"google.golang.org/cloud/storage"
)

// FuzzPipeline holds onto the generation/aggregation parts for a given fuzz category.  This allows
// VersionUpdater to stop all active fuzz generation, download pre-existing fuzzes, re-analyze
// them, and then restart generation.
type FuzzPipeline struct {
	Category string
	Agg      *aggregator.Aggregator
	Gen      *generator.Generator
}

// VersionUpdater is a struct that will handle the updating from one version to fuzz to another
// for the backend. It expects to have UpdateToNewSkiaVersion called with the new hash to update.
type VersionUpdater struct {
	storageClient *storage.Client
	// There is one of these for every fuzz category.
	pipelines []FuzzPipeline
}

// NewVersionUpdater creates a VersionUpdater
func NewVersionUpdater(s *storage.Client, p []FuzzPipeline) *VersionUpdater {
	return &VersionUpdater{
		storageClient: s,
		pipelines:     p,
	}
}

// UpdateToNewSkiaVersion runs a series of commands to update the fuzzer to a new Skia Version.
// It will stop the Generator, pause the Aggregator, update to the new version, re-scan all previous
// fuzzes and then start the Generator and the Aggregator again.  It re-uses the Aggregator pipeline
// to do the re-analysis.
func (v *VersionUpdater) UpdateToNewSkiaVersion(newHash string) (*vcsinfo.LongCommit, error) {
	oldRevision := config.Generator.SkiaVersion.Hash

	// stop all afl-fuzz processes
	for _, p := range v.pipelines {
		p.Gen.Stop()
	}

	// sync skia to version, which sets config.Generator.SkiaVersion
	if err := common.DownloadSkia(newHash, config.Generator.SkiaRoot, &config.Generator); err != nil {
		return nil, fmt.Errorf("Could not sync skia to %s: %s", newHash, err)
	}

	for _, p := range v.pipelines {
		// Reanalyze all previous found fuzzes and restart with new version
		if err := p.reanalyzeAndRestart(v.storageClient, oldRevision); err != nil {
			glog.Errorf("Problem reanalyzing and restarting %s pipeline", p.Category)
		}
	}

	// change GCS version to have the current be up to date (fuzzer-fe will be polling for it)
	if err := v.replaceCurrentSkiaVersionWith(oldRevision, config.Generator.SkiaVersion.Hash); err != nil {
		return nil, fmt.Errorf("Could not update skia error: %s", err)
	}

	return config.Generator.SkiaVersion, nil
}

func (p *FuzzPipeline) reanalyzeAndRestart(storageClient *storage.Client, oldRevision string) error {
	// download all bad and grey fuzzes
	badFuzzPaths, greyFuzzPaths, err := p.downloadAllBadAndGreyFuzzes(oldRevision, storageClient)
	if err != nil {
		return fmt.Errorf("Problem downloading all previous fuzzes: %s", err)
	}
	glog.Infof("There are %d bad fuzzes and %d grey fuzzes of category %s to rescan.", len(badFuzzPaths), len(greyFuzzPaths), p.Category)
	// This is a soft shutdown, i.e. it waits for aggregator's queues to be empty
	p.Agg.ShutDown()

	if config.Common.ForceReanalysis {
		glog.Infof("Deleting previous %s fuzz results", p.Category)
		if err := gs.DeleteAllFilesInDir(storageClient, config.GS.Bucket, fmt.Sprintf("%s/%s/", p.Category, oldRevision), config.Aggregator.NumUploadProcesses); err != nil {
			return fmt.Errorf("Could not delete previous fuzzes: %s", err)
		}
	}

	if err := p.Gen.Clear(); err != nil {
		return fmt.Errorf("Could not remove previous afl-fuzz results: %s", err)
	}

	if err := p.Agg.RestartAnalysis(); err != nil {
		return fmt.Errorf("Had problem restarting analysis/upload chain: %s", err)
	}
	// If we aren't reanalyzing, we should upload the names of anything that is currently there.
	// If we are reanalyzing, we should re-write the names after we analyze them (see below).
	if !config.Common.ForceReanalysis {
		p.uploadFuzzNames(storageClient, oldRevision, common.ExtractFuzzNamesFromPaths(badFuzzPaths), common.ExtractFuzzNamesFromPaths(greyFuzzPaths))
	}
	// Reanalyze and reupload the fuzzes, making a bug on regressions.
	glog.Infof("Reanalyzing bad fuzzes")
	p.Agg.MakeBugOnBadFuzz = false
	p.Agg.UploadGreyFuzzes = true
	p.Agg.ClearUploadedFuzzNames()
	for _, name := range badFuzzPaths {
		p.Agg.ForceAnalysis(name)
	}
	p.Agg.WaitForEmptyQueues()
	glog.Infof("Reanalyzing grey fuzzes")
	p.Agg.MakeBugOnBadFuzz = true
	for _, name := range greyFuzzPaths {
		p.Agg.ForceAnalysis(name)
	}
	p.Agg.WaitForEmptyQueues()
	p.Agg.MakeBugOnBadFuzz = false
	p.Agg.UploadGreyFuzzes = false
	bad, grey := p.Agg.UploadedFuzzNames()
	glog.Infof("Done reanalyzing %s.  Uploaded %d bad and %d grey fuzzes", p.Category, len(bad), len(grey))

	if config.Common.ForceReanalysis {
		p.uploadFuzzNames(storageClient, oldRevision, bad, grey)
	}

	// redownload samples (in case any are new)
	if err := p.Gen.DownloadSeedFiles(storageClient); err != nil {
		return fmt.Errorf("Could not download binary seed files: %s", err)
	}
	// restart afl-fuzz
	return p.Gen.Start()
}

// downloadAllBadAndGreyFuzzes downloads just the fuzzes from a commit in GCS. It uses multiple
// processes to do so and puts them in config.Aggregator.FuzzPath/[category].
func (p *FuzzPipeline) downloadAllBadAndGreyFuzzes(commitHash string, storageClient *storage.Client) (badFuzzPaths []string, greyFuzzPaths []string, err error) {
	downloadPath := filepath.Join(config.Aggregator.FuzzPath, p.Category)

	bad, err := common.DownloadAllFuzzes(storageClient, downloadPath, p.Category, commitHash, "bad", config.Generator.NumDownloadProcesses)
	if err != nil {
		return nil, nil, err
	}
	grey, err := common.DownloadAllFuzzes(storageClient, downloadPath, p.Category, commitHash, "grey", config.Generator.NumDownloadProcesses)
	return bad, grey, err
}

// replaceCurrentSkiaVersionWith puts the oldHash in skia_version/old and the newHash in
// skia_version/current.  It also removes all pending versions.
func (v *VersionUpdater) replaceCurrentSkiaVersionWith(oldHash, newHash string) error {
	// delete all pending requests
	if err := gs.DeleteAllFilesInDir(v.storageClient, config.GS.Bucket, "skia_version/pending/", 1); err != nil {
		return err
	}
	if err := gs.DeleteAllFilesInDir(v.storageClient, config.GS.Bucket, "skia_version/current/", 1); err != nil {
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

// uploadFuzzNames creates two files in the /category/revision/ folder that contain all of the bad fuzz names and the grey fuzz names that are in this folder
func (p *FuzzPipeline) uploadFuzzNames(sc *storage.Client, oldRevision string, bad, grey []string) {
	uploadString := func(fileName, contents string) error {
		name := fmt.Sprintf("%s/%s/%s", p.Category, oldRevision, fileName)
		w := sc.Bucket(config.GS.Bucket).Object(name).NewWriter(context.Background())
		defer util.Close(w)
		w.ObjectAttrs.ContentEncoding = "text/plain"

		if n, err := w.Write([]byte(contents)); err != nil {
			return fmt.Errorf("There was a problem uploading %s.  Only uploaded %d bytes: %s", name, n, err)
		}
		return nil
	}

	if err := uploadString("bad_fuzz_names.txt", strings.Join(bad, "|")); err != nil {
		glog.Errorf("Problem uploading bad fuzz names: %s", err)
	}
	if err := uploadString("grey_fuzz_names.txt", strings.Join(grey, "|")); err != nil {
		glog.Errorf("Problem uploading grey fuzz names: %s", err)
	}
}
