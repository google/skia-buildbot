package verifiers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
	tree_status_types "go.skia.org/infra/tree_status/go/types"
)

// NewTreeStatusVerifier returns an instance of TreeStatusVerifier.
func NewTreeStatusVerifier(httpClient *http.Client, treeStatusURL string, footersMap map[string]string) (types.Verifier, error) {
	return &TreeStatusVerifier{
		httpClient:    httpClient,
		treeStatusURL: treeStatusURL,
		footersMap:    footersMap,
	}, nil
}

// TreeStatusVerifier implements the types.Verifier interface.
type TreeStatusVerifier struct {
	httpClient    *http.Client
	treeStatusURL string
	footersMap    map[string]string
}

type TreeStatus struct {
	Message      string `json:"message" datastore:"message"`
	GeneralState string `json:"general_state" datastore:"general_state,omitempty"`
}

// Name implements the types.Verifier interface.
func (tv *TreeStatusVerifier) Name() string {
	return "TreeStatusVerifier"
}

// Verify implements the types.Verifier interface.
func (tv *TreeStatusVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {

	// Check to see if NoTreeChecksFooter has been specified.
	noTreeChecks := git.GetBoolFooterVal(tv.footersMap, footers.NoTreeChecksFooter, ci.Issue)
	if noTreeChecks {
		return types.VerifierSuccessState, fmt.Sprintf("Tree check is skipped because \"%s: %t\" has been specified", footers.NoTreeChecksFooter, noTreeChecks), nil
	}

	resp, err := tv.httpClient.Get(tv.treeStatusURL)
	if err != nil {
		return "", "", skerr.Wrapf(err, "Could not get response from %s", tv.treeStatusURL)
	}
	var treeStatus TreeStatus
	if err := json.NewDecoder(resp.Body).Decode(&treeStatus); err != nil {
		return "", "", skerr.Wrapf(err, "Could not decode response from %s", tv.treeStatusURL)
	}
	if treeStatus.GeneralState == tree_status_types.OpenState {
		return types.VerifierSuccessState, fmt.Sprintf("Tree is open: \"%s\"", treeStatus.Message), nil
	} else {
		return types.VerifierWaitingState, fmt.Sprintf("Waiting for tree to be open. Tree is currently in %s state: \"%s\"", treeStatus.GeneralState, treeStatus.Message), nil
	}
}

// Cleanup implements the types.Verifier interface.
func (cv *TreeStatusVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
