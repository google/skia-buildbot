package verifiers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	allowed_mocks "go.skia.org/infra/go/allowed/mocks"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/types"
)

func setupDryRunVerifierTest(t *testing.T, match bool) (*DryRunAccessListVerifier, *gerrit.ChangeInfo) {
	matchedUser := "batman@gotham.com"
	unmatchedUser := "superman@krypton.com"

	votedUser := unmatchedUser
	if match {
		votedUser = matchedUser
	}
	ci := &gerrit.ChangeInfo{
		Issue: int64(123),
		Labels: map[string]*gerrit.LabelEntry{
			gerrit.LabelCommitQueue: {
				All: []*gerrit.LabelDetail{
					{
						Value: gerrit.LabelCommitQueueDryRun,
						Email: votedUser,
					},
				},
			},
		},
	}
	allowListName := "test-allowlist"

	// Mock allow.
	allow := &allowed_mocks.Allow{}
	allow.On("Member", matchedUser).Return(true).Once()
	allow.On("Member", unmatchedUser).Return(false).Once()

	return &DryRunAccessListVerifier{
		criaGroupName: allowListName,
		dryRunAllowed: allow,
	}, ci
}

func TestVerify_DryRunAccessMatch(t *testing.T) {

	dryRunVerifier, ci := setupDryRunVerifierTest(t, true)
	state, _, err := dryRunVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)
}

func TestVerify_DryRunAccessDoNotMatch(t *testing.T) {

	dryRunVerifier, ci := setupDryRunVerifierTest(t, false)
	state, _, err := dryRunVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierFailureState, state)
}
