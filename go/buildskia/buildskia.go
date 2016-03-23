// Utility functions for downloading, building, and compiling programs against Skia.
package buildskia

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
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

var (
	skiaRevRegex = regexp.MustCompile(".*'skia_revision': '(?P<revision>[0-9a-fA-F]{2,40})'.*")
)

// GetSkiaHash returns Skia's LKGR commit hash as recorded in chromium's DEPS file.
func GetSkiaHash() (string, error) {
	// Find Skia's LKGR commit hash.
	client := httputils.NewTimeoutClient()
	resp, err := client.Get("http://chromium.googlesource.com/chromium/src/+/master/DEPS?format=TEXT")
	if err != nil {
		return "", fmt.Errorf("Could not get Skia's LKGR: %s", err)
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Got statuscode %d while accessing Chromium's DEPS file", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Could not read Skia's LKGR: %s", err)
	}
	base64Text := make([]byte, base64.StdEncoding.EncodedLen(len(string(body))))
	l, _ := base64.StdEncoding.Decode(base64Text, []byte(string(body)))
	chromiumDepsText := string(base64Text[:l])
	if strings.Contains(chromiumDepsText, "skia_revision") {
		return skiaRevRegex.FindStringSubmatch(chromiumDepsText)[1], nil
	}
	return "", fmt.Errorf("Could not find skia_revision in Chromium DEPS file")
}

// DownloadSkia uses git to clone Skia from googlesource.com and check it out
// to the specified version.  Upon success, any dependencies needed to compile
// Skia have been installed (e.g. the latest version of gyp).
//
//   version - The git hash to check out Skia at.
//   path - The path to check Skia out into.
//   depotToolsPath - The depot_tools directory.
//   clean - If true clean out the directory before cloning Skia.
//   installDeps - If true then run tools/install_dependencies.sh before
//       sync-and-gyp.
//
// It returns an error on failure.
func DownloadSkia(version, path, depotToolsPath string, clean bool, installDeps bool) (*vcsinfo.LongCommit, error) {
	glog.Infof("Cloning Skia version %s to %s, clean: %t", version, path, clean)

	if clean {
		util.RemoveAll(filepath.Join(path))
	}

	repo, err := gitinfo.CloneOrUpdate("https://skia.googlesource.com/skia", path, false)
	if err != nil {
		return nil, fmt.Errorf("Failed cloning Skia: %s", err)
	}

	if err = repo.SetToCommit(version); err != nil {
		return nil, fmt.Errorf("Problem setting Skia to version %s: %s", version, err)
	}

	env := []string{"PATH=" + depotToolsPath + ":" + os.Getenv("PATH")}
	if installDeps {
		depsCmd := &exec.Command{
			Name:        "tools/install_dependencies.sh",
			Dir:         path,
			InheritPath: false,
			Env:         env,
			LogStderr:   true,
			LogStdout:   true,
		}

		if err := exec.Run(depsCmd); err != nil {
			return nil, fmt.Errorf("Failed installing dependencies: %s", err)
		}
	}

	syncCmd := &exec.Command{
		Name:        "bin/sync-and-gyp",
		Dir:         path,
		InheritPath: false,
		Env:         env,
		LogStderr:   true,
		LogStdout:   true,
	}

	if err := exec.Run(syncCmd); err != nil {
		return nil, fmt.Errorf("Failed syncing and setting up gyp: %s", err)
	}

	if lc, err := repo.Details(version, false); err != nil {
		return nil, fmt.Errorf("Could not get git details for skia version %s: %s", version, err)
	} else {
		return lc, nil
	}
}

// NinjaBuild builds the given target using ninja.
//
//   path - The absolute path to the Skia checkout.
//   depotToolsPath - The depot_tools directory.
//   build - The type of build to perform.
//   target -The build target, e.g. "SampleApp" or "most".
//
// Returns an error on failure.
func NinjaBuild(path, depotToolsPath string, build ReleaseType, target string, numCores int) error {
	if build == "" {
		build = "Release"
	}
	buildCmd := &exec.Command{
		Name:        "ninja",
		Args:        []string{"-C", "out/" + string(build), "-j", fmt.Sprintf("%d", numCores), target},
		Dir:         path,
		InheritPath: false,
		Env: []string{
			"PATH=" + depotToolsPath + ":" + os.Getenv("PATH"),
		},
		LogStderr: true,
		LogStdout: true,
	}
	glog.Infof("About to run: %#v", *buildCmd)

	if err := exec.Run(buildCmd); err != nil {
		return fmt.Errorf("Failed ninja build: %s", err)
	}
	return nil
}

// CMakeBuild runs /skia/cmake/cmake_build to build Skia.
//
//   path - the absolute path to the Skia checkout.
//   build - is the type of build to perform.
//
// The results of the build are stored in path/CMAKE_OUTDIR.
func CMakeBuild(path string, build ReleaseType) error {
	if build == "" {
		build = "Release"
	}
	buildCmd := &exec.Command{
		Name:        filepath.Join(path, "cmake", "cmake_build"),
		Dir:         filepath.Join(path, "cmake"),
		InheritPath: false,
		Env: []string{
			"SKIA_OUT=" + filepath.Join(path, CMAKE_OUTDIR),
			"BUILDTYPE=" + string(build),
			"PATH=" + filepath.Join(path, "cmake") + ":" + os.Getenv("PATH"),
		},
		LogStderr: true,
		LogStdout: true,
	}
	glog.Infof("About to run: %#v", *buildCmd)

	if err := exec.Run(buildCmd); err != nil {
		return fmt.Errorf("Failed cmake build: %s", err)
	}
	return nil
}

// CMakeCompileAndLink will compile the given files into an executable.
//
//   path - the absolute path to the Skia checkout.
//   out - A filename, either absolute, or relative to path, to write the exe.
//   filenames - Absolute paths to the files to compile.
//   extraLinkFlags - Entra linker flags.
//
// Should run something like:
//
//   $ c++ @skia_compile_arguments.txt fiddle_main.cpp \
//         draw.cpp @skia_link_arguments.txt -lOSMesa \
//         -o myexample
//
func CMakeCompileAndLink(path, out string, filenames []string, extraLinkFlags []string) error {
	if !filepath.IsAbs(out) {
		out = filepath.Join(path, out)
	}
	args := []string{
		fmt.Sprintf("@%s", filepath.Join(path, CMAKE_OUTDIR, CMAKE_COMPILE_ARGS_FILE)),
		// The filenames get inserted here.
		fmt.Sprintf("@%s", filepath.Join(path, CMAKE_OUTDIR, CMAKE_LINK_ARGS_FILE)),
		"-o",
		out,
	}
	if len(extraLinkFlags) > 0 {
		for _, fl := range extraLinkFlags {
			args = append(args, fl)
		}
	}
	compileCmd := &exec.Command{
		Name:        "c++",
		Args:        append(append(append([]string{}, args[0]), filenames...), args[1:]...),
		Dir:         path,
		InheritPath: true,
		LogStderr:   true,
		LogStdout:   true,
	}
	glog.Infof("About to run: %#v", *compileCmd)

	if err := exec.Run(compileCmd); err != nil {
		return fmt.Errorf("Failed compile: %s", err)
	}
	return nil
}
