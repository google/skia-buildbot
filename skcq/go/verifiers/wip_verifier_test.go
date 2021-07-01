package verifiers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/skcq/go/types"
)

func TestVerify_WIP(t *testing.T) {
	unittest.SmallTest(t)

	ci := &gerrit.ChangeInfo{
		Issue:          int64(123),
		WorkInProgress: true,
	}
	wip := WIPVerifier{}
	state, _, err := wip.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierFailureState, state)
}

func TestVerify_NotWIP(t *testing.T) {
	unittest.SmallTest(t)

	ci := &gerrit.ChangeInfo{
		Issue:          int64(123),
		WorkInProgress: false,
	}
	wip := WIPVerifier{}
	state, _, err := wip.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)
}
