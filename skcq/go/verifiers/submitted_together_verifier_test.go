package verifiers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/skcq/go/types/mocks"
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

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	stv := &SubmittedTogetherVerifier{
		togetherChangesToVerifiers: map[*gerrit.ChangeInfo][]types.Verifier{
			ci: {&mocks.Verifier{}, &mocks.Verifier{}},
		},
		footersMap: map[string]string{
			string(footers.NoDependencyChecksFooter): "true",
		},
	}

	state, _, err := stv.Verify(context.Background(), ci, 0)
	require.Nil(t, err)
	require.Equal(t, types.VerifierFailureState, state)
}

func TestVerify_AllStates(t *testing.T) {

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	togetherVerifiers := []types.Verifier{&mocks.Verifier{}, &mocks.Verifier{}}

	tests := []struct {
		state types.VerifierState
	}{
		{state: types.VerifierSuccessState},
		{state: types.VerifierWaitingState},
		{state: types.VerifierFailureState},
	}

	for _, test := range tests {
		startTime := int64(123)
		verifierStatuses := []*types.VerifierStatus{
			{State: types.VerifierSuccessState, Name: "Verifier1", Reason: "Reason1"},
			{State: test.state, Name: "Verifier2", Reason: "Reason2"},
		}
		// Setup mock for verifier manager.
		vm := &mocks.VerifiersManager{}
		vm.On("RunVerifiers", testutils.AnyContext, ci, togetherVerifiers, startTime).Return(verifierStatuses).Once()

		stv := &SubmittedTogetherVerifier{
			vm: vm,
			togetherChangesToVerifiers: map[*gerrit.ChangeInfo][]types.Verifier{
				ci: togetherVerifiers,
			},
		}
		state, _, err := stv.Verify(context.Background(), ci, startTime)
		require.Nil(t, err)
		require.Equal(t, test.state, state)
	}
}

func TestCleanup(t *testing.T) {

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	cleanupPatchsetID := int64(5)

	// Setup mock for verifiers.
	v1 := &mocks.Verifier{}
	v1.On("Cleanup", testutils.AnyContext, ci, cleanupPatchsetID).Return().Once()
	v2 := &mocks.Verifier{}
	v2.On("Cleanup", testutils.AnyContext, ci, cleanupPatchsetID).Return().Once()

	stv := &SubmittedTogetherVerifier{
		togetherChangesToVerifiers: map[*gerrit.ChangeInfo][]types.Verifier{
			ci: {v1, v2},
		},
	}
	stv.Cleanup(context.Background(), ci, cleanupPatchsetID)
}
