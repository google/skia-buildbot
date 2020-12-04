package github_crs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/code_review"
)

func TestGetChangelistSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(landedPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44380", resp)
	c := New(m.Client(), "unit/test")

	id := "44380"
	ts := time.Date(2019, time.November, 7, 23, 39, 17, 0, time.UTC)

	cl, err := c.GetChangelist(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, code_review.Changelist{
		SystemID: id,
		Owner:    "engine-flutter-autoroll",
		Status:   code_review.Landed,
		Subject:  "Roll engine ddceed5f7af1..629930e8887c (1 commits) (#44380)",
		Updated:  ts,
	}, cl)
}

func TestGetChangelistOpen(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(openPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44380", resp)
	c := New(m.Client(), "unit/test")

	id := "44380"
	ts := time.Date(2019, time.November, 7, 23, 39, 17, 0, time.UTC)

	cl, err := c.GetChangelist(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, code_review.Changelist{
		SystemID: id,
		Owner:    "engine-flutter-autoroll",
		Status:   code_review.Open,
		Subject:  "Roll engine ddceed5f7af1..629930e8887c (1 commits)",
		Updated:  ts,
	}, cl)
}

func TestGetChangelistAbandoned(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(abandonedPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44345", resp)
	c := New(m.Client(), "unit/test")

	id := "44345"
	ts := time.Date(2019, time.November, 7, 14, 14, 0, 0, time.UTC)

	cl, err := c.GetChangelist(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, code_review.Changelist{
		SystemID: id,
		Owner:    "a-user",
		Status:   code_review.Abandoned,
		Subject:  "Make BorderRadius.circular() a const constructor",
		Updated:  ts,
	}, cl)
}

func TestGetChangelistDoesNotExist(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	// By not mocking anything, an error will be returned from GitHub,
	// as we would expect for a 404
	c := New(m.Client(), "unit/test")

	_, err := c.GetChangelist(context.Background(), "44345")
	require.Error(t, err)
	assert.Equal(t, code_review.ErrNotFound, err)
}

func TestGetChangelistInvalidID(t *testing.T) {
	unittest.SmallTest(t)

	c := New(nil, "unit/test")

	_, err := c.GetChangelist(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

const omitPS = ""
const omitOrder = 0

func TestGetPatchset_OnePageResults_PatchsetExists_Success(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	fiveCommits := mockhttpclient.MockGetDialogue([]byte(fiveCommitsOnPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44419/commits?page=1", fiveCommits)
	donePaging := mockhttpclient.MockGetDialogue([]byte("[]"))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44419/commits?page=2", donePaging)
	c := New(m.Client(), "unit/test")

	const clID = "44419"
	const psOneID = "111119f299e91924405cb8bd244efc1a6c28e4fa"
	const psFiveID = "555559b99ee360397a22cede6d9d16aacd245af1"

	expectedFirstPS := code_review.Patchset{
		SystemID:     psOneID,
		ChangelistID: clID,
		Order:        1,
		GitHash:      psOneID,
	}
	expectedFifthPS := code_review.Patchset{
		SystemID:     psFiveID,
		ChangelistID: clID,
		Order:        5,
		GitHash:      psFiveID,
	}

	ps, err := c.GetPatchset(context.Background(), clID, psOneID, omitOrder)
	require.NoError(t, err)
	assert.Equal(t, expectedFirstPS, ps)
	ps, err = c.GetPatchset(context.Background(), clID, omitPS, 1)
	require.NoError(t, err)
	assert.Equal(t, expectedFirstPS, ps)

	ps, err = c.GetPatchset(context.Background(), clID, psFiveID, omitOrder)
	require.NoError(t, err)
	assert.Equal(t, expectedFifthPS, ps)
	ps, err = c.GetPatchset(context.Background(), clID, omitPS, 5)
	require.NoError(t, err)
	assert.Equal(t, expectedFifthPS, ps)
}

func TestGetPatchset_TwoPageResults_PatchsetsExist_Success(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	fiveCommits := mockhttpclient.MockGetDialogue([]byte(fiveCommitsOnPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44419/commits?page=1", fiveCommits)
	twoCommits := mockhttpclient.MockGetDialogue([]byte(twoCommitsOnPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44419/commits?page=2", twoCommits)
	donePaging := mockhttpclient.MockGetDialogue([]byte("[]"))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44419/commits?page=3", donePaging)
	c := New(m.Client(), "unit/test")

	const clID = "44419"
	const psFiveID = "555559b99ee360397a22cede6d9d16aacd245af1"  // first page
	const psSevenID = "77777b5b0a55743c586708a94cbb69feb7bf32cd" // second page

	expectedFifthPS := code_review.Patchset{
		SystemID:     psFiveID,
		ChangelistID: clID,
		Order:        5,
		GitHash:      psFiveID,
	}
	expectedSeventhPS := code_review.Patchset{
		SystemID:     psSevenID,
		ChangelistID: clID,
		Order:        7,
		GitHash:      psSevenID,
	}

	ps, err := c.GetPatchset(context.Background(), clID, psSevenID, omitOrder)
	require.NoError(t, err)
	assert.Equal(t, expectedSeventhPS, ps)
	ps, err = c.GetPatchset(context.Background(), clID, omitPS, 7)
	require.NoError(t, err)
	assert.Equal(t, expectedSeventhPS, ps)

	ps, err = c.GetPatchset(context.Background(), clID, psFiveID, omitOrder)
	require.NoError(t, err)
	assert.Equal(t, expectedFifthPS, ps)
	ps, err = c.GetPatchset(context.Background(), clID, omitPS, 5)
	require.NoError(t, err)
	assert.Equal(t, expectedFifthPS, ps)
}

func TestGetPatchset_OnePageResults_PatchsetDoesNotExist_ReturnsNotFound(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	fiveCommits := mockhttpclient.MockGetDialogue([]byte(fiveCommitsOnPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44419/commits?page=1", fiveCommits)
	donePaging := mockhttpclient.MockGetDialogue([]byte("[]"))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44419/commits?page=2", donePaging)
	c := New(m.Client(), "unit/test")

	const clID = "44419"
	_, err := c.GetPatchset(context.Background(), clID, "does not exist", omitOrder)
	require.Error(t, err)
	assert.Equal(t, code_review.ErrNotFound, err)

	_, err = c.GetPatchset(context.Background(), clID, omitPS, 10000)
	require.Error(t, err)
	assert.Equal(t, code_review.ErrNotFound, err)

}

func TestGetPatchset_NoPatchsetsReturned_ReturnsNotFound(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()

	none := mockhttpclient.MockGetDialogue([]byte("[]"))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44419/commits?page=1", none)
	c := New(m.Client(), "unit/test")

	_, err := c.GetPatchset(context.Background(), "44419", "whatever", omitOrder)
	require.Error(t, err)
	assert.Equal(t, code_review.ErrNotFound, err)

	_, err = c.GetPatchset(context.Background(), "44419", omitPS, 4)
	require.Error(t, err)
	assert.Equal(t, code_review.ErrNotFound, err)

}

func TestGetPatchset_ChangelistDoesNotExist_ReturnsNotFound(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	// By not mocking anything, an error will be returned from Git,
	// as we would expect for a 404
	c := New(m.Client(), "unit/test")

	_, err := c.GetPatchset(context.Background(), "1234", "nope", omitOrder)
	require.Error(t, err)
	require.Equal(t, code_review.ErrNotFound, err)
	_, err = c.GetPatchset(context.Background(), "1234", omitPS, 1)
	require.Error(t, err)
	require.Equal(t, code_review.ErrNotFound, err)
}

func TestGetPatchset_InvalidIDForChangelist_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	c := New(nil, "unit/test")

	_, err := c.GetPatchset(context.Background(), "bad", "nope", omitOrder)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestGetChangelistForCommitSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(landedPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44380", resp)
	c := New(m.Client(), "unit/test")

	clID, err := c.GetChangelistIDForCommit(context.Background(), &vcsinfo.LongCommit{
		// This is the only field the implementation cares about.
		ShortCommit: &vcsinfo.ShortCommit{
			Subject: "Roll engine ddceed5f7af1..629930e8887c (1 commits) (#44380)",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "44380", clID)
}

func TestGetChangelistForCommitMalformed(t *testing.T) {
	unittest.SmallTest(t)

	c := New(nil, "unit/test")

	_, err := c.GetChangelistIDForCommit(context.Background(), &vcsinfo.LongCommit{
		// This is the only field the implementation cares about.
		ShortCommit: &vcsinfo.ShortCommit{
			Subject: "Roll engine ddceed5f7af1..629930e8887c (1 commits)",
		},
	})
	require.Error(t, err)
	assert.Equal(t, code_review.ErrNotFound, err)
}

func TestCommentOnSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	expectedJSON := []byte(`{"body":"untriaged \"digests\" detected"}`)
	responseJSONWeIgnoreAnyway := []byte(`{"id": 1}`)
	resp := mockhttpclient.MockPostDialogueWithResponseCode("application/json", expectedJSON, responseJSONWeIgnoreAnyway, 201)
	m.Mock("https://api.github.com/repos/unit/test/issues/44474/comments", resp)
	c := New(m.Client(), "unit/test")

	err := c.CommentOn(context.Background(), "44474", `untriaged "digests" detected`)
	require.NoError(t, err)
}

func TestCommentOnError(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	// By not mocking anything, an error will be returned from GitHub,
	// as we would expect for a Changelist not found or something.
	c := New(m.Client(), "unit/test")

	_, err := c.GetChangelist(context.Background(), "44345")
	require.Error(t, err)
}

// There's a lot more data here, but these JSON strings contain
// only the fields which we care about.
// This is based on https://github.com/flutter/flutter/pull/44380
const landedPullRequestResponse = `
{
	"title": "Roll engine ddceed5f7af1..629930e8887c (1 commits) (#44380)",
	"state": "closed",
	"user": {
		"login": "engine-flutter-autoroll"
	},
	"updated_at": "2019-11-07T23:39:17Z",
	"merged_at": "2019-11-07T23:39:17Z"
}`

const openPullRequestResponse = `
{
	"title": "Roll engine ddceed5f7af1..629930e8887c (1 commits)",
	"state": "open",
	"user": {
		"login": "engine-flutter-autoroll"
	},
	"updated_at": "2019-11-07T23:39:17Z",
	"merged_at": null
}`

// This is based on https://github.com/flutter/flutter/pull/44345
const abandonedPullRequestResponse = `
{
	"title": "Make BorderRadius.circular() a const constructor",
	"state": "closed",
	"user": {
		"login": "a-user"
	},
	"updated_at": "2019-11-07T14:14:00Z",
	"merged_at": null
}`

// This is based on https://github.com/flutter/flutter/pull/44419, with the shas changed for
// readability.
const fiveCommitsOnPullRequestResponse = `
[
  {
    "sha": "111119f299e91924405cb8bd244efc1a6c28e4fa"
  },
  {
    "sha": "22222382b7ec0efdb7570c3e6c891cf2a20379a7"
  },
  {
    "sha": "333337085723c13fa96777a1830c7113e7ffba96"
  },
  {
    "sha": "44444639d8a1cca0929829b04d90f35011b50fbf"
  },
  {
    "sha": "555559b99ee360397a22cede6d9d16aacd245af1"
  }
]`

const twoCommitsOnPullRequestResponse = `
[
  {
    "sha": "666667ad358596996cfd8664b66a9087e3d7ee1c"
  },
  {
    "sha": "77777b5b0a55743c586708a94cbb69feb7bf32cd"
  }
]`
