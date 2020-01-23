package golang

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	skexec "go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/test2json"
	"go.skia.org/infra/task_driver/go/lib/dirs"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// COMMON_DEPS are dependencies needed by the infra tests in this repo.
	COMMON_DEPS = []string{
		"github.com/golang/protobuf/protoc-gen-go",
		"github.com/kisielk/errcheck",
		"golang.org/x/tools/cmd/goimports",
		"golang.org/x/tools/cmd/stringer",
		"github.com/vektra/mockery/...",
	}
)

// WithEnv sets the Go environment to be based in the given workdir. Calls to all
// other functions in this package should use the returned Context, or a
// descendant of it.
func WithEnv(ctx context.Context, workdir string) context.Context {
	goPath := filepath.Join(workdir, "gopath")
	goRoot := filepath.Join(workdir, "go", "go")
	goBin := filepath.Join(goRoot, "bin")

	PATH := strings.Join([]string{
		goBin,
		filepath.Join(goPath, "bin"),
		filepath.Join(workdir, "gcloud_linux", "bin"),
		filepath.Join(workdir, "protoc", "bin"),
		filepath.Join(workdir, "node", "node", "bin"),
		td.PATH_PLACEHOLDER,
	}, string(os.PathListSeparator))
	return td.WithEnv(ctx, []string{
		fmt.Sprintf("GOCACHE=%s", filepath.Join(dirs.Cache(workdir), "go_cache")),
		"GOFLAGS=-mod=readonly", // Prohibit builds from modifying go.mod.
		fmt.Sprintf("GOROOT=%s", goRoot),
		fmt.Sprintf("GOPATH=%s", goPath),
		fmt.Sprintf("PATH=%s", PATH),
	})
}

// Go runs the given Go command in the given working directory.
func Go(ctx context.Context, cwd string, args ...string) (string, error) {
	return skexec.RunCommand(ctx, &skexec.Command{
		Name: "go",
		Args: args,
		Dir:  cwd,
	})
}

// Info executes commands to find the path to the Go executable and its version.
func Info(ctx context.Context) (string, string, error) {
	goExc, err := os_steps.Which(ctx, "go")
	if err != nil {
		return "", "", err
	}
	goVer, err := Go(ctx, ".", "version")
	if err != nil {
		return "", "", err
	}
	return goExc, goVer, nil
}

// ModDownload downloads the Go module dependencies of the module in cwd.
// Tries up to three times in case of transient network issues.
func ModDownload(ctx context.Context, cwd string) error {
	return td.WithRetries(ctx, 3, func(ctx context.Context) error {
		_, err := Go(ctx, cwd, "mod", "download")
		return err
	})
}

// Install runs "go install" with the given args.
func Install(ctx context.Context, cwd string, args ...string) error {
	_, err := Go(ctx, cwd, append([]string{"install"}, args...)...)
	return err
}

// InstallCommonDeps installs common dependencies needed by the infra tests in
// this repo. Tries up to three times per dependency in case of transient
// network issues.
func InstallCommonDeps(ctx context.Context, workdir string) error {
	for _, target := range COMMON_DEPS {
		if err := td.WithRetries(ctx, 3, func(ctx context.Context) error {
			return Install(ctx, workdir, "-v", target)
		}); err != nil {
			return err
		}
	}
	return nil
}

// Test runs "go test", parses the output, and creates sub-steps for individual
// tests.
func Test(ctx context.Context, cwd string, args ...string) error {
	ctx = td.StartStep(ctx, td.Props(fmt.Sprintf("go test --json %s", strings.Join(args, " "))))
	defer td.EndStep(ctx)

	// Set up the "go test" command.
	cmd := exec.CommandContext(ctx, "go", append([]string{"test", "--json"}, args...)...)
	cmd.Dir = cwd
	cmd.Env = td.GetEnv(ctx)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return td.FailStep(ctx, err)
	}
	runErrors := td.NewLogStream(ctx, "golang", td.Error)
	scanner := bufio.NewScanner(stdout)

	// Spin up a goroutine which parses the JSON output of "go test" and
	// creates sub-steps.
	var wg sync.WaitGroup
	wg.Add(1)

	// runErr records any errors that occur within the goroutine.
	var runErr error

	go func() {
		defer wg.Done()

		// We create sub-steps for individual packages, which in turn
		// have sub-steps for individual tests.
		pkgContexts := map[string]context.Context{}
		testContexts := map[string]map[string]context.Context{}
		logs := map[context.Context]io.Writer{}
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(os.Stderr, line)
			var event test2json.Event
			if err := json.NewDecoder(bytes.NewReader([]byte(line))).Decode(&event); err != nil {
				runErr = err
				if _, logErr := runErrors.Write([]byte(err.Error())); logErr != nil {
					sklog.Errorf("Failed to write log: %s", logErr)
				}
				continue
			}
			pkgCtx, ok := pkgContexts[event.Package]
			if !ok {
				// This is the first time we've seen this package;
				// create a sub-step for it.
				pkgCtx = td.StartStep(ctx, td.Props(event.Package))
				pkgContexts[event.Package] = pkgCtx
				testContexts[event.Package] = map[string]context.Context{}
			}
			stepCtx := pkgCtx
			if event.Test != "" {
				testCtx, ok := testContexts[event.Package][event.Test]
				if !ok {
					// This is the first time we've seen this test;
					// create a sub-step for it.
					testCtx = td.StartStep(pkgCtx, td.Props(event.Test))
					testContexts[event.Package][event.Test] = testCtx
				}
				stepCtx = testCtx
			}
			switch event.Action {
			case test2json.ACTION_OUTPUT:
				stream, ok := logs[stepCtx]
				if !ok {
					stream = td.NewLogStream(stepCtx, "go-test", td.Error)
				}
				_, err := stream.Write([]byte(event.Output))
				if err != nil {
					runErr = err
					if _, logErr := runErrors.Write([]byte(err.Error())); logErr != nil {
						sklog.Errorf("Failed to write log: %s", logErr)
					}
					continue
				}
			// These indicate that the step has finished.
			case test2json.ACTION_FAIL:
				td.FailStep(stepCtx, errors.New("step failed")) // TODO
				fallthrough
			case test2json.ACTION_SKIP:
				fallthrough
			case test2json.ACTION_PASS:
				fallthrough
			default:
				td.EndStep(stepCtx)

			}
		}
	}()

	// Run the command.
	if err := cmd.Run(); err != nil {
		// Wait for log processing goroutine to finish.
		wg.Wait()
		return td.FailStep(ctx, err)
	}

	// Wait for log processing goroutine to finish.
	wg.Wait()
	if runErr != nil {
		return td.FailStep(ctx, runErr)
	}
	return nil
}
