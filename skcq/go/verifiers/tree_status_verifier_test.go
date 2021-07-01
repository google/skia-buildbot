package verifiers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
	tree_status_types "go.skia.org/infra/tree_status/go/types"
)

func setupTreeStatusVerifier(t *testing.T, treeMsg, treeState string) *TreeStatusVerifier {
	treeStatusURL := "http://tree-status-url/status"

	//Mock httpclient
	serialized, err := json.Marshal(&TreeStatus{
		Message:      treeMsg,
		GeneralState: treeState,
	})
	require.NoError(t, err)
	mockClient := mockhttpclient.NewURLMock()
	mockClient.Mock(treeStatusURL, mockhttpclient.MockGetDialogue(serialized))

	return &TreeStatusVerifier{
		httpClient:    mockClient.Client(),
		treeStatusURL: treeStatusURL,
	}
}

func TestVerify_ClosedTree(t *testing.T) {
	unittest.SmallTest(t)

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	treeVerifier := setupTreeStatusVerifier(t, "Tree is closed", tree_status_types.ClosedState)

	state, _, err := treeVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierWaitingState, state)
}

func TestVerify_CautionTree(t *testing.T) {
	unittest.SmallTest(t)

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	treeVerifier := setupTreeStatusVerifier(t, "Tree is caution", tree_status_types.CautionState)

	state, _, err := treeVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierWaitingState, state)
}

func TestVerify_OpenTree(t *testing.T) {
	unittest.SmallTest(t)

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	treeVerifier := setupTreeStatusVerifier(t, "Tree is open", tree_status_types.OpenState)

	state, _, err := treeVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)
}

func TestVerify_NoTreeChecksFooter(t *testing.T) {
	unittest.SmallTest(t)

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	treeVerifier := setupTreeStatusVerifier(t, "Tree is closed", tree_status_types.ClosedState)
	treeVerifier.footersMap = map[string]string{
		string(footers.NoTreeChecksFooter): "true",
	}

	// Verify should return success even though the tree is closed because
	// of the footer.
	state, _, err := treeVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierSuccessState, state)

	// Change the footer to false to make sure the tree is checked.
	treeVerifier.footersMap[string(footers.NoTreeChecksFooter)] = "false"
	state, _, err = treeVerifier.Verify(context.Background(), ci, int64(333))
	require.Nil(t, err)
	require.Equal(t, types.VerifierWaitingState, state)
}
