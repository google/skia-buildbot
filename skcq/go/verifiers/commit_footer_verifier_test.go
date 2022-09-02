package verifiers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
)

func TestVerify_NoFooter(t *testing.T) {

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	commitFooterVerifier := &CommitFooterVerifier{}
	state, _, err := commitFooterVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)
}

func TestVerify_CommitFalseFooter(t *testing.T) {

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	commitFooterVerifier := &CommitFooterVerifier{
		footersMap: map[string]string{
			string(footers.CommitFooter): "false",
		},
	}
	state, _, err := commitFooterVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierFailureState, state)
}

func TestVerify_CommitTrueFooter(t *testing.T) {

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	commitFooterVerifier := &CommitFooterVerifier{
		footersMap: map[string]string{
			string(footers.CommitFooter): "true",
		},
	}
	state, _, err := commitFooterVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)
}
