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
	docPath      = "site/"
	docsyDir     = "/docsy"
	issue        = codereview.Issue("123")
	patchsetRef  = "123/17"
	patchsetRef2 = "123/18"
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

	docsy := &mocks.Docsy{}
	src := filepath.Join(workDir, contentSubDirectory, string(codereview.MainIssue), docPath)
	dst := filepath.Join(workDir, destinationSubDirectory, string(codereview.MainIssue), docPath)
	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)
	cr := &crmocks.CodeReview{}
	docset, err := New(context.Background(), workDir, docPath, docsyDir, gb.Dir(), cr, docsy)
	require.NoError(t, err)
	cr.AssertExpectations(t)
	docsy.AssertExpectations(t)
	return ctx, workDir, src, dst, docsy, cr, docset
}

// Returns a context, the working directory, the full path to the source, the
// full path to the destination, a mock for Docsy, a mock for CoreReview, and a
// constructed docSet that has already loaded and rendered the files for
// 'issue'.
func setupForTestWithIssueAlreadyLoaded(t *testing.T) (context.Context, string, string, string, *mocks.Docsy, *crmocks.CodeReview, *docSet) {
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
	cr.AssertExpectations(t)
	docsy.AssertExpectations(t)
	return ctx, workDir, src, dst, docsy, cr, docset
}

func TestNew_Success(t *testing.T) {
	unittest.SmallTest(t)
	_, _, src, _, _, _, docset := setupForTest(t)

	require.NotNil(t, docset.cache[codereview.MainIssue])
	require.Equal(t, mockTime, docset.cache[codereview.MainIssue].lastPatchsetCheck)
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

	_, err = New(context.Background(), workDir, docPath, docsyDir, "/tmp/this-is-not-a-valid-git-repo-path", cr, d)
	require.Contains(t, err.Error(), "Failed to clone")
}

func TestFileSystem_IssueIsClosed_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, _, _, _, _, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, true, nil)
	_, err := docset.FileSystem(ctx, issue)
	require.Error(t, err)
	cr.AssertExpectations(t)
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
	cr.AssertExpectations(t)
}

func TestFileSystem_ListModifiedFilesFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, workDir, _, _, docsy, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, nil)
	cr.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef).Return(nil, myFakeError)
	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	dst := filepath.Join(workDir, destinationSubDirectory, string(issue), docPath)

	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)
	_, err := docset.FileSystem(ctx, issue)
	require.Contains(t, err.Error(), myFakeError.Error())
	cr.AssertExpectations(t)
}

func TestFileSystem_GetPatchsetInfoFailsAndNoFileSystemAlreadyExists_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, _, _, _, _, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, myFakeError)
	_, err := docset.FileSystem(ctx, issue)
	require.Contains(t, err.Error(), myFakeError.Error())
	cr.AssertExpectations(t)
}

func TestFileSystem_GetPatchsetInfoFailsAndFileSystemAlreadyExists_SuccessReturnsExistingFileSystem(t *testing.T) {
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
	cr.AssertExpectations(t)
	docsy.AssertExpectations(t)

	cr2 := &crmocks.CodeReview{}
	cr2.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, myFakeError)
	docset.codeReview = cr2

	// Make sure we move far enough into the future that docset decides
	// the cache is stale and GetPatchsetInfo needs to be called.
	timeNow = func() time.Time {
		return mockTime.Add(2 * refreshDuration)
	}

	fs, err := docset.FileSystem(ctx, issue)
	require.NoError(t, err)
	require.Equal(t, docset.cache[issue].fs, fs)
	cr2.AssertExpectations(t)
}

func TestFileSystem_FileAdded_Success(t *testing.T) {
	unittest.SmallTest(t)
	_, workDir, _, _, _, _, _ := setupForTestWithIssueAlreadyLoaded(t)

	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	require.FileExists(t, filepath.Join(src, "users.md"))
	require.FileExists(t, filepath.Join(src, "_index.md"))
}

func TestFileSystem_FileOutSideDocPathIsAdded_SuccessAndFileIsNotPresent(t *testing.T) {
	unittest.SmallTest(t)
	ctx, workDir, _, _, docsy, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, nil)
	cr.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef).Return([]codereview.ListModifiedFilesResult{
		{
			Filename: "not-the-site-directory/users.md",
			Deleted:  false,
		},
	}, nil)
	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	dst := filepath.Join(workDir, destinationSubDirectory, string(issue), docPath)

	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)
	_, err := docset.FileSystem(ctx, issue)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(src, "_index.md"))
	require.NoFileExists(t, filepath.Join(workDir, contentSubDirectory, string(issue), "not-the-site-directory", "users.md"))
	cr.AssertExpectations(t)
	docsy.AssertExpectations(t)
}

func TestFileSystem_FileAddedButGetFileFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, _, _, _, _, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, nil)
	cr.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef).Return([]codereview.ListModifiedFilesResult{
		{
			Filename: "site/users.md",
			Deleted:  false,
		},
	}, nil)
	cr.On("GetFile", testutils.AnyContext, "site/users.md", patchsetRef).Return(nil, myFakeError)
	_, err := docset.FileSystem(ctx, issue)
	require.Contains(t, err.Error(), myFakeError.Error())
	cr.AssertExpectations(t)
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
	cr.AssertExpectations(t)
	docsy.AssertExpectations(t)
}

func TestFileSystem_IssueHasNotChange_ReturnsExistingFS(t *testing.T) {
	unittest.SmallTest(t)
	ctx, _, _, _, _, cr, docset := setupForTest(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, codereview.MainIssue).Return(patchsetRef, false, nil)

	fs, err := docset.FileSystem(ctx, codereview.MainIssue)
	require.NoError(t, err)
	require.Equal(t, docset.cache[codereview.MainIssue].fs, fs)
}

func TestFileSystem_CacheIsExpiredButIssueHasNotChange_ReturnsExistingFS(t *testing.T) {
	unittest.SmallTest(t)
	ctx, _, _, _, _, cr, docset := setupForTest(t)

	// Make sure we move far enough into the future that docset decides
	// the cache is stale and GetPatchsetInfo needs to be called.
	timeNow = func() time.Time {
		return mockTime.Add(2 * refreshDuration)
	}

	cr.On("GetPatchsetInfo", testutils.AnyContext, codereview.MainIssue).Return(patchsetRef, false, nil)

	fs, err := docset.FileSystem(ctx, codereview.MainIssue)
	require.NoError(t, err)
	require.Equal(t, docset.cache[codereview.MainIssue].fs, fs)
}

func TestFileSystem_IssueIsClosed_DirectoriesGetCleanedUp(t *testing.T) {
	unittest.SmallTest(t)
	ctx, workDir, _, _, _, _, docset := setupForTestWithIssueAlreadyLoaded(t)

	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	require.Len(t, docset.cache, 2)
	require.FileExists(t, filepath.Join(src, "users.md"))
	require.FileExists(t, filepath.Join(src, "_index.md"))

	// Now close the issue.
	cr2 := &crmocks.CodeReview{}
	cr2.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, true, nil)
	docset.codeReview = cr2
	docset.singleStep(ctx)

	require.Len(t, docset.cache, 1)
	require.NoFileExists(t, filepath.Join(src, "users.md"))
	require.NoFileExists(t, filepath.Join(src, "_index.md"))
	cr2.AssertExpectations(t)
}
