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
        "current_version": "81.0.4044.17",
        "true_branch": "4044"
      },
      {
        "channel": "stable",
        "current_version": "80.0.3987.100",
        "true_branch": "3987_137"
      }
    ]
  }
]`
)

func dummyBranches() *Branches {
	return &Branches{
		Main: &Branch{
			Milestone: 82,
			Number:    0,
			Ref:       RefMain,
		},
		Beta: &Branch{
			Milestone: 81,
			Number:    4044,
			Ref:       fmt.Sprintf(refTmplRelease, 4044),
		},
		Stable: &Branch{
			Milestone: 80,
			Number:    3987,
			Ref:       fmt.Sprintf(refTmplRelease, 3987),
		},
	}
}

func TestBranchCopy(t *testing.T) {
	unittest.SmallTest(t)

	b := dummyBranches()
	assertdeep.Copy(t, b.Beta, b.Beta.Copy())
}

func TestBranchesCopy(t *testing.T) {
	unittest.SmallTest(t)

	b := dummyBranches()
	assertdeep.Copy(t, b, b.Copy())
}

func TestBranchValidate(t *testing.T) {
	unittest.SmallTest(t)

	test := func(fn func(*Branch), expectErr string) {
		b := dummyBranches().Beta
		fn(b)
		err := b.Validate()
		if expectErr == "" {
			require.NoError(t, err)
		} else {
			require.NotNil(t, err)
			require.True(t, strings.Contains(err.Error(), expectErr))
		}
	}

	// OK.
	test(func(b *Branch) {}, "")
	test(func(b *Branch) {
		b.Ref = RefMain
		b.Number = 0
	}, "")

	// Not OK.
	test(func(b *Branch) {
		b.Milestone = 0
	}, "Milestone is required")
	test(func(b *Branch) {
		b.Number = 0
	}, "Number is required")
	test(func(b *Branch) {
		b.Ref = RefMain
	}, "Number must be zero for main branch")
}

func TestBranchesValidate(t *testing.T) {
	unittest.SmallTest(t)

	test := func(fn func(*Branches), expectErr string) {
		b := dummyBranches()
		fn(b)
		err := b.Validate()
		if expectErr == "" {
			require.NoError(t, err)
		} else {
			require.NotNil(t, err)
			require.True(t, strings.Contains(err.Error(), expectErr), err)
		}
	}

	// OK.
	test(func(b *Branches) {}, "")

	// Missing branch.
	test(func(b *Branches) {
		b.Beta = nil
	}, "Beta branch is missing")
	test(func(b *Branches) {
		b.Stable = nil
	}, "Stable branch is missing")
	test(func(b *Branches) {
		b.Main = nil
	}, "Main branch is missing")

	// Each Branch should be validated.
	test(func(b *Branches) {
		b.Beta.Milestone = 0
	}, "Milestone is required")
	test(func(b *Branches) {
		b.Stable.Number = 0
	}, "Number is required")
	test(func(b *Branches) {
		b.Main.Number = 42
	}, "Number must be zero for main branch.")
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
	assertdeep.Equal(t, dummyBranches(), b)

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
