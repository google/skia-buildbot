package chrome_branch

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	fakeData = `[
  {
    "os": "linux",
    "versions": [
      {
        "channel": "beta",
        "current_version": "81.0.4044.17",
        "true_branch": "4044"
      },
      {
        "channel": "stable",
        "current_version": "80.0.3987.100",
        "true_branch": "3987"
      }
    ]
  }
]`
)

var (
	fakeBranches = &Branches{
		Beta: &Branch{
			Milestone: 81,
			Number:    4044,
		},
		Stable: &Branch{
			Milestone: 80,
			Number:    3987,
		},
	}
)

func TestBranchCopy(t *testing.T) {
	unittest.SmallTest(t)
	assertdeep.Copy(t, fakeBranches.Beta, fakeBranches.Beta.Copy())
}

func TestBranchesCopy(t *testing.T) {
	unittest.SmallTest(t)
	assertdeep.Copy(t, fakeBranches, fakeBranches.Copy())
}

func TestGet(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	urlmock := mockhttpclient.NewURLMock()
	c := urlmock.Client()

	// Everything okay.
	urlmock.MockOnce(jsonUrl, mockhttpclient.MockGetDialogue([]byte(fakeData)))
	b, err := Get(ctx, c)
	require.NoError(t, err)
	assertdeep.Equal(t, fakeBranches, b)

	// OS missing altogether.
	urlmock.MockOnce(jsonUrl, mockhttpclient.MockGetDialogue([]byte("[]")))
	b, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "No branches found for OS"))

	// Beta channel is missing.
	urlmock.MockOnce(jsonUrl, mockhttpclient.MockGetDialogue([]byte(strings.ReplaceAll(fakeData, branchBeta, "dev"))))
	b, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "Beta branch is missing"), err)

	// Missing number.
	urlmock.MockOnce(jsonUrl, mockhttpclient.MockGetDialogue([]byte(strings.ReplaceAll(fakeData, "true_branch", "nope"))))
	b, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "invalid branch number"), err)

	// Missing milestone.
	urlmock.MockOnce(jsonUrl, mockhttpclient.MockGetDialogue([]byte(strings.ReplaceAll(fakeData, "current_version", "nope"))))
	b, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "invalid milestone"), err)
}
