package git

import (
	"context"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/git/git_common"
)

var runfilePath = ""

// Find returns the path to the Git binary found in the corresponding CIPD package.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding CIPD package, which Bazel will download as needed.
func Find() (string, error) {
	return bazel.FindExecutable("git", runfilePath)
}

func UseGitFinder(ctx context.Context) context.Context {
	return git_common.WithGitFinder(ctx, Find)
}
