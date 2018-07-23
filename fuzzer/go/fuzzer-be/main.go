package main

/*
Runs the backend portions of the fuzzer.  This includes the generator and aggregator parts (see DESIGN.md)
*/

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/aggregator"
	"go.skia.org/infra/fuzzer/go/backend"
	fcommon "go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/fuzzer/go/download_skia"
	"go.skia.org/infra/fuzzer/go/generator"
	"go.skia.org/infra/fuzzer/go/issues"
	fstorage "go.skia.org/infra/fuzzer/go/storage"
	"go.skia.org/infra/fuzzer/go/tests"
	"go.skia.org/infra/fuzzer/go/version_watcher"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

var (
	aflOutputPath          = flag.String("afl_output_path", "", "[REQUIRED] The output folder of afl-fuzz.  This will have on folder for each fuzz_to_run.  Each of those will have N folders named fuzzer0 - fuzzerN.  Should not be in /tmp or afl-fuzz will refuse to run.")
	generatorWD            = flag.String("generator_working_dir", "", "[REQUIRED] The generator's working directory.  Should not be in /tmp.")
	aggregatorWD           = flag.String("aggregator_working_dir", filepath.Join(os.TempDir(), "aggregator_wd"), "The aggregator's working directory.  Can be in /tmp.")
	fuzzSamples            = flag.String("fuzz_samples", "", "[REQUIRED] The generator's working directory.  Should not be in /tmp.")
	skiaRoot               = flag.String("skia_root", "", "[REQUIRED] The root directory of the Skia source code.  Cannot be shared with front end.")
	clangPath              = flag.String("clang_path", "", "[REQUIRED] The path to the clang executable.")
	clangPlusPlusPath      = flag.String("clang_p_p_path", "", "[REQUIRED] The path to the clang++ executable.")
	depotToolsPath         = flag.String("depot_tools_path", "", "The absolute path to depot_tools.  Can be empty if they are on your path.")
	aflRoot                = flag.String("afl_root", "", "[REQUIRED] The install directory of afl-fuzz (v1.94b or later).")
	architecture           = flag.String("architecture", "", "[REQUIRED] The name of the architecture this machine is fuzzing (v1.94b or later).")
	numBinaryFuzzProcesses = flag.Int("binary_fuzz_processes", 0, `The number of processes to run binary fuzzes per fuzz category.  This should be fewer than the number of logical cores.  Defaults to 0, which means "Make an intelligent guess"`)
	numAPIFuzzProcesses    = flag.Int("api_fuzz_processes", 0, `The number of processes to run api fuzzes per fuzz category.  This should be fewer than the number of logical cores.  Defaults to 0, which means "Make an intelligent guess"`)
	versionCheckPeriod     = flag.Duration("version_check_period", 20*time.Second, `The period used to check the version of Skia that needs fuzzing.`)
	downloadProcesses      = flag.Int("download_processes", 4, "The number of download processes to be used for fetching fuzzes when re-analyzing them. This is constant with respect to the number of fuzzes.")
	fuzzesToRun            = common.NewMultiStringFlag("fuzz_to_run", nil, fmt.Sprintf("A set of fuzzes to run.  Can be one or more of the known fuzzes: %q", fcommon.FUZZ_CATEGORIES))

	bucket              = flag.String("bucket", "skia-fuzzer", "The GCS bucket in which to store found fuzzes.")
	fuzzPath            = flag.String("fuzz_path", filepath.Join(os.TempDir(), "fuzzes"), "The directory to temporarily store the binary fuzzes during aggregation.")
	executableCachePath = flag.String("executable_cache_path", filepath.Join(os.TempDir(), "executable_cache"), "The path in which built fuzz executables can be cached.  Can be safely shared with frontend.")

	numAnalysisProcesses = flag.Int("analysis_processes", 0, `The number of processes to analyze fuzzes [per fuzz to run].  This should be fewer than the number of logical cores.  Defaults to 0, which means "Make an intelligent guess"`)
	rescanPeriod         = flag.Duration("rescan_period", 60*time.Second, `The time in which to sleep for every cycle of aggregation. `)
	numUploadProcesses   = flag.Int("upload_processes", 0, `The number of processes to upload fuzzes [per fuzz to run]. Defaults to 0, which means "Make an intelligent guess"`)
	statusPeriod         = flag.Duration("status_period", 60*time.Second, `The time period used to report the status of the aggregation/analysis/upload queue. `)
	analysisTimeout      = flag.Duration("analysis_timeout", 5*time.Second, `The maximum time an analysis should run.`)

	watchAFL         = flag.Bool("watch_afl", false, "(debug only) If the afl master's output should be piped to stdout.")
	skipGeneration   = flag.Bool("skip_generation", false, "(debug only) If the generation step should be disabled.")
	forceReanalysis  = flag.Bool("force_reanalysis", false, "(debug only) If the fuzzes should be downloaded, re-analyzed, (deleted from GCS), and reuploaded.")
	verboseBuilds    = flag.Bool("verbose_builds", false, "If output from ninja and gyp should be printed to stdout.")
	local            = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	overrideHostname = flag.String("override_hostname", "", "Hostname to be used if running locally or on Kubernetes")
	promPort         = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

var (
	requiredFlags                       = []string{"afl_output_path", "skia_root", "clang_path", "clang_p_p_path", "afl_root", "generator_working_dir", "fuzz_to_run", "executable_cache_path", "architecture"}
	storageClient *storage.Client       = nil
	issueManager  *issues.IssuesManager = nil
)

func main() {
	flag.Parse()
	// Reset this because flag.Parse() will be called again with common.Init*
	fuzzesToRun.Reset()
	if *overrideHostname != "" {
		mc := tests.NewMockCommonImpl()
		mc.On("Hostname").Return(*overrideHostname)
		fcommon.SetMockCommon(mc)
	}
	if *local {
		if *overrideHostname != "" {
			common.InitWithMust(
				*overrideHostname,
			)
		} else {
			common.InitWithMust(
				"fuzzer-be-local",
			)
		}
	} else {
		common.InitWithMust(
			"fuzzer-be",
			common.PrometheusOpt(promPort),
		)
	}
	ctx := context.Background()

	if err := writeFlagsToConfig(); err != nil {
		sklog.Fatalf("Problem with configuration: %v", err)
	}
	if err := setupOAuth(ctx); err != nil {
		sklog.Fatalf("Problem with OAuth: %s", err)
	}
	client := fstorage.NewFuzzerGCSClient(storageClient, config.GCS.Bucket)
	if err := download_skia.AtGCSRevision(ctx, client, config.Common.SkiaRoot, &config.Common, !*local); err != nil {
		sklog.Fatalf("Problem downloading Skia: %s", err)
	}

	generators := make([]*generator.Generator, 0, len(*fuzzesToRun))
	startingReports := make(map[string]<-chan data.FuzzReport)

	// TODO(kjlubick) implement a sharding scheme for fuzzer to avoid running out of CPUs.
	neededCPUs := 0
	for _, category := range *fuzzesToRun {
		if strings.HasPrefix(category, "api_") {
			neededCPUs += config.Generator.NumAPIFuzzProcesses
		} else {
			neededCPUs += config.Generator.NumBinaryFuzzProcesses
		}
	}
	if totalCPUs := runtime.NumCPU(); neededCPUs > totalCPUs {
		sklog.Warningf("Going to run out of cpus to allocate to generator processes.  Needed at least %d, but only have %d.  Expect suboptimal performance", neededCPUs, totalCPUs)
	} else {
		sklog.Infof("Going to allocate %d cpus (could go up to %d)", neededCPUs, totalCPUs)
	}

	for _, category := range *fuzzesToRun {
		gen := generator.New(category)

		if err := gen.DownloadSeedFiles(client); err != nil {
			sklog.Fatalf("Problem downloading seed files: %s", err)
		}

		// If we are reanalyzing, no point in running the generator first, just to stop it, nor
		// is there reason to download all of the current fuzz reports.
		if !*forceReanalysis {
			sklog.Infof("Starting %s generator with configuration %#v", category, config.Generator)
			var err error
			if err = gen.Start(ctx); err != nil {
				sklog.Fatalf("Problem starting generator: %s", err)
			}
			sklog.Infof("Downloading all bad %s fuzzes @%s to setup duplication detection", category, config.Common.SkiaVersion.Hash)
			baseFolder := fmt.Sprintf("%s/%s/%s/bad", category, config.Common.SkiaVersion.Hash, config.Generator.Architecture)
			if startingReports[category], err = fstorage.GetReportsFromGCS(storageClient, baseFolder, category, config.Generator.Architecture, nil, config.Generator.NumDownloadProcesses); err != nil {
				sklog.Fatalf("Could not download previously found %s fuzzes for deduplication: %s", category, err)
			}
		} else {
			sklog.Infof("Skipping %s generator and deduplication setup because --force_reanalysis is enabled", category)
		}
		generators = append(generators, gen)
	}

	sklog.Infof("Starting aggregator with configuration %#v", config.Aggregator)
	agg, err := aggregator.StartAggregator(ctx, storageClient, issueManager, startingReports)
	if err != nil {
		sklog.Fatalf("Could not start aggregator: %s", err)
	}

	updater := backend.NewVersionUpdater(client, agg, generators)
	sklog.Info("Starting version watcher")
	watcher := version_watcher.New(client, config.Common.VersionCheckPeriod, updater.UpdateToNewSkiaVersion, nil)
	watcher.Start(ctx)

	err = <-watcher.Status
	sklog.Fatal(err)
}

func writeFlagsToConfig() error {
	// Check the required ones and terminate if they are not provided
	for _, f := range requiredFlags {
		if flag.Lookup(f).Value.String() == "" {
			return fmt.Errorf("Required flag %s is empty.", f)
		}
	}
	var err error
	config.Generator.AflOutputPath, err = fileutil.EnsureDirExists(*aflOutputPath)
	if err != nil {
		return err
	}
	config.Generator.AflRoot, err = fileutil.EnsureDirExists(*aflRoot)
	if err != nil {
		return err
	}
	config.Generator.WorkingPath, err = fileutil.EnsureDirExists(*generatorWD)
	if err != nil {
		return err
	}
	config.Generator.FuzzSamples, err = fileutil.EnsureDirExists(*fuzzSamples)
	if err != nil {
		return err
	}

	config.Common.SkiaRoot, err = fileutil.EnsureDirExists(*skiaRoot)
	if err != nil {
		return err
	}
	config.Common.ExecutableCachePath, err = fileutil.EnsureDirExists(*executableCachePath)
	if err != nil {
		return err
	}

	config.Common.VerboseBuilds = *verboseBuilds
	config.Common.ClangPath = *clangPath
	config.Common.ClangPlusPlusPath = *clangPlusPlusPath
	config.Common.DepotToolsPath = *depotToolsPath
	config.Common.VersionCheckPeriod = *versionCheckPeriod

	config.Generator.Architecture = *architecture
	config.Generator.NumBinaryFuzzProcesses = *numBinaryFuzzProcesses
	config.Generator.NumAPIFuzzProcesses = *numAPIFuzzProcesses
	config.Generator.WatchAFL = *watchAFL
	config.Generator.NumDownloadProcesses = *downloadProcesses
	config.Generator.SkipGeneration = *skipGeneration

	config.GCS.Bucket = *bucket
	config.Aggregator.FuzzPath, err = fileutil.EnsureDirExists(*fuzzPath)
	if err != nil {
		return err
	}
	config.Aggregator.WorkingPath, err = fileutil.EnsureDirExists(*aggregatorWD)
	if err != nil {
		return err
	}
	config.Aggregator.NumAnalysisProcesses = *numAnalysisProcesses
	config.Aggregator.NumUploadProcesses = *numUploadProcesses
	config.Aggregator.StatusPeriod = *statusPeriod
	config.Aggregator.RescanPeriod = *rescanPeriod
	config.Aggregator.AnalysisTimeout = *analysisTimeout
	config.Common.ForceReanalysis = *forceReanalysis

	// Check all the fuzzes are valid ones we can handle
	for _, f := range *fuzzesToRun {
		if !fcommon.HasCategory(f) {
			return fmt.Errorf("Unknown fuzz category %q", f)
		}
	}
	config.Generator.FuzzesToGenerate = *fuzzesToRun
	return nil
}

func setupOAuth(ctx context.Context) error {
	ts, err := auth.NewDefaultTokenSource(*local, storage.ScopeReadWrite)
	if err != nil {
		return fmt.Errorf("Failed to get token source: %s", err)
	}
	client := auth.ClientFromTokenSource(ts)

	if storageClient, err = storage.NewClient(ctx, option.WithHTTPClient(client)); err != nil {
		return fmt.Errorf("Problem authenticating to GCS: %v", err)
	}
	issueManager = issues.NewManager(client)
	return nil
}
