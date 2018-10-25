package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
)

// BuildClangHarness builds the test harness for fuzzing using clang, pulling it from the executable
// cache if possible.  It returns the path to the executable (which should be copied somewhere else)
// and any error.
func BuildClangHarness(ctx context.Context, buildType buildskia.ReleaseType, isClean bool) (string, error) {
	sklog.Infof("Building %s clang harness, or fetching from cache", buildType)
	buildArgs := []string{
		fmt.Sprintf("cc=%q", config.Common.ClangPath),
		fmt.Sprintf("cxx=%q", config.Common.ClangPlusPlusPath),
	}
	return buildOrGetCachedHarness(ctx, "clang", buildType, isClean, buildArgs)
}

// BuildASANHarness builds the test harness for fuzzing using clang and AddressSanitizer, pulling it
// from the executable cache if possible.  It returns the path to the executable (which should be
// copied somewhere else) and any error.
func BuildASANHarness(ctx context.Context, buildType buildskia.ReleaseType, isClean bool) (string, error) {
	sklog.Infof("Building %s ASAN harness, or fetching from cache", buildType)
	buildArgs := []string{
		fmt.Sprintf("cc=%q", config.Common.ClangPath),
		fmt.Sprintf("cxx=%q", config.Common.ClangPlusPlusPath),
		`sanitize="address"`, // No UBSAN, to avoid noise.
	}
	return buildOrGetCachedHarness(ctx, "asan", buildType, isClean, buildArgs)
}

// BuildFuzzingHarness builds the test harness for fuzzing using afl-instrumented clang, pulling it
// from the executable cache if possible.  It returns the path to the executable (which should be
// copied somewhere else) and any error.
func BuildFuzzingHarness(ctx context.Context, buildType buildskia.ReleaseType, isClean bool) (string, error) {
	sklog.Infof("Building %s fuzzing harness, or fetching from cache", buildType)

	buildArgs := []string{
		fmt.Sprintf("cc=%q", filepath.Join(config.Generator.AflRoot, "afl-clang")),
		fmt.Sprintf("cxx=%q", filepath.Join(config.Generator.AflRoot, "afl-clang++")),
	}
	if fileutil.FileExists(filepath.Join(config.Generator.AflRoot, "afl-clang-fast")) &&
		fileutil.FileExists(filepath.Join(config.Generator.AflRoot, "afl-clang-fast++")) {
		buildArgs = []string{
			fmt.Sprintf("cc=%q", filepath.Join(config.Generator.AflRoot, "afl-clang-fast")),
			fmt.Sprintf("cxx=%q", filepath.Join(config.Generator.AflRoot, "afl-clang-fast++")),
		}
	}

	return buildOrGetCachedHarness(ctx, "afl-instrumented", buildType, isClean, buildArgs)
}

// buildOrGetCachedHarness first looks into the ExecutableCache for a already built binary.  If it
// cannot find one, it triggers a build and puts it in the cache.  The cache is structured like:
// [ExecutableCachePath]/[skia-hash]/[buildType]/[buildname]
// It returns the path to the executable (which should be copied somewhere else) and any error.
// buildName is a human friendly name for this build type. buildType is Release, Debug, etc,
// buildName and buildType work together to identify a unique build (in the eyes of the cache, at
// least).  isClean is whether the build output directory should be cleared before making a new
// build.  buildArgs are the arguments passed to GN.
func buildOrGetCachedHarness(ctx context.Context, buildName string, buildType buildskia.ReleaseType, isClean bool, buildArgs []string) (string, error) {
	if buildType == buildskia.RELEASE_BUILD {
		buildArgs = append(buildArgs, "is_debug=false", "skia_enable_skottie=true")
	}
	// This makes crashing because we ran out of memory or because someone called SK_ABORT turn
	// into an exit(1), so we don't count it as a "crash".
	buildArgs = append(buildArgs, `extra_cflags=["-DIS_FUZZING", "-DIS_FUZZING_WITH_AFL"]`)
	// System freetype has many MSAN-like bugs, which can throw off our fuzzer. Build our own
	// (newer) freetype to minimize these.
	buildArgs = append(buildArgs, "skia_use_system_freetype2=false")
	// Experimental gif codec
	buildArgs = append(buildArgs, "skia_use_wuffs=true")

	d := filepath.Join(config.Common.SkiaRoot, "skia")
	gi, err := gitinfo.NewGitInfo(ctx, d, false, false)
	if err != nil {
		return "", fmt.Errorf("Could not locate git info about Skia Root %s: %s", d, err)
	}
	hashes := gi.LastN(ctx, 1)
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
			sklog.Infof("Did not find %s %s build for revision %s in cache.  Going to build it.", buildName, buildType, hashes[0])
			if builtExePath, err := buildHarness(ctx, buildType, isClean, buildArgs); err != nil {
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
		sklog.Infof("Found %s %s build for revision %s in cache", buildName, buildType, hashes[0])
		return cachedFile, nil
	}
}

// buildHarnesGNs builds the test harness for fuzzing. It activates Skia's GN command, which creates
// the build (ninja) files for a Clang build. Then, it uses buildskia.GNNinjaBuild to execute the
// build. It returns the path to the executable (which should be copied somewhere else) and
// any error. buildType is Release, Debug, etc, isClean is whether the build output directory should
// be cleared before making a new build. buildArgs are the arguments that should be passed into GN.
func buildHarness(ctx context.Context, buildType buildskia.ReleaseType, isClean bool, buildArgs []string) (string, error) {
	// clean previous build if specified

	buildLocation := filepath.Join(config.Common.SkiaRoot, "skia", "out", string(buildType))
	if isClean {
		if err := os.RemoveAll(buildLocation); err != nil {
			return "", fmt.Errorf("Could not clear out %s before building: %s", buildLocation, err)
		}
	}

	if err := buildskia.GNGen(ctx, config.Common.SkiaRoot, config.Common.DepotToolsPath, string(buildType), buildArgs); err != nil {
		return "", fmt.Errorf("Failed GN: %s", err)
	}

	builtExe := filepath.Join(buildLocation, TEST_HARNESS_NAME)

	_, err := buildskia.GNNinjaBuild(ctx, config.Common.SkiaRoot, config.Common.DepotToolsPath, string(buildType), TEST_HARNESS_NAME, config.Common.VerboseBuilds)
	return builtExe, err
}
