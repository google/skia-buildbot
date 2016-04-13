package common

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/exec"
)

// BuildClangHarness builds the test harness for parsing skp files using clang. If any step fails,
// it returns an error.
func BuildClangHarness(buildType buildskia.ReleaseType, isClean bool) error {
	glog.Infof("Building %s clang harness", buildType)
	buildVars := []string{
		fmt.Sprintf("CC=%s", config.Common.ClangPath),
		fmt.Sprintf("CXX=%s", config.Common.ClangPlusPlusPath),
	}
	return buildHarness(buildType, isClean, buildVars)
}

// BuildASANHarness builds the test harness for parsing skp files using clang and AddressSanitizer.
// If any step fails, it returns an error.
func BuildASANHarness(buildType buildskia.ReleaseType, isClean bool) error {
	glog.Infof("Building %s ASAN harness", buildType)
	buildVars := []string{
		fmt.Sprintf("CC=%s", config.Common.ClangPath),
		fmt.Sprintf("CXX=%s", config.Common.ClangPlusPlusPath),
		"CXXFLAGS=-O1 -g -fsanitize=address -fno-omit-frame-pointer",
		"LDFLAGS=-g -fsanitize=address",
		ASAN_OPTIONS,
	}
	return buildHarness(buildType, isClean, buildVars)
}

// BuildFuzzingHarness builds the test harness for parsing skp files using afl-instrumented clang.
// If any step fails, it returns an error.
func BuildFuzzingHarness(buildType buildskia.ReleaseType, isClean bool) error {
	glog.Infof("Building %s fuzzing harness", buildType)
	buildVars := []string{
		fmt.Sprintf("CC=%s", filepath.Join(config.Generator.AflRoot, "afl-clang")),
		fmt.Sprintf("CXX=%s", filepath.Join(config.Generator.AflRoot, "afl-clang++")),
	}

	return buildHarness(buildType, isClean, buildVars)
}

// buildHarness builds the test harness for parsing skp (and other) files.
// It activates Skia's gyp command, which creates the build (ninja) files.
// Then, it uses Ninja on those build files to create the build.
// If any step fails in unexpected ways, it returns an error.
func buildHarness(buildType buildskia.ReleaseType, isClean bool, buildVars []string) error {
	// clean previous build if specified
	buildLocation := filepath.Join("out", string(buildType))
	if isClean {
		if err := os.RemoveAll(filepath.Join(config.Generator.SkiaRoot, buildLocation)); err != nil {
			return fmt.Errorf("Could not clear out %s before building: %s", filepath.Join(config.Generator.SkiaRoot, buildLocation), err)
		}
	}

	gypCmd := &exec.Command{
		Name:      "./gyp_skia",
		Dir:       config.Generator.SkiaRoot,
		LogStdout: config.Common.VerboseBuilds,
		LogStderr: config.Common.VerboseBuilds,
		Env:       append(buildVars, "GYP_DEFINES=skia_clang_build=1"),
	}

	// run gyp
	if err := exec.Run(gypCmd); err != nil {
		return fmt.Errorf("Failed gyp: %s", err)
	}

	return buildskia.NinjaBuild(config.Generator.SkiaRoot, config.Common.DepotToolsPath, buildVars, buildType, "fuzz", 16, config.Common.VerboseBuilds)
}
