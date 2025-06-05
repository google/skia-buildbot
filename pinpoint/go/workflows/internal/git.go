// Activities to access Chromium git repositories.

package internal

import (
	"context"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/common"
	pb "go.skia.org/infra/pinpoint/proto/v1"

	"golang.org/x/oauth2/google"
)

// ReadGitFileActivity is an Activity that reads the contents of a file from a git commit.
func ReadGitFileActivity(ctx context.Context, combinedCommit *common.CombinedCommit, path string) ([]byte, error) {
	sklog.Info("ReadGitFileActivity started")
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "problem setting up default token source")
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).Client()

	var commit *pb.Commit
	if len(combinedCommit.ModifiedDeps) > 0 {
		commit = combinedCommit.ModifiedDeps[len(combinedCommit.ModifiedDeps)-1]
	} else {
		commit = combinedCommit.Main
	}

	repo := gitiles.NewRepo(commit.Repository, httpClient)
	return repo.ReadFileAtRef(ctx, path, commit.GitHash)
}
