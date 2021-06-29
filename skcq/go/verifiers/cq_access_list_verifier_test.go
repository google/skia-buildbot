package verifiers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/skcq/go/types"
)

func setupCQAccessVerifierTest(t *testing.T, match bool) (*CQAccessListVerifier, *gerrit.ChangeInfo) {
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
						Value: gerrit.LabelCommitQueueSubmit,
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
	return &CQAccessListVerifier{
		criaGroupName: allowListName,
		cqAllowed:     cria,
	}, ci
}

func TestVerify_CQAccessMatch(t *testing.T) {
	cqAccessVerifier, ci := setupCQAccessVerifierTest(t, true)
	state, _, err := cqAccessVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)
}

func TestVerify_CQAccessDoNotMatch(t *testing.T) {
	cqAccessVerifier, ci := setupCQAccessVerifierTest(t, false)
	state, _, err := cqAccessVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierFailureState, state)
}
