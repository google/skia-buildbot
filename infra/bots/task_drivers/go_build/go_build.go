package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/test2json"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

/*
	Build all Golang binaries and tests for the given platform(s).
*/

var (
	// Required properties for this task.
	projectID       = flag.String("project-id", "", "ID of the Google Cloud project.")
	taskID          = flag.String("task-id", "", "ID of this task.")
	taskName        = flag.String("task-name", "", "Name of the task.")
	outputPath      = flag.String("output-path", "", "Write binaries to this path.")
	outputWhitelist = common.NewMultiStringFlag("output-whitelist", nil, "Include these binaries in output-path.")

	// Optional flags.
	local          = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output         = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
	buildFlagsList = flag.String("build-flags", "", "Quoted list of Go build flags.")
	targets        = common.NewMultiStringFlag("target", nil, "Targets to pass to \"go build\"")
)

var (
	// copyTestPathRegex matches the output of //go_deps/copy_test.sh. It needs
	// To be kept in sync.
	copyTestPathRegex = regexp.MustCompile(`Wrote test executable (.+)`)
)

func main() {
	// Setup.
	ctx := td.StartRun(projectID, taskID, taskName, output, local)
	defer td.EndRun(ctx)
	if *outputPath == "" {
		td.Fatalf(ctx, "--output-path is required")
	}
	if *targets == nil {
		*targets = []string{"./..."}
	}
	// Create a temporary directory inside the outputPath. This is required
	// because the outputPath may not be located on the same device as /tmp,
	// which causes os.Rename to fail.
	tmp, err := os_steps.TempDir(ctx, *outputPath, "tmp")
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer func() {
		if err := os_steps.RemoveAll(ctx, tmp); err != nil {
			td.Fatal(ctx, err)
		}
	}()
	binOutPath := filepath.Join(*outputPath, "bin")
	binTmpPath := filepath.Join(tmp, "bin")
	if err := os_steps.MkdirAll(ctx, binTmpPath); err != nil {
		td.Fatal(ctx, err)
	}
	testOutPath := filepath.Join(*outputPath, "test")
	if err := os_steps.MkdirAll(ctx, testOutPath); err != nil {
		td.Fatal(ctx, err)
	}
	testTmpPath := filepath.Join(tmp, "test")
	if err := os_steps.MkdirAll(ctx, testTmpPath); err != nil {
		td.Fatal(ctx, err)
	}
	cwd, err := os_steps.Abs(ctx, ".")
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Before building, generate rice-box.go files where needed.
	// TODO(borenet): Can these be go-generated and checked in?
	if _, err := exec.RunCwd(ctx, ".", "make", "-C", "perf", "migration_source"); err != nil {
		td.Fatal(ctx, err)
	}

	// Build all non-test binaries.
	var buildFlags []string
	if *buildFlagsList != "" {
		buildFlags = strings.Split(*buildFlagsList, " ")
	}
	buildCmd := []string{"go", "build", "-o", binTmpPath}
	buildCmd = append(buildCmd, buildFlags...)
	buildCmd = append(buildCmd, *targets...)
	if _, err := exec.RunCwd(ctx, ".", buildCmd...); err != nil {
		td.Fatal(ctx, err)
	}
	// Copy the whitelisted regular binaries.
	if err := td.Do(ctx, td.Props("Copy binaries"), func(ctx context.Context) error {
		if *outputWhitelist == nil {
			if err := os_steps.Rename(ctx, binTmpPath, binOutPath); err != nil {
				return err
			}
		} else {
			if err := os_steps.MkdirAll(ctx, binOutPath); err != nil {
				return err
			}
			for _, wl := range *outputWhitelist {
				src := filepath.Join(binTmpPath, wl)
				dst := filepath.Join(binOutPath, wl)
				if err := os_steps.Rename(ctx, src, dst); err != nil {
					return err
				}
			}
		}
		return os_steps.RemoveAll(ctx, binTmpPath)
	}); err != nil {
		td.Fatal(ctx, err)
	}

	// Build the tests. Unfortunately, there is no command which
	// builds all of the tests at once and saves the binaries
	// for future use[1], so we use the -exec flag to "go test",
	// using a script which copies the executable to the output
	// directory. Note that the test executables passed to this
	// script only contain the base name of the package, eg.
	// "types" => "types.test". To prevent conflicts, the script
	// appends a random number to the file name, and we use the
	// JSON output from "go test" to match the resulting files
	// name back to the full package import paths and then
	// rename them accordingly.
	//
	// As an alternative, I tried using "go list" to find the
	// packages with test files and then running "go test -c"
	// for each of those. Doing so sequentially took about 10
	// minutes; running in parallel resulted in timeouts after a
	// much longer time, presumably due to contention of some
	// kind.
	//
	// [1] https://github.com/golang/go/issues/15513
	copyTestPath := filepath.Join(cwd, "go_deps", "copy_test.sh")
	execCmd := fmt.Sprintf("%s %s", copyTestPath, testTmpPath)
	// Note: We're skipping vet here because we want to compile
	// all of the tests and run them regardless of whether vet
	// passes. We'll need to explicitly run it somewhere else.
	testCmd := []string{"go", "test", "-vet=off", "-json", "-exec", execCmd}
	testCmd = append(testCmd, buildFlags...)
	testCmd = append(testCmd, *targets...)
	output, err := exec.RunCwd(ctx, ".", testCmd...)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if err := td.Do(ctx, td.Props("Copy test binaries"), func(ctx context.Context) error {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			ev, err := test2json.ParseEvent(line)
			if err != nil {
				return err
			}
			m := copyTestPathRegex.FindStringSubmatch(ev.Output)
			if len(m) == 2 {
				// Get the host-side path to the test binary.
				testDst := filepath.Join(testOutPath, ev.Package+".test")
				if err := os_steps.MkdirAll(ctx, filepath.Dir(testDst)); err != nil {
					return err
				}
				if err := os_steps.Rename(ctx, m[1], testDst); err != nil {
					return err
				}
			}
		}
		return os_steps.RemoveAll(ctx, testTmpPath)
	}); err != nil {
		td.Fatal(ctx, err)
	}

	// Copy additional tools from $GOPATH/bin.
	if err := td.Do(ctx, td.Props("Copy tools"), func(ctx context.Context) error {
		env, err := golang.GetEnv(ctx)
		if err != nil {
			return err
		}
		platform := fmt.Sprintf("%s-%s", env["GOOS"], env["GOARCH"])
		src := filepath.Join(env["GOPATH"], "bin", platform)
		files, err := os_steps.ReadDir(ctx, src)
		if err != nil {
			return err
		}
		for _, file := range files {
			srcFull := filepath.Join(src, file.Name())
			// TODO(borenet): Copy using Go?
			if _, err := exec.RunCwd(ctx, ".", "cp", srcFull, binOutPath); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
}
