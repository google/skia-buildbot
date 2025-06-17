package chromiumbuilder

// Code related to dependency injection for the package. These types exist
// solely to support dependency injection for testing in other chromiumbuilder
// code.

import (
	"context"
	"os"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
)

type checkoutFactory = func(context.Context, string, string) (git.Checkout, error)

func realCheckoutFactory(ctx context.Context, repoUrl, workdir string) (git.Checkout, error) {
	return git.NewCheckout(ctx, repoUrl, workdir)
}

// vfs does not have the concept of directory creation, so we need to have a
// separate way of handling dependency injection for that. In tests, the two
// will likely be backed by the same mock filesystem.
type directoryCreator = func(string, os.FileMode) error

func realDirectoryCreator(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Similarly, vfs does not have the concept of directory removal.
type directoryRemover = func(string) error

func realDirectoryRemover(path string) error {
	return os.RemoveAll(path)
}

// exec.RunIndefinitely() cannot be used as-is for testing like other exec.Run*
// functions since it does not take a context, and it relies directly on
// os/exec.Command behavior (for waiting). It would likely be possible to add
// support for this by abstracting away the os/exec dependency, but for now,
// just use this type for dependency injection.
type concurrentCommandRunner = func(*exec.Command) (exec.Process, <-chan error, error)

func realConcurrentCommandRunner(command *exec.Command) (exec.Process, <-chan error, error) {
	return exec.RunIndefinitely(command)
}

type environmentGetter = func(string) string

func realEnvironmentGetter(key string) string {
	return os.Getenv(key)
}
