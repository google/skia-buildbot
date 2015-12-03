package common

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/exec"
)

// BuildClangDM builds the test harness for parsing skp files using clang.  If any step fails, it returns an error.
func BuildClangDM(buildType string, isClean bool) error {
	if err := os.Setenv("CC", config.Common.ClangPath); err != nil {
		return err
	}
	if err := os.Setenv("CXX", config.Common.ClangPlusPlusPath); err != nil {
		return err
	}
	return buildDM(buildType, isClean)
}

// BuildAflDM builds the test harness for parsing skp files using afl-instrumented clang.  If any step fails, it returns an error.
func BuildAflDM(buildType string) error {
	// If clang is /usr/bin/clang-3.6, you may need to run:
	// sudo ln /usr/bin/clang-3.6 /usr/bin/clang
	// sudo ln /usr/bin/clang++-3.6 /usr/bin/clang++
	// on the vm for this to work
	// TODO(kjlubick): add this to vm setup script
	if err := os.Setenv("CC", filepath.Join(config.Generator.AflRoot, "afl-clang")); err != nil {
		return err
	}
	if err := os.Setenv("CXX", filepath.Join(config.Generator.AflRoot, "afl-clang++")); err != nil {
		return err
	}
	return buildDM(buildType, true)
}

// buildDM builds the test harness for parsing skp (and other) files.
// First it creates a hard link for the gyp and cpp files. The gyp file is linked into Skia's gyp folder and the cpp file is linked into SKIA_ROOT/../fuzzer_cache/src, which is where the gyp file is configured to point.
// Then it activates Skia's gyp command, which creates the build (ninja) files.
// Finally, it runs those build files.
// If any step fails in unexpected ways, it returns an error.
func buildDM(buildType string, isClean bool) error {
	glog.Infof("Building %s dm", buildType)

	// clean previous build if specified
	buildLocation := filepath.Join("out", buildType)
	if isClean {
		if err := os.RemoveAll(filepath.Join(config.Generator.SkiaRoot, buildLocation)); err != nil {
			return fmt.Errorf("Could not clear out %s before building: %s", filepath.Join(config.Generator.SkiaRoot, buildLocation), err)
		}
	}

	gypCmd := &exec.Command{
		Name: "./gyp_skia",
		Dir:  config.Generator.SkiaRoot,
		Env:  []string{"GYP_DEFINES=skia_clang_build=1"},
	}

	// run gyp
	if err := exec.Run(gypCmd); err != nil {
		return fmt.Errorf("Failed gyp: %s", err)
	}

	ninjaCmd := &exec.Command{
		Name:      "ninja",
		Args:      []string{"-C", buildLocation, "dm"},
		LogStdout: false,
		LogStderr: false,
		Dir:       config.Generator.SkiaRoot,
	}

	// run ninja
	return exec.Run(ninjaCmd)
}
