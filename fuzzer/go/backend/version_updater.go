package backend

import (
	"fmt"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/aggregator"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/generator"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
)

// VersionUpdater is a struct that will handle the updating from one version to fuzz to another
// for the backend. It expects to have UpdateToNewSkiaVersion called with the new hash to update.
type VersionUpdater struct {
	storageClient *storage.Client
	aggregator    *aggregator.Aggregator
	// There is one of these for every fuzz category.
	generators []*generator.Generator
}

// NewVersionUpdater creates a VersionUpdater
func NewVersionUpdater(s *storage.Client, agg *aggregator.Aggregator, g []*generator.Generator) *VersionUpdater {
	return &VersionUpdater{
		storageClient: s,
		aggregator:    agg,
		generators:    g,
	}
}

// UpdateToNewSkiaVersion runs a series of commands to update the fuzzer to a new Skia Version.
// It will stop the Generator, pause the Aggregator, update to the new version, re-scan all previous
// fuzzes and then start the Generator and the Aggregator again.  It re-uses the Aggregator pipeline
// to do the re-analysis.
func (v *VersionUpdater) UpdateToNewSkiaVersion(newHash string) error {
	oldRevision := config.Common.SkiaVersion.Hash

	// stop all afl-fuzz processes
	for _, g := range v.generators {
		g.Stop()
	}

	// sync skia to version, which sets config.Common.SkiaVersion
	if err := common.DownloadSkia(newHash, config.Common.SkiaRoot, &config.Common, false); err != nil {
		return fmt.Errorf("Could not sync skia to %s: %s", newHash, err)
	}

	// Reanalyze all previous found fuzzes and restart with new version
	if err := v.reanalyze(oldRevision); err != nil {
		sklog.Errorf("Problem reanalyzing and restarting aggregation pipeline: %s", err)
	}

	for _, g := range v.generators {
		if err := g.Start(); err != nil {
			return fmt.Errorf("Could not restart generator %s: %s", g.Category, err)
		}
	}

	// change GCS version to have the current be up to date (fuzzer-fe will be polling for it)
	if err := v.replaceCurrentSkiaVersionWith(oldRevision, config.Common.SkiaVersion.Hash); err != nil {
		return fmt.Errorf("Could not update skia error: %s", err)
	}

	sklog.Infof("We are updated to Skia revision %s", newHash)

	return nil
}

func (v *VersionUpdater) reanalyze(oldRevision string) error {

	// This is a soft shutdown, i.e. it waits for aggregator's queues to be empty
	v.aggregator.ShutDown()
	for _, g := range v.generators {
		if err := g.Clear(); err != nil {
			return fmt.Errorf("Could not clear generator %s: %s", g.Category, err)
		}
		// redownload samples (in case any are new)
		if err := g.DownloadSeedFiles(v.storageClient); err != nil {
			return fmt.Errorf("Could not download binary seed files: %s", err)
		}
	}

	// Recompile Skia at the new version.
	if err := v.aggregator.RestartAnalysis(); err != nil {
		return fmt.Errorf("Had problem restarting analysis/upload chain: %s", err)
	}

	for _, category := range config.Generator.FuzzesToGenerate {

		// download all bad and grey fuzzes
		badFuzzPaths, greyFuzzPaths, err := downloadAllBadAndGreyFuzzes(oldRevision, category, v.storageClient)
		if err != nil {
			return fmt.Errorf("Problem downloading all previous fuzzes: %s", err)
		}
		sklog.Infof("There are %d bad fuzzes and %d grey fuzzes of category %s to rescan.", len(badFuzzPaths), len(greyFuzzPaths), category)

		if config.Common.ForceReanalysis {
			sklog.Infof("Deleting previous %s fuzz results", category)
			if err := gs.DeleteAllFilesInDir(v.storageClient, config.GS.Bucket, fmt.Sprintf("%s/%s/%s", category, oldRevision, config.Generator.Architecture), config.Aggregator.NumUploadProcesses); err != nil {
				return fmt.Errorf("Could not delete previous fuzzes: %s", err)
			}
		}

		// If we aren't reanalyzing, we should upload the names of anything that is currently there.
		// If we are reanalyzing, we should re-write the names after we analyze them (see below).
		if !config.Common.ForceReanalysis {
			uploadFuzzNames(v.storageClient, oldRevision, category, common.ExtractFuzzNamesFromPaths(badFuzzPaths), common.ExtractFuzzNamesFromPaths(greyFuzzPaths))
		}
		// Reanalyze and reupload the fuzzes, making a bug on regressions.
		sklog.Infof("Reanalyzing bad %s fuzzes", category)
		v.aggregator.WatchForRegressions = false
		v.aggregator.MakeBugOnBadFuzz = false
		v.aggregator.UploadGreyFuzzes = true
		v.aggregator.ClearUploadedFuzzNames()
		for _, name := range badFuzzPaths {
			v.aggregator.ForceAnalysis(name, category)
		}
		v.aggregator.WaitForEmptyQueues()
		sklog.Infof("Reanalyzing grey %s fuzzes", category)
		v.aggregator.MakeBugOnBadFuzz = true
		v.aggregator.WatchForRegressions = true
		for _, name := range greyFuzzPaths {
			v.aggregator.ForceAnalysis(name, category)
		}
		v.aggregator.WaitForEmptyQueues()
		v.aggregator.WatchForRegressions = false
		v.aggregator.MakeBugOnBadFuzz = true
		v.aggregator.UploadGreyFuzzes = false
		bad, grey, deduped := v.aggregator.UploadedFuzzNames()
		sklog.Infof("Done reanalyzing %s.  Uploaded %d bad and %d grey fuzzes.  There were %d duplicate bad fuzzes that were skipped.", category, len(bad), len(grey), len(deduped))
		metrics2.GetInt64Metric("fuzzer.fuzzes.status", map[string]string{"category": category, "architecture": config.Generator.Architecture, "status": "bad"}).Update(int64(len(bad)))
		metrics2.GetInt64Metric("fuzzer.fuzzes.status", map[string]string{"category": category, "architecture": config.Generator.Architecture, "status": "grey"}).Update(int64(len(grey)))

		if config.Common.ForceReanalysis {
			uploadFuzzNames(v.storageClient, oldRevision, category, bad, grey)
		}
	}
	sklog.Info("All done reanlyzing fuzzes")

	return nil
}

// downloadAllBadAndGreyFuzzes downloads just the fuzzes from a commit in GCS. It uses multiple
// processes to do so and puts them in config.Aggregator.FuzzPath/[category].
func downloadAllBadAndGreyFuzzes(commitHash, category string, storageClient *storage.Client) (badFuzzPaths []string, greyFuzzPaths []string, err error) {

	bad, err := common.DownloadAllFuzzes(storageClient, config.Aggregator.FuzzPath, category, commitHash, config.Generator.Architecture, "bad", config.Generator.NumDownloadProcesses)
	if err != nil {
		return nil, nil, err
	}
	grey, err := common.DownloadAllFuzzes(storageClient, config.Aggregator.FuzzPath, category, commitHash, config.Generator.Architecture, "grey", config.Generator.NumDownloadProcesses)
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

// uploadFuzzNames creates two files in the /category/revision/architecture folder that contain all
// of the bad fuzz names and the grey fuzz names that are in this folder
func uploadFuzzNames(sc *storage.Client, oldRevision, category string, bad, grey []string) {
	uploadString := func(fileName, contents string) error {
		name := fmt.Sprintf("%s/%s/%s/%s", category, oldRevision, config.Generator.Architecture, fileName)
		w := sc.Bucket(config.GS.Bucket).Object(name).NewWriter(context.Background())
		defer util.Close(w)
		w.ObjectAttrs.ContentEncoding = "text/plain"

		if n, err := w.Write([]byte(contents)); err != nil {
			return fmt.Errorf("There was a problem uploading %s.  Only uploaded %d bytes: %s", name, n, err)
		}
		return nil
	}

	if err := uploadString("bad_fuzz_names.txt", strings.Join(bad, "|")); err != nil {
		sklog.Errorf("Problem uploading bad fuzz names: %s", err)
	}
	if err := uploadString("grey_fuzz_names.txt", strings.Join(grey, "|")); err != nil {
		sklog.Errorf("Problem uploading grey fuzz names: %s", err)
	}
}
