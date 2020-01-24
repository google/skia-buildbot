package golang

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	skexec "go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/ring"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/test2json"
	"go.skia.org/infra/task_driver/go/lib/dirs"
	"go.skia.org/infra/task_driver/go/lib/log_parser"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

const (
	// Maximum number of output lines stored in memory to pass along as
	// error messages for failed tests.
	OUTPUT_LINES = 20
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
	// We create sub-steps for individual packages, which in turn
	// have sub-steps for individual tests. We need to store the
	// contexts, keyed by package and test name.
	pkgContexts := map[string]context.Context{}
	testContexts := map[string]map[string]context.Context{}

	// All steps may have associated log output. Key by context for
	// convenience. Additionally, maintain the last N lines of
	// output for all steps so that we can include a snippet in the
	// error message.
	logs := map[context.Context]io.Writer{}
	outputLines := map[context.Context]*ring.StringRing{}

	return log_parser.Run(ctx, cwd, append([]string{"go", "test", "--json"}, args...), bufio.ScanLines, func(ctx context.Context, line string) error {
		// Decode an event.
		event, err := test2json.ParseEvent(line)
		if err != nil {
			return err
		}

		// Find or create the step associated with this event.
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

		// Record any output.
		if event.Output != "" {
			stream, ok := logs[stepCtx]
			if !ok {
				stream = td.NewLogStream(stepCtx, "stdout", td.Info)
				logs[stepCtx] = stream
			}
			_, err := stream.Write([]byte(event.Output))
			if err != nil {
				return skerr.Wrap(err)
			}
			lines, ok := outputLines[stepCtx]
			if !ok {
				lines, err = ring.NewStringRing(OUTPUT_LINES)
				if err != nil {
					return skerr.Wrap(err)
				}
				outputLines[stepCtx] = lines
			}
			lines.Put(event.Output)
		}

		// Handle the event action.
		switch event.Action {
		// The below actions mark the end of the step.
		case test2json.ACTION_FAIL:
			msg := "Step failed with no output"
			output := outputLines[stepCtx]
			if output != nil {
				msg = strings.Join(output.GetAll(), "\n")
			}
			_ = td.FailStep(stepCtx, errors.New(msg))
			fallthrough
		case test2json.ACTION_SKIP:
			fallthrough
		case test2json.ACTION_PASS:
			td.EndStep(stepCtx)

		// Catch-all for un-handled actions.
		default:
		}

		return nil
	}, nil)
}
