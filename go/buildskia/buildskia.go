// Utility functions for downloading, building, and compiling programs against Skia.
package buildskia

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/util/limitwriter"
	"go.skia.org/infra/go/vcsinfo"
)

type ReleaseType string

// ReleaseType constants.
const (
	RELEASE_BUILD           ReleaseType = "Release"
	DEBUG_BUILD             ReleaseType = "Debug"
	RELEASE_DEVELOPER_BUILD ReleaseType = "Release_Developer"
)

const (
	CMAKE_OUTDIR            = "cmakeout"
	CMAKE_COMPILE_ARGS_FILE = "skia_compile_arguments.txt"
	CMAKE_LINK_ARGS_FILE    = "skia_link_arguments.txt"
)

// GetSkiaHead returns the most recent commit hash on Skia's main branch.
//
// If client is nil then a default timeout client is used.
func GetSkiaHead(client *http.Client) (string, error) {
	head, err := gitiles.NewRepo(common.REPO_SKIA, client).Details(context.TODO(), git.DefaultBranch)
	if err != nil {
		return "", skerr.Wrapf(err, "Could not get Skia's HEAD")
	}
	return head.Hash, nil
}

// GetSkiaHash returns Skia's LKGR commit hash as recorded in chromium's DEPS file.
//
// If client is nil then a default timeout client is used.
func GetSkiaHash(client *http.Client) (string, error) {
	depsContents, err := gitiles.NewRepo(common.REPO_CHROMIUM, client).ReadFile(context.TODO(), deps_parser.DepsFileName)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to read Chromium DEPS")
	}
	dep, err := deps_parser.GetDep(string(depsContents), common.REPO_SKIA)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to get Skia's LKGR")
	}
	return dep.Version, nil
}

// DownloadSkia uses git to clone Skia from googlesource.com and check it out
// to the specified gitHash for the specified branch. Upon success, any
// dependencies needed to compile Skia have been installed (e.g. the latest
// version of gyp).
//
//   branch - The empty string signifies the main branch.
//   gitHash - The git hash to check out Skia at.
//   path - The path to check Skia out into.
//   depotToolsPath - The depot_tools directory.
//   clean - If true clean out the directory before cloning Skia.
//   installDeps - If true then run tools/install_dependencies.sh before
//       sync. The calling user should be sudo capable.
//
// It returns an error on failure.
func DownloadSkia(ctx context.Context, branch, gitHash, path, depotToolsPath string, clean bool, installDeps bool) (*vcsinfo.LongCommit, error) {
	sklog.Infof("Cloning Skia gitHash %s to %s, clean: %t", gitHash, path, clean)

	if clean {
		util.RemoveAll(filepath.Join(path))
	}

	repo, err := gitinfo.CloneOrUpdate(ctx, "https://skia.googlesource.com/skia", path, true)
	if err != nil {
		return nil, fmt.Errorf("Failed cloning Skia: %s", err)
	}

	if branch != "" {
		if err := repo.Checkout(ctx, branch); err != nil {
			return nil, fmt.Errorf("Failed to change to branch %s: %s", branch, err)
		}
	}

	if err = repo.Reset(ctx, gitHash); err != nil {
		return nil, fmt.Errorf("Problem setting Skia to gitHash %s: %s", gitHash, err)
	}

	env := []string{"PATH=" + depotToolsPath + ":" + os.Getenv("PATH")}
	if installDeps {
		depsCmd := &exec.Command{
			Name:        "sudo",
			Args:        []string{"tools/install_dependencies.sh"},
			Dir:         path,
			InheritPath: false,
			Env:         env,
			LogStderr:   true,
			LogStdout:   true,
		}

		if err := exec.Run(ctx, depsCmd); err != nil {
			return nil, fmt.Errorf("Failed installing dependencies: %s", err)
		}
	}

	syncCmd := &exec.Command{
		Name:        "bin/sync",
		Dir:         path,
		InheritPath: false,
		Env:         env,
		LogStderr:   true,
		LogStdout:   true,
	}

	if err := exec.Run(ctx, syncCmd); err != nil {
		return nil, fmt.Errorf("Failed syncing and setting up gyp: %s", err)
	}

	if lc, err := repo.Details(ctx, gitHash, false); err != nil {
		return nil, fmt.Errorf("Could not get git details for skia gitHash %s: %s", gitHash, err)
	} else {
		return lc, nil
	}
}

// GNDownloadSkia uses depot_tools fetch to clone Skia from googlesource.com
// and check it out to the specified gitHash for the specified branch. Upon
// success, any dependencies needed to compile Skia have been installed.
//
//   branch - The empty string signifies the main branch.
//   gitHash - The git hash to check out Skia at.
//   path - The path to check Skia out into.
//   depotToolsPath - The depot_tools directory.
//   clean - If true clean out the directory before cloning Skia.
//   installDeps - If true then run tools/install_dependencies.sh before
//       syncing. The calling user should be sudo capable.
//
// It returns an error on failure.
func GNDownloadSkia(ctx context.Context, branch, gitHash, path, depotToolsPath string, clean bool, installDeps bool) (*vcsinfo.LongCommit, error) {
	sklog.Infof("Cloning Skia gitHash %s to %s, clean: %t", gitHash, path, clean)

	if clean {
		util.RemoveAll(filepath.Join(path))
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("Failed to create dir for checkout: %s", err)
	}

	env := []string{"PATH=" + depotToolsPath + ":" + os.Getenv("PATH")}
	fetchCmd := &exec.Command{
		Name:        filepath.Join(depotToolsPath, "fetch"),
		Args:        []string{"skia"},
		Dir:         path,
		InheritPath: false,
		Env:         env,
		LogStderr:   true,
		LogStdout:   true,
	}

	if err := exec.Run(ctx, fetchCmd); err != nil {
		// Failing to fetch might be because we already have Skia checked out here.
		sklog.Warningf("Failed to fetch skia: %s", err)
	}

	repoPath := filepath.Join(path, "skia")
	repo, err := gitinfo.NewGitInfo(ctx, repoPath, false, true)
	if err != nil {
		return nil, fmt.Errorf("Failed working with Skia repo: %s", err)
	}

	if err = repo.Reset(ctx, gitHash); err != nil {
		return nil, fmt.Errorf("Problem setting Skia to gitHash %s: %s", gitHash, err)
	}

	if installDeps {
		depsCmd := &exec.Command{
			Name:        "sudo",
			Args:        []string{"tools/install_dependencies.sh"},
			Dir:         repoPath,
			InheritPath: false,
			Env:         env,
			LogStderr:   true,
			LogStdout:   true,
		}

		if err := exec.Run(ctx, depsCmd); err != nil {
			return nil, fmt.Errorf("Failed installing dependencies: %s", err)
		}
	}

	syncCmd := &exec.Command{
		Name:        filepath.Join(depotToolsPath, "gclient"),
		Args:        []string{"sync"},
		Dir:         path,
		InheritPath: false,
		Env:         env,
		LogStderr:   true,
		LogStdout:   true,
	}

	if err := exec.Run(ctx, syncCmd); err != nil {
		return nil, fmt.Errorf("Failed syncing: %s", err)
	}

	fetchGn := &exec.Command{
		Name:        filepath.Join(repoPath, "bin", "fetch-gn"),
		Args:        []string{},
		Dir:         repoPath,
		InheritPath: false,
		Env:         []string{"PATH=" + depotToolsPath + ":" + os.Getenv("PATH")},
		LogStderr:   true,
		LogStdout:   true,
	}

	if err := exec.Run(ctx, fetchGn); err != nil {
		return nil, fmt.Errorf("Failed installing dependencies: %s", err)
	}

	if lc, err := repo.Details(ctx, gitHash, false); err != nil {
		return nil, fmt.Errorf("Could not get git details for skia gitHash %s: %s", gitHash, err)
	} else {
		return lc, nil
	}
}

// GNGen runs GN on Skia.
//
//   path       - The absolute path to the Skia checkout, should be the same
//                path passed to DownloadSkiaGN.
//   depotTools - The path to depot_tools.
//   outSubDir  - The name of the sub-directory under 'out' to build in.
//   args       - A slice of strings to pass to gn --args. See the skia
//                BUILD.gn and https://skia.org/user/quick/gn.
//
// The results of the build are stored in path/skia/out/<outSubDir>.
func GNGen(ctx context.Context, path, depotTools, outSubDir string, args []string) error {
	genCmd := &exec.Command{
		Name:        filepath.Join(depotTools, "gn"),
		Args:        []string{"gen", filepath.Join("out", outSubDir), fmt.Sprintf(`--args=%s`, strings.Join(args, " "))},
		Dir:         filepath.Join(path, "skia"),
		InheritPath: false,
		Env: []string{
			"PATH=" + depotTools + ":" + os.Getenv("PATH"),
		},
		LogStderr: true,
		LogStdout: true,
	}
	sklog.Infof("About to run: %#v", *genCmd)

	if err := exec.Run(ctx, genCmd); err != nil {
		return fmt.Errorf("Failed gn gen: %s", err)
	}
	return nil
}

// GNNinjaBuild builds the given target using ninja.
//
//   path - The absolute path to the Skia checkout as passed into DownloadSkiaGN.
//   depotToolsPath - The depot_tools directory.
//   outSubDir - The name of the sub-directory under 'out' to build in.
//   target - The specific target to build. Pass in the empty string to build all targets.
//   verbose - If the build's std out should be logged (usally quite long)
//
// Returns the build logs and any errors on failure.
func GNNinjaBuild(ctx context.Context, path, depotToolsPath, outSubDir, target string, verbose bool) (string, error) {
	args := []string{"-C", filepath.Join("out", outSubDir)}
	if target != "" {
		args = append(args, target)
	}
	buf := bytes.Buffer{}
	output := limitwriter.New(&buf, 64*1024)
	buildCmd := &exec.Command{
		Name:           filepath.Join(depotToolsPath, "ninja"),
		Args:           args,
		Dir:            filepath.Join(path, "skia"),
		InheritPath:    false,
		Env:            []string{"PATH=" + depotToolsPath + ":" + os.Getenv("PATH")},
		CombinedOutput: output,
		LogStderr:      true,
		LogStdout:      verbose,
	}
	sklog.Infof("About to run: %#v", *buildCmd)

	if err := exec.Run(ctx, buildCmd); err != nil {
		return buf.String(), fmt.Errorf("Failed compile: %s", err)
	}
	return buf.String(), nil
}

// NinjaBuild builds the given target using ninja.
//
//   skiaPath - The absolute path to the Skia checkout.
//   depotToolsPath - The depot_tools directory.
//   extraEnv - Any additional environment variables that need to be set.  Can be nil.
//   build - The type of build to perform.
//   target - The build target, e.g. "SampleApp" or "most".
//   verbose - If the build's std out should be logged (usally quite long)
//
// Returns an error on failure.
func NinjaBuild(ctx context.Context, skiaPath, depotToolsPath string, extraEnv []string, build ReleaseType, target string, numCores int, verbose bool) error {
	buildCmd := &exec.Command{
		Name:        filepath.Join(depotToolsPath, "ninja"),
		Args:        []string{"-C", "out/" + string(build), "-j", fmt.Sprintf("%d", numCores), target},
		Dir:         skiaPath,
		InheritPath: false,
		Env: append(extraEnv,
			"PATH="+depotToolsPath+":"+os.Getenv("PATH"),
		),
		LogStderr: true,
		LogStdout: verbose,
	}
	sklog.Infof("About to run: %#v", *buildCmd)

	if err := exec.Run(ctx, buildCmd); err != nil {
		return fmt.Errorf("Failed ninja build: %s", err)
	}
	return nil
}

// CMakeBuild runs /skia/cmake/cmake_build to build Skia.
//
//   path       - The absolute path to the Skia checkout.
//   depotTools - The path to depot_tools.
//   build      - Is the type of build to perform.
//
// The results of the build are stored in path/CMAKE_OUTDIR.
func CMakeBuild(ctx context.Context, path, depotTools string, build ReleaseType) error {
	if build == "" {
		build = "Release"
	}
	buildCmd := &exec.Command{
		Name:        filepath.Join(path, "cmake", "cmake_build"),
		Dir:         filepath.Join(path, "cmake"),
		InheritPath: false,
		Env: []string{
			"SKIA_OUT=" + filepath.Join(path, CMAKE_OUTDIR), // Note that cmake_build will put the results in a sub-directory
			// that is the build type.
			"BUILDTYPE=" + string(build),
			"PATH=" + filepath.Join(path, "cmake") + ":" + depotTools + ":" + os.Getenv("PATH"),
		},
		LogStderr: true,
		LogStdout: true,
	}
	sklog.Infof("About to run: %#v", *buildCmd)

	if err := exec.Run(ctx, buildCmd); err != nil {
		return fmt.Errorf("Failed cmake build: %s", err)
	}
	return nil
}

// CMakeCompileAndLink will compile the given files into an executable.
//
//   path - the absolute path to the Skia checkout.
//   out - A filename, either absolute, or relative to path, to write the exe.
//   filenames - Absolute paths to the files to compile.
//   extraIncludeDirs - Entra directories to search for includes.
//   extraLinkFlags - Entra linker flags.
//
// Returns the stdout+stderr of the compiler and a non-nil error if the compile failed.
//
// Should run something like:
//
//   $ c++ @skia_compile_arguments.txt fiddle_main.cpp \
//         draw.cpp @skia_link_arguments.txt -lOSMesa \
//         -o myexample
//
func CMakeCompileAndLink(ctx context.Context, path, out string, filenames []string, extraIncludeDirs []string, extraLinkFlags []string, build ReleaseType) (string, error) {
	if !filepath.IsAbs(out) {
		out = filepath.Join(path, out)
	}
	args := []string{
		fmt.Sprintf("@%s", filepath.Join(path, CMAKE_OUTDIR, string(build), CMAKE_COMPILE_ARGS_FILE)),
	}
	if len(extraIncludeDirs) > 0 {
		args = append(args, "-I"+strings.Join(extraIncludeDirs, ","))
	}
	for _, fn := range filenames {
		args = append(args, fn)
	}
	moreArgs := []string{
		fmt.Sprintf("@%s", filepath.Join(path, CMAKE_OUTDIR, string(build), CMAKE_LINK_ARGS_FILE)),
		"-o",
		out,
	}
	for _, s := range moreArgs {
		args = append(args, s)
	}
	if len(extraLinkFlags) > 0 {
		for _, fl := range extraLinkFlags {
			args = append(args, fl)
		}
	}
	buf := bytes.Buffer{}
	output := limitwriter.New(&buf, 64*1024)
	compileCmd := &exec.Command{
		Name:           "c++",
		Args:           args,
		Dir:            path,
		InheritPath:    true,
		CombinedOutput: output,
		Timeout:        10 * time.Second,
		LogStderr:      true,
		LogStdout:      true,
	}
	sklog.Infof("About to run: %#v", *compileCmd)

	if err := exec.Run(ctx, compileCmd); err != nil {
		return buf.String(), fmt.Errorf("Failed compile: %s", err)
	}
	return buf.String(), nil
}

// CMakeCompile will compile the given files into an executable.
//
//   path - the absolute path to the Skia checkout.
//   out - A filename, either absolute, or relative to path, to write the .o file.
//   filenames - Absolute paths to the files to compile.
//
// Should run something like:
//
//   $ c++ @skia_compile_arguments.txt fiddle_main.cpp \
//         -o fiddle_main.o
//
func CMakeCompile(ctx context.Context, path, out string, filenames []string, extraIncludeDirs []string, build ReleaseType) error {
	if !filepath.IsAbs(out) {
		out = filepath.Join(path, out)
	}
	args := []string{
		"-c",
		fmt.Sprintf("@%s", filepath.Join(path, CMAKE_OUTDIR, string(build), CMAKE_COMPILE_ARGS_FILE)),
	}
	if len(extraIncludeDirs) > 0 {
		args = append(args, "-I"+strings.Join(extraIncludeDirs, ","))
	}
	for _, fn := range filenames {
		args = append(args, fn)
	}
	moreArgs := []string{
		"-o",
		out,
	}
	for _, s := range moreArgs {
		args = append(args, s)
	}
	compileCmd := &exec.Command{
		Name:        "c++",
		Args:        args,
		Dir:         path,
		InheritPath: true,
		LogStderr:   true,
		LogStdout:   true,
	}
	sklog.Infof("About to run: %#v", *compileCmd)

	if err := exec.Run(ctx, compileCmd); err != nil {
		return fmt.Errorf("Failed compile: %s", err)
	}
	return nil
}
