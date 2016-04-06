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

// argsAfterExecutable is a map of arguments that come after the executable
// and before the path to the bytes file, that will be fuzzed.
var argsAfterExecutable = map[string][]string{
	"api_parse_path":   []string{"--type", "api", "--name", "ParsePath", "--bytes"},
	"api_image_filter": []string{"--type", "api", "--name", "SerializedImageFilter", "--bytes"},
	"skcodec_scale":    []string{"--type", "image_scale", "--bytes"},
	"skcodec_mode":     []string{"--type", "image_mode", "--bytes"},
	"skpicture":        []string{"--type", "skp", "--bytes"},
}

// AnalysisArgsFor creates an appropriate analysis command for the category of fuzz specified given
// the passed in variables. It is expected that these arguments will be executed with GNU timeout
// GNU timeout is used instead of the option on exec.Command because experimentation with the latter
// showed evidence of that way leaking processes, which lead to OOM errors. GNU catchsegv generates
// human readable dumps of crashes, which can then be scanned for stacktrace information.
func AnalysisArgsFor(category string, pathToExecutable, pathToFile string) AnalysisArgs {
	timeoutInSeconds := fmt.Sprintf("%ds", config.Aggregator.AnalysisTimeout/time.Second)
	args, found := argsAfterExecutable[category]
	if !found {
		glog.Errorf("Unknown fuzz category %q", category)
		return nil
	}
	cmd := append([]string{timeoutInSeconds, "catchsegv", pathToExecutable}, args...)
	return append(cmd, pathToFile)
}

// GenerationArgsFor creates the appropriate arguments to run afl-fuzz on a fuzz of the given
// category. We set the maximum memory to 5GB to avoid all but the most extreme cases of memory
// problems. The timeout is set at 100ms (typical execution times for most fuzzes are around 20ms)
// which is short enough to not bog down afl-fuzz in the weeds of a long running time, but long
// enough to accomodate typical execution paths.
func GenerationArgsFor(category, pathToExecutable, fuzzerName string, isMaster bool) GenerationArgs {
	args, found := argsAfterExecutable[category]
	if !found {
		glog.Errorf("Unknown fuzz category %q", category)
		return nil
	}
	masterFlag := "-M"
	if !isMaster {
		masterFlag = "-S"
	}
	seedPath := filepath.Join(config.Generator.FuzzSamples, category)
	outputPath := filepath.Join(config.Generator.AflOutputPath, category)
	cmd := append([]string{"-i", seedPath, "-o", outputPath, "-m", "5000", masterFlag, fuzzerName, "--", pathToExecutable}, args...)
	return append(cmd, "@@")
}
