package gerrit

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/docserver/go/codereview"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

var myFakeError = fmt.Errorf("My fake error")

const issue codereview.Issue = "123"
const issueInt64 int64 = 123

func TestGetPatchsetInfo_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	gc := &mocks.GerritInterface{}
	changeInfo := &gerrit.ChangeInfo{
		Status: gerrit.ChangeStatusNew,
		Patchsets: []*gerrit.Revision{
			{
				Ref: "refs/changes/96/386796/22",
			},
		},
	}
	gc.On("GetChange", testutils.AnyContext, string(issue)).Return(changeInfo, nil)

	cr := gerritCodeReview{
		gc: gc,
	}
	ref, isClosed, err := cr.GetPatchsetInfo(context.Background(), issue)
	require.NoError(t, err)
	require.False(t, isClosed)
	require.Equal(t, ref, changeInfo.Patchsets[0].Ref)
}

func TestGetPatchsetInfo_GetChangeFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	gc := &mocks.GerritInterface{}
	gc.On("GetChange", testutils.AnyContext, string(issue)).Return(nil, myFakeError)

	cr := gerritCodeReview{
		gc: gc,
	}
	_, _, err := cr.GetPatchsetInfo(context.Background(), issue)
	require.Contains(t, err.Error(), myFakeError.Error())
}

func TestListModifiedFiles_FilesReturnsError_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	gc := &mocks.GerritInterface{}
	gc.On("Files", testutils.AnyContext, issueInt64, "23").Return(nil, myFakeError)

	cr := gerritCodeReview{
		gc: gc,
	}
	_, err := cr.ListModifiedFiles(context.Background(), issue, "refs/changes/96/386796/23")
	require.Contains(t, err.Error(), myFakeError.Error())
}

func TestListModifiedFiles_MalformedIssue_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	gc := &mocks.GerritInterface{}
	cr := gerritCodeReview{
		gc: gc,
	}
	_, err := cr.ListModifiedFiles(context.Background(), "not a valid issue number", "refs/changes/96/386796/23")
	require.Contains(t, err.Error(), "invalid syntax")
}

func TestListModifiedFiles_FilesReturnsEmptySlice_ReturnsEmptySlice(t *testing.T) {
	unittest.SmallTest(t)

	gc := &mocks.GerritInterface{}
	response := map[string]*gerrit.FileInfo{}
	gc.On("Files", testutils.AnyContext, issueInt64, "23").Return(response, nil)

	cr := gerritCodeReview{
		gc: gc,
	}
	files, err := cr.ListModifiedFiles(context.Background(), issue, "refs/changes/96/386796/23")
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestListModifiedFiles_FilesReturnsWithCommitMessage_ReturnsEmptySlice(t *testing.T) {
	unittest.SmallTest(t)

	gc := &mocks.GerritInterface{}
	response := map[string]*gerrit.FileInfo{
		commitMessageFileName: {},
	}
	gc.On("Files", testutils.AnyContext, issueInt64, "23").Return(response, nil)

	cr := gerritCodeReview{
		gc: gc,
	}
	files, err := cr.ListModifiedFiles(context.Background(), issue, "refs/changes/96/386796/23")
	require.NoError(t, err)
	require.Empty(t, files)
}

type ListModifiedFilesResultSlice []codereview.ListModifiedFilesResult

func (p ListModifiedFilesResultSlice) Len() int { return len(p) }
func (p ListModifiedFilesResultSlice) Less(i, j int) bool {
	return strings.Compare(p[i].Filename, p[j].Filename) < 0
}
func (p ListModifiedFilesResultSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func TestListModifiedFiles_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	gc := &mocks.GerritInterface{}
	response := map[string]*gerrit.FileInfo{
		"site/_index.md": {
			Status: "D",
		},
		"site/users.md": {
			Status: "M",
		},
		"site/dev.md": {},
	}
	gc.On("Files", testutils.AnyContext, issueInt64, "23").Return(response, nil)

	cr := gerritCodeReview{
		gc: gc,
	}
	files, err := cr.ListModifiedFiles(context.Background(), issue, "refs/changes/96/386796/23")
	require.NoError(t, err)
	expected := []codereview.ListModifiedFilesResult{
		{
			Filename: "site/_index.md",
			Deleted:  true,
		},
		{
			Filename: "site/users.md",
			Deleted:  false,
		},
		{
			Filename: "site/dev.md",
			Deleted:  false,
		},
	}
	sort.Sort(ListModifiedFilesResultSlice(expected))
	sort.Sort(ListModifiedFilesResultSlice(files))
	require.Equal(t, expected, files)
}
