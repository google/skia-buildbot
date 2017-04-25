package backend

import (
	"fmt"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/aggregator"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/download_skia"
	"go.skia.org/infra/fuzzer/go/generator"
	fstorage "go.skia.org/infra/fuzzer/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/net/context"
)

// VersionUpdater is a struct that will handle the updating from one version to fuzz to another
// for the backend. It expects to have UpdateToNewSkiaVersion called with the new hash to update.
type VersionUpdater struct {
	storageClient fstorage.FuzzerGCSClient
	aggregator    *aggregator.Aggregator
	// There is one of these for every fuzz category.
	generators []*generator.Generator
}

// NewVersionUpdater creates a VersionUpdater
func NewVersionUpdater(s fstorage.FuzzerGCSClient, agg *aggregator.Aggregator, g []*generator.Generator) *VersionUpdater {
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
func (v *VersionUpdater) UpdateToNewSkiaVersion(newRevision string) error {
	oldRevision := config.Common.SkiaVersion.Hash

	// stop all afl-fuzz processes
	for _, g := range v.generators {
		g.Stop()
	}

	// sync skia to version, which sets config.Common.SkiaVersion
	if err := download_skia.AtRevision(newRevision, config.Common.SkiaRoot, &config.Common, true); err != nil {
		return fmt.Errorf("Could not sync skia to %s: %s", newRevision, err)
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

	// Let frontend know this backend has finished rolling forward.
	if err := v.reportWorkDone(oldRevision, config.Common.SkiaVersion.Hash); err != nil {
		return fmt.Errorf("Could not update skia error: %s", err)
	}

	sklog.Infof("We are updated to Skia revision %s", newRevision)

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
			if err := v.storageClient.DeleteAllFilesInFolder(fmt.Sprintf("%s/%s/%s", category, oldRevision, config.Generator.Architecture), config.Aggregator.NumUploadProcesses); err != nil {
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
		metrics2.GetInt64Metric("fuzzer_fuzzes_status", map[string]string{"category": category, "architecture": config.Generator.Architecture, "status": "bad"}).Update(int64(len(bad)))
		metrics2.GetInt64Metric("fuzzer_fuzzes_status", map[string]string{"category": category, "architecture": config.Generator.Architecture, "status": "grey"}).Update(int64(len(grey)))

		if config.Common.ForceReanalysis {
			uploadFuzzNames(v.storageClient, oldRevision, category, bad, grey)
		}
	}
	sklog.Info("All done reanlyzing fuzzes")

	return nil
}

// downloadAllBadAndGreyFuzzes downloads just the fuzzes from a commit in GCS. It uses multiple
// processes to do so and puts them in config.Aggregator.FuzzPath/[category].
func downloadAllBadAndGreyFuzzes(commitHash, category string, storageClient fstorage.FuzzerGCSClient) (badFuzzPaths []string, greyFuzzPaths []string, err error) {

	bad, err := storageClient.DownloadAllFuzzes(config.Aggregator.FuzzPath, category, commitHash, config.Generator.Architecture, "bad", config.Generator.NumDownloadProcesses)
	if err != nil {
		return nil, nil, err
	}
	grey, err := storageClient.DownloadAllFuzzes(config.Aggregator.FuzzPath, category, commitHash, config.Generator.Architecture, "grey", config.Generator.NumDownloadProcesses)
	return bad, grey, err
}

// reportWorkDone puts the oldRevision in skia_version/old and the newRevision in
// skia_version/current.  It also removes all pending versions.
func (v *VersionUpdater) reportWorkDone(oldRevision, newRevision string) error {
	// delete work request
	workFile := fmt.Sprintf("skia_version/pending/working_%s", common.Hostname())
	if err := v.storageClient.DeleteFile(context.Background(), workFile); err != nil && !(err == storage.ErrObjectNotExist || strings.Contains(err.Error(), "404")) {
		return err
	} else if err != nil {
		sklog.Warningf("There was an error while deleting %s, but continuing anyway: %s", workFile, err)
	}
	count := 0
	if err := v.storageClient.AllFilesInDirectory(context.Background(), "skia_version/pending/working_", func(item *storage.ObjectAttrs) {
		count++
	}); err != nil {
		return err
	}
	sklog.Infof("There are %d backend workers still rolling forward.", count)
	// If count is 0, there are no workers left rolling forward.  Otherwise, there is still
	// work to do by other workers, so this backend is done.
	if count == 0 {
		// clear out pending version. There shouldn't be more than one file, but if
		// there is, recover from such a broken state.
		if err := v.storageClient.DeleteAllFilesInFolder("skia_version/pending/", 1); err != nil {
			return err
		}
		// Clear out current version. Same rationale as pending version
		if err := v.storageClient.DeleteAllFilesInFolder("skia_version/current/", 1); err != nil {
			return err
		}

		// Set the current version for the frontend to see (all the backends should
		// already be on newRevision)
		newVersionFile := fmt.Sprintf("skia_version/current/%s", newRevision)
		if err := v.storageClient.SetFileContents(context.Background(), newVersionFile, gcs.FILE_WRITE_OPTS_TEXT, []byte(newRevision)); err != nil {
			return fmt.Errorf("Could not set current version: %s", err)
		}
		// Put the old version in the old folder to record what we have fuzzed on.
		oldVersionFile := fmt.Sprintf("skia_version/old/%s", oldRevision)
		if err := v.storageClient.SetFileContents(context.Background(), oldVersionFile, gcs.FILE_WRITE_OPTS_TEXT, []byte(oldRevision)); err != nil {
			return fmt.Errorf("Could not set old version: %s", err)
		}
	}

	return nil
}

// uploadFuzzNames creates two files in the /category/revision/architecture folder that contain all
// of the bad fuzz names and the grey fuzz names that are in this folder
func uploadFuzzNames(sc fstorage.FuzzerGCSClient, oldRevision, category string, bad, grey []string) {
	uploadString := func(fileName, contents string) error {
		name := fmt.Sprintf("%s/%s/%s/%s", category, oldRevision, config.Generator.Architecture, fileName)

		if err := sc.SetFileContents(context.Background(), name, gcs.FILE_WRITE_OPTS_TEXT, []byte(contents)); err != nil {
			return fmt.Errorf("There was a problem uploading %s. %s", name, err)
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
