// docset keeps track of checkouts of a repository of Markdown documents.
package docset

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/docsy/go/codereview"
	crmocks "go.skia.org/infra/docsy/go/codereview/mocks"
	"go.skia.org/infra/docsy/go/docsy/mocks"
	gittestutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	docPath  = "site/"
	docsyDir = "/docsy"

	issue       codereview.Issue = "123"
	patchsetRef                  = "123/17"
)

var mockTime = time.Unix(0, 12).UTC()

var myFakeError = fmt.Errorf("My fake error")

// Returns a context, the working directory, the full path to the source, the
// full path to the destination, a mock for Docsy, a mock for CoreReview, and a
// constructed docSet.
func setupForTest(t *testing.T) (context.Context, string, string, string, *mocks.Docsy, *crmocks.CodeReview, *docSet) {
	timeNow = func() time.Time {
		return mockTime
	}
	ctx := context.Background()
	gb := gittestutils.GitInit(t, ctx)
	gb.Add(ctx, "site/_index.md", "This is an index file.")
	gb.Commit(ctx)

	workDir, err := ioutil.TempDir("", "docset")
	require.NoError(t, err)
	t.Cleanup(
		func() {
			require.NoError(t, os.RemoveAll(workDir))
		})

	d := &mocks.Docsy{}
	src := filepath.Join(workDir, contentSubDirectory, string(codereview.MainIssue), docPath)
	dst := filepath.Join(workDir, destinationSubDirectory, string(codereview.MainIssue), docPath)
	d.On("Render", testutils.AnyContext, src, dst).Return(nil)
	cr := &crmocks.CodeReview{}
	cr.On("MainIssue").Return(codereview.MainIssue)
	docset, err := New(context.Background(), workDir, docPath, docsyDir, gb.Dir(), cr, d)
	require.NoError(t, err)
	return ctx, workDir, src, dst, d, cr, docset
}

func TestNew_Success(t *testing.T) {
	unittest.SmallTest(t)
	_, _, src, _, _, cr, docset := setupForTest(t)

	require.NotNil(t, docset.cache[cr.MainIssue()])
	require.Equal(t, mockTime, docset.cache[cr.MainIssue()].lastPatchsetCheck)
	require.FileExists(t, filepath.Join(src, "_index.md"))

}

func TestNew_BadGitRepoURL_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	workDir, err := ioutil.TempDir("", "docset")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(workDir))
	}()

	d := &mocks.Docsy{}
	cr := &crmocks.CodeReview{}
	cr.On("MainIssue").Return(codereview.MainIssue)

	_, err = New(context.Background(), workDir, docPath, docsyDir, "/tmp/this-is-not-a-valid-git-repo-path", cr, d)
	require.Contains(t, err.Error(), "Failed to clone")
}

func TestFileSystem_IssueIsClosed_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, _, _, _, _, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, true, nil)
	_, err := docset.FileSystem(ctx, issue)
	require.Error(t, err)
}

func TestFileSystem_NoFilesChanged_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx, workDir, _, _, docsy, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, nil)
	cr.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef).Return(nil, nil)
	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	dst := filepath.Join(workDir, destinationSubDirectory, string(issue), docPath)

	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)
	_, err := docset.FileSystem(ctx, issue)
	require.NoError(t, err)
}

func TestFileSystem_GetPatchsetInfoFailsIfNoFileSystemAlreadyExists_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, _, _, _, _, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, myFakeError)
	_, err := docset.FileSystem(ctx, issue)
	require.Contains(t, err.Error(), myFakeError.Error())
}

func TestFileSystem_GetPatchsetInfoFailsWhenFileSystemAlreadyExists_SuccessReturnsExistingFileSystem(t *testing.T) {
	unittest.SmallTest(t)
	ctx, workDir, _, _, docsy, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, nil)
	cr.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef).Return([]codereview.ListModifiedFilesResult{
		{
			Filename: "site/users.md",
			Deleted:  false,
		},
	}, nil)
	cr.On("GetFile", testutils.AnyContext, "site/users.md", patchsetRef).Return([]byte("This is file content."), nil)
	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	dst := filepath.Join(workDir, destinationSubDirectory, string(issue), docPath)

	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)
	_, err := docset.FileSystem(ctx, issue)
	require.NoError(t, err)

	cr2 := &crmocks.CodeReview{}
	cr2.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, myFakeError)
	cr2.On("MainIssue").Return(codereview.MainIssue)
	docset.codeReview = cr2

	// Make sure we move far enough into the future that docset decides
	// the cache is stale and GetPatchsetInfo needs to be called.
	timeNow = func() time.Time {
		return mockTime.Add(2 * refreshDuration)
	}

	fs, err := docset.FileSystem(ctx, issue)
	require.NoError(t, err)
	require.Equal(t, docset.cache[issue].fs, fs)
}

func TestFileSystem_FileAdded_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx, workDir, _, _, docsy, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, nil)
	cr.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef).Return([]codereview.ListModifiedFilesResult{
		{
			Filename: "site/users.md",
			Deleted:  false,
		},
	}, nil)
	cr.On("GetFile", testutils.AnyContext, "site/users.md", patchsetRef).Return([]byte("This is file content."), nil)
	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	dst := filepath.Join(workDir, destinationSubDirectory, string(issue), docPath)

	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)
	_, err := docset.FileSystem(ctx, issue)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(src, "users.md"))
	require.FileExists(t, filepath.Join(src, "_index.md"))
}

func TestFileSystem_FileDeleted_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx, workDir, _, _, docsy, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, nil)
	cr.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef).Return([]codereview.ListModifiedFilesResult{
		{
			Filename: "site/_index.md",
			Deleted:  true,
		},
	}, nil)
	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	dst := filepath.Join(workDir, destinationSubDirectory, string(issue), docPath)

	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)
	_, err := docset.FileSystem(ctx, issue)
	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(src, "site", "_index.md"))
}
