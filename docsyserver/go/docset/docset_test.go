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
	"go.skia.org/infra/docsyserver/go/codereview"
	crmocks "go.skia.org/infra/docsyserver/go/codereview/mocks"
	"go.skia.org/infra/docsyserver/go/docsy/mocks"
	gittestutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/now"
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

var mockTime = time.Unix(12, 0).UTC()

var myFakeError = fmt.Errorf("My fake error")

// Returns a context, the working directory, the full path to the source, the
// full path to the destination, a mock for Docsy, a mock for CoreReview, and a
// constructed docSet.
func setupForTest(t *testing.T) (context.Context, string, string, string, *mocks.Docsy, *crmocks.CodeReview, *docSet) {
	ctx := context.WithValue(context.Background(), now.ContextKey, mockTime)

	// Create a test repo to work with.
	gb := gittestutils.GitInit(t, ctx)
	gb.Add(ctx, "site/_index.md", "This is an index file.")
	gb.Commit(ctx)

	workDir := t.TempDir()
	src := filepath.Join(workDir, contentSubDirectory, string(codereview.MainIssue), docPath)
	dst := filepath.Join(workDir, destinationSubDirectory, string(codereview.MainIssue), docPath)
	docsy := &mocks.Docsy{}
	cr := &crmocks.CodeReview{}
	docset := New(workDir, docPath, docsyDir, gb.Dir(), cr, docsy)

	return ctx, workDir, src, dst, docsy, cr, docset
}

func setupForTestWithMainRepoLoaded(t *testing.T) (context.Context, string, string, string, *mocks.Docsy, *crmocks.CodeReview, *docSet) {
	ctx, workDir, src, dst, docsy, cr, docset := setupForTest(t)
	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)

	// Call singleStep so that that main repo at HEAD gets loaded.
	err := docset.singleStep(ctx)
	require.NoError(t, err)
	docsy.AssertExpectations(t)

	return ctx, workDir, src, dst, docsy, cr, docset
}

// Returns a context, the working directory, the full path to the source, the
// full path to the destination, a mock for Docsy, a mock for CoreReview, and a
// constructed docSet that has already loaded and rendered the files for the
// main repo and 'issue'.
//
// The 'src' and 'dst' returned are for the 'issue' and not the main repo.
func setupForTestWithMainRepoAndIssueAlreadyLoaded(t *testing.T) (context.Context, string, string, string, *mocks.Docsy, *crmocks.CodeReview, *docSet) {
	ctx, workDir, src, dst, docsy, cr, docset := setupForTestWithMainRepoLoaded(t)

	// CodeReview should report a new patchset on the issue with a new file, 'users.md'.
	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, nil)
	cr.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef).Return([]codereview.ListModifiedFilesResult{
		{
			Filename: "site/users.md",
			Deleted:  false,
		},
	}, nil)
	cr.On("GetFile", testutils.AnyContext, "site/users.md", patchsetRef).Return([]byte("This is file content."), nil)

	src = filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	dst = filepath.Join(workDir, destinationSubDirectory, string(issue), docPath)
	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)

	_, err := docset.FileSystem(ctx, issue)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(src, "users.md"))
	require.FileExists(t, filepath.Join(src, "_index.md"))
	cr.AssertExpectations(t)
	docsy.AssertExpectations(t)
	return ctx, workDir, src, dst, docsy, cr, docset
}

func TestStart_Success(t *testing.T) {
	unittest.LargeTest(t)
	parentContext, _, src, dst, docsy, _, docset := setupForTest(t)

	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)

	ctx, cancel := context.WithCancel(parentContext)
	defer cancel()

	// Start should load the main repo.
	err := docset.Start(ctx)

	require.NoError(t, err)
	docsy.AssertExpectations(t)
	require.NotNil(t, docset.cache[codereview.MainIssue])
	require.Equal(t, mockTime, docset.cache[codereview.MainIssue].lastPatchsetCheck)
	require.FileExists(t, filepath.Join(src, "_index.md"))

	// The main repo is loaded and is returned from FileSystem.
	fs, err := docset.FileSystem(ctx, codereview.MainIssue)
	require.NoError(t, err)
	require.Equal(t, fs, docset.cache[codereview.MainIssue].fs)
}

func TestStart_RenderFails_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	parentContext, _, src, dst, docsy, _, docset := setupForTest(t)

	docsy.On("Render", testutils.AnyContext, src, dst).Return(myFakeError)

	ctx, cancel := context.WithCancel(parentContext)
	defer cancel()

	err := docset.Start(ctx)

	require.Error(t, err)
	require.Contains(t, err.Error(), myFakeError.Error())
}

func TestStart_BadGitRepoURL_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	workDir := t.TempDir()
	emptyDirectoryIsNotAValidGitRepo := t.TempDir()
	docset := New(workDir, docPath, docsyDir, emptyDirectoryIsNotAValidGitRepo, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := docset.Start(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Failed to clone")
}

func TestFileSystem_NoFilesChanged_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, workDir, _, _, docsy, cr, docset := setupForTestWithMainRepoLoaded(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, nil)
	// ListModifiedFiles returns empty slice.
	cr.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef).Return([]codereview.ListModifiedFilesResult{}, nil)

	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	dst := filepath.Join(workDir, destinationSubDirectory, string(issue), docPath)
	docsy.On("Render", testutils.AnyContext, src, dst).Return(nil)

	_, err := docset.FileSystem(ctx, issue)

	require.NoError(t, err)
	docsy.AssertExpectations(t)
	cr.AssertExpectations(t)
}

func TestFileSystem_GetPatchsetInfoFailsAndNoFileSystemAlreadyExistsInCache_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, _, _, _, _, cr, docset := setupForTestWithMainRepoLoaded(t)

	cr.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, myFakeError)

	_, err := docset.FileSystem(ctx, issue)

	require.Contains(t, err.Error(), myFakeError.Error())
	cr.AssertExpectations(t)
}

func TestFileSystem_FilesAreSymlinks(t *testing.T) {
	unittest.LargeTest(t)
	_, _, src, _, _, _, _ := setupForTestWithMainRepoAndIssueAlreadyLoaded(t)

	// _index.md is not modified in the issue, so it should exist as a symlink
	// in the issues src directory.
	fileinfo, err := os.Lstat(filepath.Join(src, "_index.md"))
	require.NoError(t, err)
	require.True(t, fileinfo.Mode()&os.ModeSymlink == os.ModeSymlink)
}

func TestFileSystem_GetPatchsetInfoFailsAndFileSystemAlreadyExistsInCache_SuccessReturnsExistingFileSystem(t *testing.T) {
	unittest.LargeTest(t)
	ctx, _, _, _, _, _, docset := setupForTestWithMainRepoAndIssueAlreadyLoaded(t)

	cr2 := &crmocks.CodeReview{}
	cr2.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, false, myFakeError)
	docset.codeReview = cr2

	// Make sure we move far enough into the future that docset decides
	// the cache is stale and GetPatchsetInfo needs to be called.
	ctx = context.WithValue(ctx, now.ContextKey, mockTime.Add(2*refreshDuration))

	fs, err := docset.FileSystem(ctx, issue)

	require.NoError(t, err)
	require.Equal(t, docset.cache[issue].fs, fs)
	cr2.AssertExpectations(t)
}

func TestFileSystem_FileOutsideDocPathIsAdded_SuccessAndFileIsNotPresent(t *testing.T) {
	unittest.LargeTest(t)
	ctx, workDir, _, _, docsy, cr, docset := setupForTestWithMainRepoLoaded(t)

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
	unittest.LargeTest(t)
	ctx, _, _, _, _, cr, docset := setupForTestWithMainRepoLoaded(t)

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

func TestFileSystem_FileDeletedInIssue_FileIsRemoved(t *testing.T) {
	unittest.LargeTest(t)
	ctx, workDir, _, _, docsy, cr, docset := setupForTestWithMainRepoLoaded(t)

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

func TestFileSystem_CacheIsExpiredButIssueHasNotChanged_ReturnsExistingFS(t *testing.T) {
	unittest.LargeTest(t)
	ctx, _, _, _, _, cr, docset := setupForTestWithMainRepoAndIssueAlreadyLoaded(t)

	// Make sure we move far enough into the future that docset decides
	// the cache is stale and GetPatchsetInfo needs to be called.
	updatedMockTime := mockTime.Add(2 * refreshDuration)
	ctx = context.WithValue(ctx, now.ContextKey, updatedMockTime)

	cr.On("GetPatchsetInfo", testutils.AnyContext, codereview.MainIssue).Return(patchsetRef, false, nil)

	fs, err := docset.FileSystem(ctx, issue)

	require.NoError(t, err)
	require.Equal(t, docset.cache[issue].fs, fs)
	require.Equal(t, docset.cache[issue].lastPatchsetCheck, updatedMockTime)
}

func TestFileSystem_IssueIsClosed_ReturnsErrorAndDirectoriesGetCleanedUp(t *testing.T) {
	unittest.LargeTest(t)
	ctx, workDir, _, _, _, _, docset := setupForTestWithMainRepoAndIssueAlreadyLoaded(t)

	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	require.Len(t, docset.cache, 2)
	require.FileExists(t, filepath.Join(src, "users.md"))
	require.FileExists(t, filepath.Join(src, "_index.md"))

	// Make sure we move far enough into the future that docset decides
	// the cache is stale and GetPatchsetInfo needs to be called.
	updatedMockTime := mockTime.Add(2 * refreshDuration)
	ctx = context.WithValue(ctx, now.ContextKey, updatedMockTime)

	// Now close the issue.
	cr2 := &crmocks.CodeReview{}
	cr2.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, true, nil)
	docset.codeReview = cr2

	_, err := docset.FileSystem(ctx, issue)
	require.Error(t, err)
	require.Contains(t, err.Error(), IssueClosedErr.Error())

	require.Len(t, docset.cache, 1)
	require.NoFileExists(t, filepath.Join(src, "users.md"))
	require.NoFileExists(t, filepath.Join(src, "_index.md"))
	cr2.AssertExpectations(t)
}

func TestFileSystem_IssueIsUpdated_NewFilesAreUpdated(t *testing.T) {
	unittest.LargeTest(t)
	ctx, workDir, _, _, _, _, docset := setupForTestWithMainRepoAndIssueAlreadyLoaded(t)

	// Make sure we move far enough into the future that docset decides
	// the cache is stale and GetPatchsetInfo needs to be called.
	updatedMockTime := mockTime.Add(2 * refreshDuration)
	ctx = context.WithValue(ctx, now.ContextKey, updatedMockTime)

	// Get an updated patchset that updates users.md.
	cr2 := &crmocks.CodeReview{}
	cr2.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef2, false, nil)
	cr2.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef2).Return([]codereview.ListModifiedFilesResult{
		{
			Filename: "site/users.md",
			Deleted:  false,
		},
	}, nil)
	contents := "This is updated content."
	cr2.On("GetFile", testutils.AnyContext, "site/users.md", patchsetRef2).Return([]byte(contents), nil)
	docset.codeReview = cr2

	_, err := docset.FileSystem(ctx, issue)

	require.NoError(t, err)
	cr2.AssertExpectations(t)
	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	// Note that the old _index.md was removed and re-symlinked successfully.
	require.FileExists(t, filepath.Join(src, "_index.md"))
	// And we have written over an old file.
	b, err := ioutil.ReadFile(filepath.Join(src, "users.md"))
	require.NoError(t, err)
	require.Equal(t, contents, string(b))
}

func TestSingleStep_IssueIsClosed_DirectoriesGetCleanedUp(t *testing.T) {
	unittest.LargeTest(t)
	ctx, workDir, _, _, _, _, docset := setupForTestWithMainRepoAndIssueAlreadyLoaded(t)

	src := filepath.Join(workDir, contentSubDirectory, string(issue), docPath)
	require.Len(t, docset.cache, 2)
	require.FileExists(t, filepath.Join(src, "users.md"))
	require.FileExists(t, filepath.Join(src, "_index.md"))

	// Make sure we move far enough into the future that docset decides
	// the cache is stale and GetPatchsetInfo needs to be called.
	updatedMockTime := mockTime.Add(2 * refreshDuration)
	ctx = context.WithValue(ctx, now.ContextKey, updatedMockTime)

	// Now close the issue.
	cr2 := &crmocks.CodeReview{}
	cr2.On("GetPatchsetInfo", testutils.AnyContext, issue).Return(patchsetRef, true, nil)
	docset.codeReview = cr2

	err := docset.singleStep(ctx)
	require.NoError(t, err)

	require.Len(t, docset.cache, 1)
	require.NoFileExists(t, filepath.Join(src, "users.md"))
	require.NoFileExists(t, filepath.Join(src, "_index.md"))
	cr2.AssertExpectations(t)
}

func TestRefresh_ListModifiedFilesFails_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, _, _, _, _, cr, docset := setupForTestWithMainRepoAndIssueAlreadyLoaded(t)

	// Make sure we move far enough into the future that docset decides
	// the cache is stale and GetPatchsetInfo needs to be called.
	ctx = context.WithValue(ctx, now.ContextKey, mockTime.Add(2*refreshDuration))

	cr.On("ListModifiedFiles", testutils.AnyContext, issue, patchsetRef2).Return(nil, myFakeError)

	_, err := docset.refresh(ctx, issue, patchsetRef2)

	require.Error(t, err)
	require.Contains(t, err.Error(), myFakeError.Error())
	cr.AssertExpectations(t)
}
