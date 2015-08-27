package main

/*
	This program runs all unit tests in the repository.
*/

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/timer"
)

const (
	// This gets filled out and printed when a test fails.
	TEST_FAILURE = `
============================= TEST FAILURE =============================
Test: %s

Command: %s

Error:
%s

Full output:
------------------------------------------------------------------------
%s
------------------------------------------------------------------------
`
)

// flags
var (
	short = flag.Bool("short", false, "Whether or not to run the short version of the tests.")
)

var (
	// Error message shown when a required executable is not installed.
	ERR_NEED_INSTALL = "%s failed to run! Is it installed? Error: %v"

	// Directories in which to run "go test"
	GO_DIRS = []string{
		".",
		"alertserver",
		"ct",
		"datahopper",
		"golden",
		"perf",
		"status",
	}

	// Directories with these names are skipped when searching for tests.
	NO_CRAWL_DIR_NAMES = []string{
		".git",
		"bower_components",
		"third_party",
		"node_modules",
	}

	// Directories with these paths, relative to the checkout root, are
	// skipped when searching for tests.
	NO_CRAWL_REL_PATHS = []string{
		"common",
	}
)

// cmdTest returns a test which runs a command and fails if the command fails.
func cmdTest(cmd []string, cwd, name string) *test {
	return &test{
		Name: name,
		Cmd:  strings.Join(cmd, " "),
		run: func() (error, string) {
			command := exec.Command(cmd[0], cmd[1:]...)
			if cwd != "" {
				command.Dir = cwd
			}
			output, err := command.Output()
			if err != nil {
				if _, err2 := exec.LookPath(cmd[0]); err2 != nil {
					return fmt.Errorf(ERR_NEED_INSTALL, cmd[0], err), string(output)
				}
			}
			return err, string(output)
		},
	}
}

// goTest returns a test which runs `go test` in the given cwd.
func goTest(cwd string) *test {
	cmd := []string{"go", "test", "-v", "./go/...", "-parallel", "1"}
	if *short {
		cmd = append(cmd, "-test.short")
	}
	return cmdTest(cmd, cwd, fmt.Sprintf("go tests in %s", cwd))
}

// pythonTest returns a test which runs the given Python script and fails if
// the script fails.
func pythonTest(testPath string) *test {
	return cmdTest([]string{"python", testPath}, ".", path.Base(testPath))
}

// test is a struct which represents a single test to run.
type test struct {
	Name string
	Cmd  string
	run  func() (error, string)
}

// Run executes the function for the given test and returns an error if it fails.
func (t test) Run() error {
	defer timer.New(t.Name).Stop()
	err, output := t.run()
	if err != nil {
		return fmt.Errorf(TEST_FAILURE, t.Name, t.Cmd, err, output)
	}
	return nil
}

// Find and run tests.
func main() {
	defer common.LogPanic()
	common.Init()

	defer timer.New("Finished").Stop()

	_, filename, _, _ := runtime.Caller(0)
	rootDir := path.Dir(filename)

	// If we are running full tests make sure we have the latest
	// pdfium_test installed.
	if !*short {
		glog.Info("Installing pdfium_test if necessary.")
		pdfiumInstall := path.Join(rootDir, "pdfium", "install_pdfium.sh")
		if err := exec.Command(pdfiumInstall).Run(); err != nil {
			glog.Fatalf("Failed to install pdfium_test: %v", err)
		}
		glog.Info("Latest pdfium_test installed successfully.")
	}

	// Gather all of the tests to run.
	glog.Info("Searching for tests.")
	tests := []*test{}

	// Search for Python tests in the repo.
	if err := filepath.Walk(rootDir, func(p string, info os.FileInfo, err error) error {
		basename := path.Base(p)
		// Skip some directories.
		if info.IsDir() {
			for _, skip := range NO_CRAWL_DIR_NAMES {
				if basename == skip {
					return filepath.SkipDir
				}
			}
			for _, skip := range NO_CRAWL_REL_PATHS {
				if p == path.Join(rootDir, skip) {
					return filepath.SkipDir
				}
			}
		}
		if strings.HasSuffix(basename, "_test.py") {
			tests = append(tests, pythonTest(p))
		}
		return nil
	}); err != nil {
		glog.Fatal(err)
	}

	// Go tests.
	for _, go_dir := range GO_DIRS {
		tests = append(tests, goTest(go_dir))
	}

	// Other tests.
	tests = append(tests, cmdTest([]string{"go", "vet", "./..."}, ".", "go vet"))
	tests = append(tests, cmdTest([]string{"errcheck", "go.skia.org/infra/..."}, ".", "errcheck"))
	goimportsCmd := []string{"goimports", "-l", "."}
	tests = append(tests, &test{
		Name: "goimports",
		Cmd:  strings.Join(goimportsCmd, " "),
		run: func() (error, string) {
			command := exec.Command(goimportsCmd[0], goimportsCmd[1:]...)
			output, err := command.Output()
			outStr := strings.Trim(string(output), "\n")
			if err != nil {
				if _, err2 := exec.LookPath(goimportsCmd[0]); err2 != nil {
					return fmt.Errorf(ERR_NEED_INSTALL, goimportsCmd[0], err), outStr
				}
				return err, outStr
			}
			diffFiles := strings.Split(outStr, "\n")
			if len(diffFiles) > 0 && !(len(diffFiles) == 1 && diffFiles[0] == "") {
				return fmt.Errorf("goimports found diffs in the following files:\n  - %s", strings.Join(diffFiles, ",\n  - ")), outStr
			}
			return nil, ""

		},
	})

	// Run the tests.
	glog.Infof("Found %d tests.", len(tests))
	var mutex sync.Mutex
	errors := map[string]error{}
	var wg sync.WaitGroup
	for _, t := range tests {
		wg.Add(1)
		go func(t *test) {
			defer wg.Done()
			if err := t.Run(); err != nil {
				mutex.Lock()
				errors[t.Name] = err
				mutex.Unlock()
			}
		}(t)
	}
	wg.Wait()
	if len(errors) > 0 {
		for _, e := range errors {
			glog.Error(e)
		}
		os.Exit(1)
	}
	glog.Info("All tests succeeded.")
}
