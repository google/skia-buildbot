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

func TestGetChangeListSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(landedPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44380", resp)
	c := New(m.Client(), "unit/test")

	id := "44380"
	ts := time.Date(2019, time.November, 7, 23, 39, 17, 0, time.UTC)

	cl, err := c.GetChangeList(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, code_review.ChangeList{
		SystemID: id,
		Owner:    "engine-flutter-autoroll",
		Status:   code_review.Landed,
		Subject:  "Roll engine ddceed5f7af1..629930e8887c (1 commits) (#44380)",
		Updated:  ts,
	}, cl)
}

func TestGetChangeListOpen(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(openPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44380", resp)
	c := New(m.Client(), "unit/test")

	id := "44380"
	ts := time.Date(2019, time.November, 7, 23, 39, 17, 0, time.UTC)

	cl, err := c.GetChangeList(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, code_review.ChangeList{
		SystemID: id,
		Owner:    "engine-flutter-autoroll",
		Status:   code_review.Open,
		Subject:  "Roll engine ddceed5f7af1..629930e8887c (1 commits)",
		Updated:  ts,
	}, cl)
}

func TestGetChangeListAbandoned(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(abandonedPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44345", resp)
	c := New(m.Client(), "unit/test")

	id := "44345"
	ts := time.Date(2019, time.November, 7, 14, 14, 0, 0, time.UTC)

	cl, err := c.GetChangeList(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, code_review.ChangeList{
		SystemID: id,
		Owner:    "a-user",
		Status:   code_review.Abandoned,
		Subject:  "Make BorderRadius.circular() a const constructor",
		Updated:  ts,
	}, cl)
}

func TestGetChangeListDoesNotExist(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	// By not mocking anything, an error will be returned from GitHub,
	// as we would expect for a 404
	c := New(m.Client(), "unit/test")

	_, err := c.GetChangeList(context.Background(), "44345")
	require.Error(t, err)
	assert.Equal(t, code_review.ErrNotFound, err)
}

func TestGetChangeListInvalidID(t *testing.T) {
	unittest.SmallTest(t)

	c := New(nil, "unit/test")

	_, err := c.GetChangeList(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestGetPatchSetsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(fiveCommitsOnPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44419/commits", resp)
	c := New(m.Client(), "unit/test")

	id := "44419"

	xps, err := c.GetPatchSets(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, []code_review.PatchSet{
		{
			SystemID:     "a892f9f299e91924405cb8bd244efc1a6c28e4fa",
			ChangeListID: id,
			Order:        1,
			GitHash:      "a892f9f299e91924405cb8bd244efc1a6c28e4fa",
		},
		{
			SystemID:     "042f0382b7ec0efdb7570c3e6c891cf2a20379a7",
			ChangeListID: id,
			Order:        2,
			GitHash:      "042f0382b7ec0efdb7570c3e6c891cf2a20379a7",
		},
		{
			SystemID:     "a332b7085723c13fa96777a1830c7113e7ffba96",
			ChangeListID: id,
			Order:        3,
			GitHash:      "a332b7085723c13fa96777a1830c7113e7ffba96",
		},
		{
			SystemID:     "d3e3d639d8a1cca0929829b04d90f35011b50fbf",
			ChangeListID: id,
			Order:        4,
			GitHash:      "d3e3d639d8a1cca0929829b04d90f35011b50fbf",
		},
		{
			SystemID:     "74a239b99ee360397a22cede6d9d16aacd245af1",
			ChangeListID: id,
			Order:        5,
			GitHash:      "74a239b99ee360397a22cede6d9d16aacd245af1",
		},
	}, xps)
}

func TestGetPatchSetsDoesNotExist(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	// By not mocking anything, an error will be returned from Git,
	// as we would expect for a 404
	c := New(m.Client(), "unit/test")

	_, err := c.GetPatchSets(context.Background(), "44345")
	require.Error(t, err)
	require.Equal(t, code_review.ErrNotFound, err)
}

func TestGetChangeListForCommitSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(landedPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44380", resp)
	c := New(m.Client(), "unit/test")

	cl, err := c.GetChangeListForCommit(context.Background(), &vcsinfo.LongCommit{
		// This is the only field the implementation cares about.
		ShortCommit: &vcsinfo.ShortCommit{
			Subject: "Roll engine ddceed5f7af1..629930e8887c (1 commits) (#44380)",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, code_review.ChangeList{
		SystemID: "44380",
		Owner:    "engine-flutter-autoroll",
		Status:   code_review.Landed,
		Subject:  "Roll engine ddceed5f7af1..629930e8887c (1 commits) (#44380)",
		Updated:  time.Date(2019, time.November, 7, 23, 39, 17, 0, time.UTC),
	}, cl)
}

func TestGetPatchSetsInvalidID(t *testing.T) {
	unittest.SmallTest(t)

	c := New(nil, "unit/test")

	_, err := c.GetPatchSets(context.Background(), "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestGetChangeListForCommitMalformed(t *testing.T) {
	unittest.SmallTest(t)

	c := New(nil, "unit/test")

	_, err := c.GetChangeListForCommit(context.Background(), &vcsinfo.LongCommit{
		// This is the only field the implementation cares about.
		ShortCommit: &vcsinfo.ShortCommit{
			Subject: "Roll engine ddceed5f7af1..629930e8887c (1 commits)",
		},
	})
	require.Error(t, err)
	assert.Equal(t, code_review.ErrNotFound, err)
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

// This is based on https://github.com/flutter/flutter/pull/44419
const fiveCommitsOnPullRequestResponse = `
[
  {
    "sha": "a892f9f299e91924405cb8bd244efc1a6c28e4fa"
  },
  {
    "sha": "042f0382b7ec0efdb7570c3e6c891cf2a20379a7"
  },
  {
    "sha": "a332b7085723c13fa96777a1830c7113e7ffba96"
  },
  {
    "sha": "d3e3d639d8a1cca0929829b04d90f35011b50fbf"
  },
  {
    "sha": "74a239b99ee360397a22cede6d9d16aacd245af1"
  }
]`
