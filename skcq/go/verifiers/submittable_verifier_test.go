package verifiers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/skcq/go/types"
)

func TestVerify_Submittable(t *testing.T) {
	unittest.SmallTest(t)

	ci := &gerrit.ChangeInfo{
		Issue:       int64(123),
		Submittable: true,
	}
	submittable := SubmittableVerifier{}
	state, _, err := submittable.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)
}

func TestVerify_NotSubmittable(t *testing.T) {
	unittest.SmallTest(t)

	ci := &gerrit.ChangeInfo{
		Issue:       int64(123),
		Submittable: false,
	}
	submittable := SubmittableVerifier{}
	state, _, err := submittable.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierFailureState, state)
}
