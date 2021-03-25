// Package gerrit implements CodeReview for Gerrit.
package gerrit

import (
	"context"
	"path"
	"strconv"

	"go.skia.org/infra/docsy/go/codereview"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
)

// gerritCodeView implements CodeReview.
type gerritCodeReview struct {
	// Gerrit used to interact with the Gerrit system.
	gc gerrit.GerritInterface

	// gitiles is used to download file contents.
	gitiles *gitiles.Repo
}

// New returns a new instance of gerritCodeReview.
//
// The gerritURL value would probably be gerrit.GerritSkiaURL.
func New(local bool, gerritURL, gitilesURL string) (*gerritCodeReview, error) {
	ts, err := auth.NewDefaultTokenSource(local, auth.SCOPE_GERRIT)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	gc, err := gerrit.NewGerrit(gerritURL, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &gerritCodeReview{
		gc:      gc,
		gitiles: gitiles.NewRepo(gitilesURL, client),
	}, nil
}

// MainIssue implements CodeReview.
func (cr *gerritCodeReview) MainIssue() codereview.Issue {
	return "-1"
}

// ListModifiedFiles implements CodeReview.
func (cr *gerritCodeReview) ListModifiedFiles(ctx context.Context, issue codereview.Issue, ref string) ([]codereview.ListModifiedFilesResult, error) {
	// Convert Ref to patch.
	patch := path.Base(ref)
	issueInt64, err := strconv.ParseInt(string(issue), 10, 64)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	ret := []codereview.ListModifiedFilesResult{}

	files, err := cr.gc.Files(ctx, issueInt64, patch)
	for filename, fileinfo := range files {
		if filename == "/COMMIT_MSG" {
			continue
		}
		ret = append(ret, codereview.ListModifiedFilesResult{
			Filename: filename,
			Deleted:  fileinfo.Status == "D",
		})
	}
	return ret, nil
}

// GetFile implements CodeReview.
func (cr *gerritCodeReview) GetFile(ctx context.Context, filename, ref string) ([]byte, error) {
	return cr.gitiles.ReadFileAtRef(ctx, filename, ref)
}

// GetPatchsetInfo implements CodeReview.
func (cr *gerritCodeReview) GetPatchsetInfo(ctx context.Context, issue codereview.Issue) (string, bool, error) {
	changeInfo, err := cr.gc.GetChange(ctx, string(issue))
	if err != nil {
		return "", false, skerr.Wrap(err)
	}
	return changeInfo.Patchsets[len(changeInfo.Patchsets)-1].Ref, changeInfo.IsClosed(), nil
}

// Assert that gerritCodeReview implements the CodeReview interface.
var _ codereview.CodeReview = (*gerritCodeReview)(nil)
