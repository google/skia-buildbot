package chrome_branch

import (
	"context"
	"fmt"
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
        "true_branch": "4044"
      },
      {
        "channel": "stable",
        "true_branch": "3987"
      }
    ]
  }
]`
)

func TestBranchesCopy(t *testing.T) {
	unittest.SmallTest(t)
	v := &Branches{
		Beta:   "beta-branch",
		Stable: "stable-branch",
	}
	assertdeep.Copy(t, v, v.Copy())
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
	require.Equal(t, "4044", b.Beta)
	require.Equal(t, "3987", b.Stable)

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
	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("No %q branch found for OS", branchBeta)))
}

func TestExecute(t *testing.T) {
	unittest.SmallTest(t)

	// Everything okay.
	b := &Branches{
		Beta:   "4044",
		Stable: "3987",
	}
	actual, err := Execute("{{.Beta}}", b)
	require.NoError(t, err)
	require.Equal(t, b.Beta, actual)
	actual, err = Execute("{{.Stable}}", b)
	require.NoError(t, err)
	require.Equal(t, b.Stable, actual)

	// No such branch.
	actual, err = Execute("{{.Bogus}}", b)
	require.Error(t, err)
	require.Equal(t, "", actual)
}

func TestManager(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	urlmock := mockhttpclient.NewURLMock()
	c := urlmock.Client()

	// New Manager.
	urlmock.MockOnce(jsonUrl, mockhttpclient.MockGetDialogue([]byte(fakeData)))
	m, err := New(ctx, c)
	require.NoError(t, err)
	b := m.Get()
	require.NotNil(t, b)
	require.Equal(t, "4044", b.Beta)
	require.Equal(t, "3987", b.Stable)
	assertdeep.Copy(t, m.branches, b)
	actual, err := m.Execute("{{.Beta}}")
	require.NoError(t, err)
	require.Equal(t, b.Beta, actual)

	// Update should succeed without error.
	urlmock.MockOnce(jsonUrl, mockhttpclient.MockGetDialogue([]byte(fakeData)))
	require.NoError(t, m.Update(ctx))

	// Update should pick up modified branches.
	urlmock.MockOnce(jsonUrl, mockhttpclient.MockGetDialogue([]byte(`[
  {
    "os": "linux",
    "versions": [
      {
        "channel": "beta",
        "true_branch": "5000"
      },
      {
        "channel": "stable",
        "true_branch": "4044"
      }
    ]
  }
]`)))
	require.NoError(t, m.Update(ctx))
	assertdeep.Equal(t, &Branches{
		Beta:   "5000",
		Stable: "4044",
	}, m.Get())
}
