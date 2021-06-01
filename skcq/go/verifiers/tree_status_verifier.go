package verifiers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/tree_status/go/types"
)

func NewTreeStatusVerifier(httpClient *http.Client, treeStatusURL string) (Verifier, error) {
	return &TreeStatusVerifier{
		httpClient:    httpClient,
		treeStatusURL: treeStatusURL,
	}, nil
}

type TreeStatusVerifier struct {
	httpClient    *http.Client
	treeStatusURL string
}

type TreeStatus struct {
	Message      string `json:"message" datastore:"message"`
	GeneralState string `json:"general_state" datastore:"general_state,omitempty"`
}

func (tv *TreeStatusVerifier) Name() string {
	return "[TreeStatusVerifier]"
}

func (tv *TreeStatusVerifier) Verify(ci *gerrit.ChangeInfo) (state VerifierState, reason string, err error) {
	resp, err := tv.httpClient.Get(tv.treeStatusURL)
	if err != nil {
		return "", "", skerr.Fmt("Could not get response from %s: %s", tv.treeStatusURL, err)
	}
	var treeStatus TreeStatus
	if err := json.NewDecoder(resp.Body).Decode(&treeStatus); err != nil {
		return "", "", skerr.Fmt("Could not decode response from %s: %s", tv.treeStatusURL, err)
	}
	if treeStatus.GeneralState == types.OpenState {
		return SuccessState, fmt.Sprintf("%s Tree is open: \"%s\"", tv.Name(), treeStatus.Message), nil
	} else {
		return WaitingState, fmt.Sprintf("%s Waiting for tree to be open. Tree is currently in %s state: \"%s\"", tv.Name(), treeStatus.GeneralState, treeStatus.Message), nil
	}
}

func (cv *TreeStatusVerifier) Cleanup(ci *gerrit.ChangeInfo) {
	return
}
