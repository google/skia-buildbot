package golang

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/dirs"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	goEnv []string // Filled in by Init().
)

// Init initializes the package by setting the Go environment to be based in the
// given workdir. It should be called before using anything else in this
// package. Returns the environment variables which should be used when running
// Go commands.
func Init(workdir string) []string {
	goPath := path.Join(workdir, "gopath")
	goRoot := path.Join(workdir, "go", "go")
	goBin := path.Join(goRoot, "bin")

	PATH := strings.Join([]string{
		goBin,
		path.Join(goPath, "bin"),
		path.Join(workdir, "gcloud_linux", "bin"),
		path.Join(workdir, "protoc", "bin"),
		path.Join(workdir, "node", "node", "bin"),
		td.PATH_PLACEHOLDER,
	}, string(os.PathListSeparator))
	goEnv = []string{
		fmt.Sprintf("GOCACHE=%s", path.Join(dirs.Cache(workdir), "go_cache")),
		"GOFLAGS=-mod=readonly", // Prohibit builds from modifying go.mod.
		fmt.Sprintf("GOROOT=%s", goRoot),
		fmt.Sprintf("GOPATH=%s", goPath),
		fmt.Sprintf("PATH=%s", PATH),
	}
	return util.CopyStringSlice(goEnv)
}

// Go runs the given Go command in the given working directory.
func Go(ctx context.Context, cwd string, args ...string) (string, error) {
	if goEnv == nil {
		return "", errors.New("You must call Init() before any other functions in golang package.")
	}
	return exec.RunCommand(ctx, &exec.Command{
		Name: "go",
		Args: args,
		Env:  goEnv,
		Dir:  cwd,
	})
}

// Info executes commands to find the path to the Go executable and its version.
func Info(ctx context.Context) (string, string, error) {
	goExc, err := exec.RunCwd(ctx, ".", "which", "go")
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
func ModDownload(ctx context.Context, cwd string) error {
	_, err := Go(ctx, cwd, "mod", "download")
	return err
}

// Install runs "go install" with the given args.
func Install(ctx context.Context, cwd string, args ...string) error {
	_, err := Go(ctx, cwd, append([]string{"install"}, args...)...)
	return err
}
