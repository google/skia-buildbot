package common

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/exec"
)

// BuildClangHarness builds the test harness for parsing skp files using clang.  If any step fails, it returns an error.
func BuildClangHarness(buildType string, isClean bool) error {
	buildVars := []string{
		fmt.Sprintf("CC=%s", config.Common.ClangPath),
		fmt.Sprintf("CXX=%s", config.Common.ClangPlusPlusPath),
	}
	return buildHarness(buildType, isClean, buildVars)
}

// BuildFuzzingHarness builds the test harness for parsing skp files using afl-instrumented clang.  If any step fails, it returns an error.
func BuildFuzzingHarness(buildType string) error {
	buildVars := []string{
		fmt.Sprintf("CC=%s", filepath.Join(config.Generator.AflRoot, "afl-clang")),
		fmt.Sprintf("CXX=%s", filepath.Join(config.Generator.AflRoot, "afl-clang++")),
	}

	return buildHarness(buildType, true, buildVars)
}

// buildHarness builds the test harness for parsing skp (and other) files.
// First it creates a hard link for the gyp and cpp files. The gyp file is linked into Skia's gyp folder and the cpp file is linked into SKIA_ROOT/../fuzzer_cache/src, which is where the gyp file is configured to point.
// Then it activates Skia's gyp command, which creates the build (ninja) files.
// Finally, it runs those build files.
// If any step fails in unexpected ways, it returns an error.
func buildHarness(buildType string, isClean bool, buildVars []string) error {
	glog.Infof("Building %s fuzzing harness", buildType)

	// clean previous build if specified
	buildLocation := filepath.Join("out", buildType)
	if isClean {
		if err := os.RemoveAll(filepath.Join(config.Generator.SkiaRoot, buildLocation)); err != nil {
			return fmt.Errorf("Could not clear out %s before building: %s", filepath.Join(config.Generator.SkiaRoot, buildLocation), err)
		}
	}

	gypCmd := &exec.Command{
		Name:      "./gyp_skia",
		Dir:       config.Generator.SkiaRoot,
		LogStdout: false,
		LogStderr: false,
		Env:       append(buildVars, "GYP_DEFINES=skia_clang_build=1"),
	}

	// run gyp
	if err := exec.Run(gypCmd); err != nil {
		return fmt.Errorf("Failed gyp: %s", err)
	}

	ninjaPath := filepath.Join(config.Common.DepotToolsPath, "ninja")

	ninjaCmd := &exec.Command{
		Name:        ninjaPath,
		Args:        []string{"-C", buildLocation, "fuzz"},
		LogStdout:   true,
		LogStderr:   true,
		InheritPath: true,
		Dir:         config.Generator.SkiaRoot,
		Env:         buildVars,
	}

	// run ninja
	return exec.Run(ninjaCmd)
}
