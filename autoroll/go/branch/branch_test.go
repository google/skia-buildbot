package branch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestStatic(t *testing.T) {
	unittest.SmallTest(t)

	ref := "refs/heads/my-branch"
	b := NewStaticBranch(ref)
	require.Equal(t, ref, b.Ref())
	require.NoError(t, b.Update(context.Background()))
	require.Equal(t, ref, b.Ref())
}

func TestChrome(t *testing.T) {
	unittest.SmallTest(t)

}
