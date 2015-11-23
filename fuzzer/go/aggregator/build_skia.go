package aggregator

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
	if err := os.Setenv("CC", filepath.Join(config.Generator.ClangPath)); err != nil {
		return err
	}
	if err := os.Setenv("CXX", filepath.Join(config.Generator.ClangPlusPlusPath)); err != nil {
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

	// Change directory to skia folder
	if err := os.Chdir(config.Generator.SkiaRoot); err != nil {
		return nil
	}

	// clean previous build if specified
	buildLocation := filepath.Join("out", buildType)
	if isClean {
		if err := os.RemoveAll(buildLocation); err != nil {
			return err
		}
	}

	// run gyp
	if message, err := exec.RunSimple("./gyp_skia"); err != nil {
		glog.Errorf("Failed gyp message: %s", message)
		return err
	}

	// run ninja
	cmd := fmt.Sprintf("ninja -C %s dm", buildLocation)
	if message, err := exec.RunSimple(cmd); err != nil {
		glog.Errorf("Failed ninja message: %s", message)
		return err
	}
	return nil
}
