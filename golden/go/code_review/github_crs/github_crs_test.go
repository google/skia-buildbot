package github_crs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/code_review"
)

func TestGetChangeListSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	m := mockhttpclient.NewURLMock()
	resp := mockhttpclient.MockGetDialogue([]byte(landedPullRequestResponse))
	m.Mock("https://api.github.com/repos/unit/test/pulls/44388", resp)
	//c := New(httputils.DefaultClientConfig().With2xxOnly().Client(), "flutter/flutter")
	c := New(m.Client(), "unit/test")

	id := "44388"
	ts := time.Date(2019, time.November, 7, 23, 39, 17, 0, time.UTC)

	cl, err := c.GetChangeList(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, code_review.ChangeList{
		SystemID: id,
		Owner:    "engine-flutter-autoroll",
		Status:   code_review.Landed,
		Subject:  "Roll engine ddceed5f7af1..629930e8887c (1 commits)",
		Updated:  ts,
	}, cl)
}

// There's a lot more data here, but this is only what we care about.
// See https://developer.github.com/v3/pulls/#get-a-single-pull-request
// for all the fields.
// This is based on https://github.com/flutter/flutter/pull/44388
const landedPullRequestResponse = `
{
	"title": "Roll engine ddceed5f7af1..629930e8887c (1 commits)",
	"state": "closed",
	"user": {
		"login": "engine-flutter-autoroll"
	},
	"updated_at": "2019-11-07T23:39:17Z",
	"merged_at": "2019-11-07T23:39:17Z"
}
`
