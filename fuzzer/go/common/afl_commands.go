package common

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
)

type AnalysisArgs []string
type GenerationArgs []string

// AnalysisArgsFor creates an appropriate analysis command for the category of fuzz specified given
// the passed in variables. It is expected that these arguments will be executed with GNU timeout
// GNU timeout is used instead of the option on exec.Command because experimentation with the latter
// showed evidence of that way leaking processes, which lead to OOM errors. GNU catchsegv generates
// human readable dumps of crashes, which can then be scanned for stacktrace information.
func AnalysisArgsFor(category string, pathToExecutable, pathToFile string) AnalysisArgs {
	timeoutInSeconds := fmt.Sprintf("%ds", config.Aggregator.AnalysisTimeout/time.Second)
	if category == "skpicture" {
		return []string{timeoutInSeconds, "catchsegv", pathToExecutable, "--type", "skp", "--bytes", pathToFile}
	}
	glog.Errorf("Unknown fuzz category %q", category)
	return nil
}

// GenerationArgsFor creates the appropriate arguments to run afl-fuzz on a fuzz of the given
// category. We set the maximum memory to 5GB to avoid all but the most extreme cases of memory
// problems. The timeout is set at 100ms (typical execution times for most fuzzes are around 20ms)
// which is short enough to not bog down afl-fuzz in the weeds of a long running time, but long
// enough to accomodate typical execution paths.
func GenerationArgsFor(category, pathToExecutable, fuzzerName string, isMaster bool) GenerationArgs {
	masterFlag := "-M"
	if !isMaster {
		masterFlag = "-S"
	}
	seedPath := filepath.Join(config.Generator.FuzzSamples, category)
	outputPath := filepath.Join(config.Generator.AflOutputPath, category)
	if category == "skpicture" {
		return []string{"-i", seedPath, "-o", outputPath, "-m", "5000", "-t", "100", masterFlag, fuzzerName, "--", pathToExecutable, "--type", "skp", "--bytes", "@@"}
	}
	glog.Errorf("Unknown fuzz category %q", category)
	return nil
}
