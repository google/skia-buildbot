package verifiers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
	tree_status_types "go.skia.org/infra/tree_status/go/types"
)

func NewTreeStatusVerifier(httpClient *http.Client, treeStatusURL string, footersMap map[string]string) (types.Verifier, error) {
	return &TreeStatusVerifier{
		httpClient:    httpClient,
		treeStatusURL: treeStatusURL,
		footersMap:    footersMap,
	}, nil
}

type TreeStatusVerifier struct {
	httpClient    *http.Client
	treeStatusURL string
	footersMap    map[string]string
}

type TreeStatus struct {
	Message      string `json:"message" datastore:"message"`
	GeneralState string `json:"general_state" datastore:"general_state,omitempty"`
}

func (tv *TreeStatusVerifier) Name() string {
	return "TreeStatusVerifier"
}

func (tv *TreeStatusVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {

	// Check to see if NoTreeChecksFooter has been specified.
	noTreeChecks := footers.GetBoolVal(tv.footersMap, footers.NoTreeChecksFooter, ci.Issue)
	if noTreeChecks {
		return types.VerifierSuccessState, fmt.Sprintf("Tree check is skipped because \"%s: %t\" has been specified", footers.NoTreeChecksFooter, noTreeChecks), nil
	}

	resp, err := tv.httpClient.Get(tv.treeStatusURL)
	if err != nil {
		return "", "", skerr.Fmt("Could not get response from %s: %s", tv.treeStatusURL, err)
	}
	var treeStatus TreeStatus
	if err := json.NewDecoder(resp.Body).Decode(&treeStatus); err != nil {
		return "", "", skerr.Fmt("Could not decode response from %s: %s", tv.treeStatusURL, err)
	}
	if treeStatus.GeneralState == tree_status_types.OpenState {
		return types.VerifierSuccessState, fmt.Sprintf("Tree is open: \"%s\"", treeStatus.Message), nil
	} else {
		return types.VerifierWaitingState, fmt.Sprintf("Waiting for tree to be open. Tree is currently in %s state: \"%s\"", treeStatus.GeneralState, treeStatus.Message), nil
	}
}

func (cv *TreeStatusVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
