package main

/*
Runs the backend portions of the fuzzer.  This includes the generator and aggregator parts (see DESIGN.md)
*/

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/aggregator"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/fileutil"
)

var (
	aflOutputPath     = flag.String("afl_output_path", "", "[REQUIRED] The output folder of afl-fuzz.  This will have N folders named fuzzer0 - fuzzerN.  Should not be in /tmp or afl-fuzz will refuse to run.")
	skiaRoot          = flag.String("skia_root", "", "[REQUIRED] The root directory of the Skia source code.")
	clangPath         = flag.String("clang_path", "", "[REQUIRED] The path to the clang executable.")
	clangPlusPlusPath = flag.String("clang_p_p_path", "", "[REQUIRED] The path to the clang++ executable.")
	aflRoot           = flag.String("afl_root", "", "[REQUIRED] The install directory of afl-fuzz (v1.94b or later).")
	numFuzzProcesses  = flag.Int("fuzz_processes", 0, `The number of processes to run afl-fuzz.  This should be fewer than the number of logical cores.  Defaults to 0, which means "Make an intelligent guess"`)
	skipGeneration    = flag.Bool("debug_skip_generation", false, "(debug only) If the generation step should be skipped")

	binaryFuzzPath          = flag.String("fuzz_path", filepath.Join(os.TempDir(), "fuzzes"), "The directory to temporarily store the binary fuzzes during aggregation.")
	executablePath          = flag.String("executable_path", filepath.Join(os.TempDir(), "executables"), "The directory to store temporary executables that will run the fuzzes during aggregation. Defaults to /tmp/executables.")
	numAggregationProcesses = flag.Int("aggregation_processes", 0, `The number of processes to aggregate fuzzes.  This should be fewer than the number of logical cores.  Defaults to 0, which means "Make an intelligent guess"`)
	rescanPeriod            = flag.Duration("rescan_period", 60*time.Second, `The time in which to sleep for every cycle of aggregation. `)
	analysisTimeout         = flag.Duration("analysis_timeout", 5*time.Second, `The maximum time an analysis should run.`)
)

var requiredFlags = []string{"afl_output_path", "skia_root", "clang_path", "clang_p_p_path", "afl_root"}

func main() {
	flag.Parse()
	if err := writeFlagsToConfig(); err != nil {
		glog.Fatalf("Problem with configuration: %v", err)
	}
	if !*skipGeneration {
		glog.Infof("Starting generator with configuration %#v", config.Generator)
		//TODO(kjlubick): Start AFL-fuzz
	}
	glog.Infof("Starting aggregator with configuration %#v", config.Aggregator)
	glog.Fatal(aggregator.StartBinaryAggregator())
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
	config.Generator.SkiaRoot, err = fileutil.EnsureDirExists(*skiaRoot)
	if err != nil {
		return err
	}
	config.Generator.AflRoot, err = fileutil.EnsureDirExists(*aflRoot)
	if err != nil {
		return err
	}
	config.Generator.ClangPath = *clangPath
	config.Generator.ClangPlusPlusPath = *clangPlusPlusPath
	config.Generator.NumFuzzProcesses = *numFuzzProcesses

	config.Aggregator.BinaryFuzzPath, err = fileutil.EnsureDirExists(*binaryFuzzPath)
	if err != nil {
		return err
	}
	config.Aggregator.ExecutablePath, err = fileutil.EnsureDirExists(*executablePath)
	if err != nil {
		return err
	}
	config.Aggregator.NumAggregationProcesses = *numAggregationProcesses
	config.Aggregator.RescanPeriod = *rescanPeriod
	config.Aggregator.AnalysisTimeout = *analysisTimeout
	return nil
}
