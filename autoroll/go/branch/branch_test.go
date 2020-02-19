package branch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestStatic(t *testing.T) {
	unittest.SmallTest(t)

	ref := "refs/heads/my-branch"
	b := NewStaticBranch(ref)
	require.Equal(t, ref, b.String())
	require.NoError(t, b.Update(context.Background()))
	require.Equal(t, ref, b.String())
}

func TestChrome(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	m := &mocks.Manager{}
	b := &ChromeBranch{
		m:    m,
		tmpl: "refs/branch-heads/{{.Beta}}",
	}
	m.On("Update", ctx).Return(nil)
	m.On("Execute", b.tmpl).Return("refs/branch-heads/4044", nil)
	require.NoError(t, b.Update(ctx))
	require.Equal(t, "refs/branch-heads/4044", b.String())

	// NOTE: We'd like to test the below, but mockery doesn't allow us to
	// change the mocked return value once set...
	//m.On("Execute", b.tmpl).Return("refs/branch-heads/5000", nil)
	//require.NoError(t, b.Update(ctx))
	//require.Equal(t, "refs/branch-heads/5000", b.String())
}
