package common

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gitinfo"
)

// BuildClangHarness builds the test harness for fuzzing using clang, pulling it from the executable
// cache if possible.  It returns the path to the executable (which should be copied somewhere else)
// and any error.
func BuildClangHarness(buildType buildskia.ReleaseType, isClean bool) (string, error) {
	glog.Infof("Building %s clang harness, or fetching from cache", buildType)
	buildVars := []string{
		fmt.Sprintf("CC=%s", config.Common.ClangPath),
		fmt.Sprintf("CXX=%s", config.Common.ClangPlusPlusPath),
	}
	return buildOrGetCachedHarness("clang", buildType, isClean, buildVars)
}

// BuildASANHarness builds the test harness for fuzzing using clang and AddressSanitizer, pulling it
// from the executable cache if possible.  It returns the path to the executable (which should be
// copied somewhere else) and any error.
func BuildASANHarness(buildType buildskia.ReleaseType, isClean bool) (string, error) {
	glog.Infof("Building %s ASAN harness, or fetching from cache", buildType)
	buildVars := []string{
		fmt.Sprintf("CC=%s", config.Common.ClangPath),
		fmt.Sprintf("CXX=%s", config.Common.ClangPlusPlusPath),
		"CXXFLAGS=-O1 -g -fsanitize=address -fno-omit-frame-pointer",
		"LDFLAGS=-g -fsanitize=address",
		ASAN_OPTIONS,
	}
	return buildOrGetCachedHarness("asan", buildType, isClean, buildVars)
}

// BuildFuzzingHarness builds the test harness for fuzzing using afl-instrumented clang, pulling it
// from the executable cache if possible.  It returns the path to the executable (which should be
// copied somewhere else) and any error.
func BuildFuzzingHarness(buildType buildskia.ReleaseType, isClean bool) (string, error) {
	glog.Infof("Building %s fuzzing harness, or fetching from cache", buildType)
	buildVars := []string{
		fmt.Sprintf("CC=%s", filepath.Join(config.Generator.AflRoot, "afl-clang")),
		fmt.Sprintf("CXX=%s", filepath.Join(config.Generator.AflRoot, "afl-clang++")),
	}

	return buildOrGetCachedHarness("afl-instrumented", buildType, isClean, buildVars)
}

// buildOrGetCachedHarness first looks into the ExecutableCache for a already built binary.  If it
// cannot find one, it triggers a build and puts it in the cache.  The cache is structured like:
// [ExecutableCachePath]/[skia-hash]/[buildType]/[buildname]
// It returns the path to the executable (which should be copied somewhere else) and any error.
// buildName is a human friendly name for this build type. buildType is Release, Debug, etc,
// buildName and buildType work together to identify a unique build (in the eyes of the cache, at
// least).  isClean is whether the build output directory should be cleared before making a new
// build.  BuildVars are the environment variables that should be set during the build.
func buildOrGetCachedHarness(buildName string, buildType buildskia.ReleaseType, isClean bool, buildVars []string) (string, error) {

	gi, err := gitinfo.NewGitInfo(config.Common.SkiaRoot, false, false)
	if err != nil {
		return "", fmt.Errorf("Could not locate git info about Skia Root %s: %s", config.Common.SkiaRoot, err)
	}
	hashes := gi.LastN(1)
	if len(hashes) != 1 {
		return "", fmt.Errorf("Could not get last git hash, instead got %q", hashes)
	}

	cache := filepath.Join(config.Common.ExecutableCachePath, hashes[0], string(buildType))
	cache, err = fileutil.EnsureDirExists(cache)
	if err != nil {
		return "", fmt.Errorf("Could not create cache dir %s: %s", cache, err)
	}

	cachedFile := filepath.Join(cache, buildName)
	if info, err := os.Stat(cachedFile); err != nil {
		if os.IsNotExist(err) {
			glog.Infof("Did not find %s %s build for revision %s in cache.  Going to build it.", buildName, buildType, hashes[0])
			if builtExePath, err := buildHarness(buildType, isClean, buildVars); err != nil {
				return "", fmt.Errorf("There was a problem building: %s", err)
			} else {
				return cachedFile, fileutil.CopyExecutable(builtExePath, cachedFile)
			}
		}
		// If it's not, Something is bad, and we should error
		return "", fmt.Errorf("There was something unexpectedly wrong with the cached executable: %s", err)
	} else if info.IsDir() {
		return "", fmt.Errorf("The cached executable %s was actually a directory.  This should not be the case", cachedFile)
	} else {
		glog.Infof("Found %s %s build for revision %s in cache", buildName, buildType, hashes[0])
		return cachedFile, nil
	}
}

// buildHarness builds the test harness for fuzzing. It activates Skia's gyp command, which creates
// the build (ninja) files for a Clang build. Then, it uses buildskia.NinjaBuild to execute the
// build. It returns the path to the executable (which should be copied somewhere else) and
// any error. buildType is Release, Debug, etc, isClean is whether the build output directory should
// be cleared before making a new build.  BuildVars are the environment variables that should be
// set during the build.
func buildHarness(buildType buildskia.ReleaseType, isClean bool, buildVars []string) (string, error) {
	// clean previous build if specified
	buildLocation := filepath.Join(config.Common.SkiaRoot, "out", string(buildType))
	if isClean {
		if err := os.RemoveAll(buildLocation); err != nil {
			return "", fmt.Errorf("Could not clear out %s before building: %s", buildLocation, err)
		}
	}

	gypCmd := &exec.Command{
		Name:      "./gyp_skia",
		Dir:       config.Common.SkiaRoot,
		LogStdout: config.Common.VerboseBuilds,
		LogStderr: config.Common.VerboseBuilds,
		Env:       append(buildVars, "GYP_DEFINES=skia_clang_build=1"),
	}

	if err := exec.Run(gypCmd); err != nil {
		return "", fmt.Errorf("Failed gyp: %s", err)
	}
	builtExe := filepath.Join(buildLocation, TEST_HARNESS_NAME)

	return builtExe, buildskia.NinjaBuild(config.Common.SkiaRoot, config.Common.DepotToolsPath, buildVars, buildType, "fuzz", 16, config.Common.VerboseBuilds)
}
