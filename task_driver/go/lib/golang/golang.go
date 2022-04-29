package golang

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	skexec "go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/test2json"
	"go.skia.org/infra/task_driver/go/lib/dirs"
	"go.skia.org/infra/task_driver/go/lib/log_parser"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// commonDeps are dependencies needed by the infra tests in this repo.
	commonDeps = []string{
		"github.com/golang/protobuf/protoc-gen-go",
		"github.com/kisielk/errcheck",
		"github.com/twitchtv/twirp/protoc-gen-twirp",
		"github.com/skia-dev/protoc-gen-twirp_typescript",
		"golang.org/x/tools/cmd/goimports",
		"golang.org/x/tools/cmd/stringer",
	}
)

// WithEnv sets the Go environment to be based in the given workdir. Calls to all
// other functions in this package should use the returned Context, or a
// descendant of it.
func WithEnv(ctx context.Context, workdir string) context.Context {
	goPath := filepath.Join(workdir, "gopath")
	goRoot := computeGoRoot(workdir)
	goBin := filepath.Join(goRoot, "bin")

	PATH := strings.Join([]string{
		goBin,
		filepath.Join(goPath, "bin"),
		filepath.Join(workdir, "gcloud_linux", "bin"),
		filepath.Join(workdir, "protoc", "bin"),
		filepath.Join(workdir, "node", "node", "bin"),
		td.PathPlaceholder,
	}, string(os.PathListSeparator))
	return td.WithEnv(ctx, []string{
		"CGO_ENABLED=0",
		fmt.Sprintf("GOCACHE=%s", filepath.Join(dirs.Cache(workdir), "go_cache")),
		"GOFLAGS=-mod=readonly", // Prohibit builds from modifying go.mod.
		fmt.Sprintf("GOROOT=%s", goRoot),
		fmt.Sprintf("GOPATH=%s", goPath),
		fmt.Sprintf("PATH=%s", PATH),
	})
}

// computeGoRoot returns the path to the Go SDK without the symbolic links created by CIPD.
//
// Starting with Go 1.18, the standard library includes "//go:embed" directives that point to other
// files in the standard library. For security reasons, the "embed" package does not support
// symbolic links (discussion at https://github.com/golang/go/issues/35950#issuecomment-561725322),
// and it produces "cannot embed irregular file" errors when it encounters one.
//
// In our CI environment, Go is provided via a CIPD package. Files in CIPD packages are surfaced to
// Swarming tasks via symbolic links inside a directory within the Swarming work directory. If we
// were to point the GOROOT environment variable to said directory, we would get the error above.
// To prevent this, we compute the real path to the Go SDK, and set GOROOT to said path.
//
// An alternative approach is to copy the entire Go SDK to an different location without symbolic
// links, as done in rules_go: https://github.com/bazelbuild/rules_go/pull/3083. However, this
// approach is slower and more complex.
func computeGoRoot(workdir string) string {
	symlinkGoRoot := filepath.Join(workdir, "go", "go")
	symlinkVersionFile := filepath.Join(symlinkGoRoot, "VERSION") // Arbitrary symlink.
	symlinkFreeVersionFile, err := filepath.EvalSymlinks(symlinkVersionFile)
	if err != nil {
		// If the symbolic link could not be resolved, fall back to the non-resolved GOROOT. This
		// should never happen in production, but we exercise this code path from tests that do not
		// mock a directory structure inside the work directory.
		return symlinkGoRoot
	}
	return filepath.Dir(symlinkFreeVersionFile)
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
	for _, target := range commonDeps {
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
	return log_parser.Run(ctx, cwd, append([]string{"go", "test", "--json"}, args...), bufio.ScanLines, func(sm *log_parser.StepManager, line string) error {
		// Decode an event.
		event, err := test2json.ParseEvent(line)
		if err != nil {
			return err
		}

		// Find or create the step associated with this event.
		step := sm.FindStep(event.Package)
		if step == nil {
			step = sm.StartStep(td.Props(event.Package))
		}
		if event.Test != "" {
			// Slash-separated test names indicate nested tests;
			// create a step hierarchy to match. The current step
			// is the last in the chain.
			for _, stepName := range strings.Split(event.Test, "/") {
				test := step.FindChild(stepName)
				if test == nil {
					// This is the first time we've seen
					// this test; create a sub-step for it.
					test = step.StartChild(td.Props(stepName))
				}
				step = test
			}
		}

		// Record any output.
		if event.Output != "" {
			step.Stdout(event.Output)
		}

		// Handle the event action.
		switch event.Action {
		// The below actions mark the end of the step.
		case test2json.ActionFail:
			step.Fail()
			fallthrough
		case test2json.ActionSkip:
			fallthrough
		case test2json.ActionPass:
			step.End()

		// Catch-all for un-handled actions.
		default:
		}

		return nil
	})
}
