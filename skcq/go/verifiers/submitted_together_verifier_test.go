package verifiers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/skcq/go/verifiers/interfaces/mocks"
)

func TestVerify_NoTogetherChanges(t *testing.T) {
	stv := &SubmittedTogetherVerifier{
		togetherChangesToVerifiers: nil,
	}

	state, _, err := stv.Verify(context.Background(), nil, 0)
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)
}

func TestVerify_NoDepChecksFooter(t *testing.T) {
	stv := &SubmittedTogetherVerifier{
		togetherChangesToVerifiers: []*Verifier{&mocks.Verifier{}, &mocks.Verifier{}},
		footersMap: map[string]string{
			string(footers.NoDependencyChecksFooter): "true",
		},
	}

	state, _, err := stv.Verify(context.Background(), nil, 0)
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)
}
