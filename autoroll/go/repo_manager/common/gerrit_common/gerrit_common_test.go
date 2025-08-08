package gerrit_common

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	gerrit_testutils "go.skia.org/infra/go/gerrit/testutils"
	"go.skia.org/infra/go/mockhttpclient"
)

func TestGetNotSubmittedReason_IsSubmitted(t *testing.T) {
	test := func(name string, rev *revision.Revision, results ...*gerrit.ChangeInfo) {
		t.Run(name, func(t *testing.T) {
			mockGerrit := gerrit_testutils.NewGerrit(t)
			if len(results) > 0 {
				mockGerrit.MockSearch(results, 0, gerrit.SearchCommit(rev.Id))
			}
			actual, err := GetNotSubmittedReason(t.Context(), rev, mockGerrit.Mock.Client())
			require.NoError(t, err)
			require.Equal(t, "", actual)
			mockGerrit.AssertEmpty()
		})
	}
	test("Merged", &revision.Revision{
		Id: "fake-commit",
		Details: `some commit message

Change-Id: 56789
`,
		URL: strings.Replace(gerrit_testutils.FakeGerritURL, "-review", "", 1),
	}, &gerrit.ChangeInfo{
		Issue:    56789,
		ChangeId: "56789",
		Status:   gerrit.ChangeStatusMerged,
	})

	test("Ambiguous", &revision.Revision{
		Id: "fake-commit",
		Details: `some commit message

Change-Id: 56789
`,
		URL: strings.Replace(gerrit_testutils.FakeGerritURL, "-review", "", 1),
	}, &gerrit.ChangeInfo{
		Issue:    23238732,
		ChangeId: "wrong one",
		Status:   gerrit.ChangeStatusMerged,
	}, &gerrit.ChangeInfo{
		Issue:    56789,
		ChangeId: "56789",
		Status:   gerrit.ChangeStatusMerged,
	})

}

func TestGetNotSubmittedReason_NotSubmitted(t *testing.T) {
	test := func(name, expect string, rev *revision.Revision, results ...*gerrit.ChangeInfo) {
		t.Run(name, func(t *testing.T) {
			mockGerrit := gerrit_testutils.NewGerrit(t)
			if len(results) > 0 {
				mockGerrit.MockSearch(results, 0, gerrit.SearchCommit(rev.Id))
			}
			actual, err := GetNotSubmittedReason(t.Context(), rev, mockGerrit.Mock.Client())
			require.NoError(t, err)
			require.Equal(t, expect, actual)
			mockGerrit.AssertEmpty()
		})
	}
	test("Not a Gerrit change", "Revision is not a Gerrit change; cannot verify that it has been reviewed and submitted", &revision.Revision{
		Id:      "fake-commit",
		Details: "some commit message with no footers",
	})

	test("Not merged", fmt.Sprintf("CL %s/c/12345 is not merged", gerrit_testutils.FakeGerritURL), &revision.Revision{
		Id: "fake-commit",
		Details: `some commit message

Change-Id: 12345
`,
		URL: strings.Replace(gerrit_testutils.FakeGerritURL, "-review", "", 1),
	}, &gerrit.ChangeInfo{
		Issue:    12345,
		ChangeId: "12345",
	})
}

func TestGetNotSubmittedReason_NotInSearchButFound_Success(t *testing.T) {
	mockGerrit := gerrit_testutils.NewGerrit(t)
	rev := &revision.Revision{
		Id: "fake-commit",
		Details: `some commit message

Change-Id: 56789
`,
		URL: strings.Replace(gerrit_testutils.FakeGerritURL, "-review", "", 1),
	}
	results := []*gerrit.ChangeInfo{
		{
			Issue:    23238732,
			ChangeId: "wrong one",
			Status:   gerrit.ChangeStatusMerged,
		},
		{
			Issue:    3782378,
			ChangeId: "also wrong",
			Status:   gerrit.ChangeStatusMerged,
		},
	}
	mockGerrit.MockSearch(results, 0, gerrit.SearchCommit(rev.Id))
	mockGerrit.MockGetIssueProperties(&gerrit.ChangeInfo{
		Issue:    56789,
		ChangeId: "56789",
		Status:   gerrit.ChangeStatusMerged,
	})

	actual, err := GetNotSubmittedReason(context.Background(), rev, mockGerrit.Mock.Client())
	require.NoError(t, err)
	require.Equal(t, "", actual)
	mockGerrit.AssertEmpty()
}

func TestGetNotSubmittedReason_NotInSearchNotFound_Returns404(t *testing.T) {
	mockGerrit := gerrit_testutils.NewGerrit(t)
	rev := &revision.Revision{
		Id: "fake-commit",
		Details: `some commit message

Change-Id: 56789
`,
		URL: strings.Replace(gerrit_testutils.FakeGerritURL, "-review", "", 1),
	}
	results := []*gerrit.ChangeInfo{
		{
			Issue:    23238732,
			ChangeId: "wrong one",
			Status:   gerrit.ChangeStatusMerged,
		},
		{
			Issue:    3782378,
			ChangeId: "also wrong",
			Status:   gerrit.ChangeStatusMerged,
		},
	}
	mockGerrit.MockSearch(results, 0, gerrit.SearchCommit(rev.Id))
	url := gerrit_testutils.FakeGerritURL + "/a" + fmt.Sprintf(gerrit.URLTmplChange, "56789")
	mockGerrit.Mock.MockOnce(url, mockhttpclient.MockGetError("404 change not found", 404))

	actual, err := GetNotSubmittedReason(context.Background(), rev, mockGerrit.Mock.Client())
	require.NoError(t, err)
	require.Equal(t, "failed to retrieve Gerrit CL for change ID \"56789\"", actual)
	mockGerrit.AssertEmpty()
}

func TestCommitURLToGerritURL(t *testing.T) {
	test := func(inp, expect string) {
		actual, err := commitURLToGerritURL(inp)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
	}
	test("https://skia.googlesource.com/skia.git/+show/16fe2a7fa5c404b7dac7bc83e28cb8fd9a1cc42e", "https://skia-review.googlesource.com")
}
