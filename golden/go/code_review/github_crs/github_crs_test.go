package github_crs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "go.skia.org/infra/go/httputils"
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
	require.Equal(t, code_review.ChangeList{
		SystemID: id,
		Owner:    "engine-flutter-autoroll",
		Status:   code_review.Landed,
		Subject:  "Roll engine ddceed5f7af1..629930e8887c (1 commits) (#44380)",
		Updated:  ts,
	}, cl)
}

func TestGetPatchSetsSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(fiveCommitsOnPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44419/commits", resp)
	//c := New(httputils.DefaultClientConfig().With2xxOnly().Client(), "flutter/flutter")
	c := New(m.Client(), "unit/test")

	id := "44419"

	xps, err := c.GetPatchSets(context.Background(), id)
	require.NoError(t, err)
	require.Len(t, xps, 5)
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
	require.Equal(t, code_review.ChangeList{
		SystemID: "44380",
		Owner:    "engine-flutter-autoroll",
		Status:   code_review.Landed,
		Subject:  "Roll engine ddceed5f7af1..629930e8887c (1 commits) (#44380)",
		Updated:  time.Date(2019, time.November, 7, 23, 39, 17, 0, time.UTC),
	}, cl)
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
