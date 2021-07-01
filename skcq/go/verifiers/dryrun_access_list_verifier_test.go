package verifiers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
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

	mockClient := mockhttpclient.NewURLMock()
	mockClient.Mock(fmt.Sprintf(allowed.GROUP_URL_TEMPLATE, allowListName), mockhttpclient.MockGetDialogue([]byte(fmt.Sprintf(`{"group": {"members": ["user:%s"]}}`, matchedUser))))
	cria, err := allowed.NewAllowedFromChromeInfraAuth(mockClient.Client(), allowListName)
	require.Nil(t, err)
	return &DryRunAccessListVerifier{
		criaGroupName: allowListName,
		dryRunAllowed: cria,
	}, ci
}

func TestVerify_DryRunAccessMatch(t *testing.T) {
	unittest.SmallTest(t)

	dryRunVerifier, ci := setupDryRunVerifierTest(t, true)
	state, _, err := dryRunVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)
}

func TestVerify_DryRunAccessDoNotMatch(t *testing.T) {
	unittest.SmallTest(t)

	dryRunVerifier, ci := setupDryRunVerifierTest(t, false)
	state, _, err := dryRunVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierFailureState, state)
}
