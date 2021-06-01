package verifiers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/tree_status/go/types"
)

type VerifierState string

const (
	SuccessState VerifierState = "Success"
	WaitingState VerifierState = "Waiting"
	FailureState VerifierState = "Failure"
)

type Verifier interface {
	// Name of the verifier.
	Name() string

	// UPDATE THIS DOC!!!
	// Verify runs the verifier.
	// If verification was not successful but SkCQ should wait for the result, then a waitMsg will be
	// returned.
	// If verification was not successful and SkCQ should fail the change, then a rejectMsg will be
	// returned.
	// If there is another infra related error then error will be non-nil.
	// Successful verification will return an empty waitMsg, an empty rejectMsg and nil error.
	// Verify(ci *gerrit.ChangeInfo) (waitMsg string, rejectMsg string, err error)
	Verify(ci *gerrit.ChangeInfo) (state VerifierState, reason string, err error)

	// Cleanup runs any cleanup tasks that the verifier needs to execute before the change is
	// removed from the CQ.
	Cleanup(ci *gerrit.ChangeInfo) error
}

func NewCommitterListVerifier(httpClient *http.Client, criaGroup string) (Verifier, error) {
	// Instatiate this once and pass it in because it's used int he other place as well.
	committerAllowed, err := allowed.NewAllowedFromChromeInfraAuth(httpClient, criaGroup)
	if err != nil {
		return nil, skerr.Fmt("Could not create an allowed from %s: %s", criaGroup, err)
	}
	return &CommitterListVerifier{
		criaGroupName:    criaGroup,
		committerAllowed: committerAllowed,
	}, nil
}

type CommitterListVerifier struct {
	criaGroupName    string
	committerAllowed *allowed.AllowedFromChromeInfraAuth
}

func (cv *CommitterListVerifier) Name() string {
	return "[CommitterListVerifier]"
}

func (cv *CommitterListVerifier) Verify(ci *gerrit.ChangeInfo) (state VerifierState, reason string, err error) {
	committersWhoCQed := GetAllowedVoters(ci, cv.committerAllowed, gerrit.LabelCommitQueue, gerrit.LabelCommitQueueSubmit)
	if len(committersWhoCQed) > 0 {
		return SuccessState, fmt.Sprintf("CQ+2 voted by committers %s", strings.Join(committersWhoCQed, ",")), nil
	}
	return FailureState, "CQ+2 requires a vote from a committer", nil
}

func (cv *CommitterListVerifier) Cleanup(ci *gerrit.ChangeInfo) error {
	return nil
}

func NewDryRunAccessListVerifier(httpClient *http.Client, criaGroup string) (Verifier, error) {
	dryRunAllowed, err := allowed.NewAllowedFromChromeInfraAuth(httpClient, criaGroup)
	if err != nil {
		return nil, skerr.Fmt("Could not create an allowed from %s: %s", criaGroup, err)
	}
	return &DryRunAccessListVerifier{
		criaGroupName: criaGroup,
		dryRunAllowed: dryRunAllowed,
	}, nil
}

type DryRunAccessListVerifier struct {
	criaGroupName string
	dryRunAllowed *allowed.AllowedFromChromeInfraAuth
}

func (cv *DryRunAccessListVerifier) Name() string {
	return "[DryRunAccessListVerifier]"
}

func (cv *DryRunAccessListVerifier) Verify(ci *gerrit.ChangeInfo) (state VerifierState, reason string, err error) {
	allowedDryRunVoters := GetAllowedVoters(ci, cv.dryRunAllowed, gerrit.LabelCommitQueue, gerrit.LabelCommitQueueDryRun)
	if len(allowedDryRunVoters) > 0 {
		return SuccessState, fmt.Sprintf("CQ+1 voted by allowed dry-run voters %s", strings.Join(allowedDryRunVoters, ",")), nil
	}
	return FailureState, "CQ+1 requires a vote by an allowed dry-run voter", nil
}

func (cv *DryRunAccessListVerifier) Cleanup(ci *gerrit.ChangeInfo) error {
	return nil
}

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
		return SuccessState, fmt.Sprintf("Tree is open: \"%s\"", treeStatus.Message), nil
	} else {
		return WaitingState, fmt.Sprintf("Waiting for tree to be open. Tree is currently in %s state: \"%s\"", treeStatus.GeneralState, treeStatus.Message), nil
	}
}

func (cv *TreeStatusVerifier) Cleanup(ci *gerrit.ChangeInfo) error {
	return nil
}

// NEW NEW NEW NEW NEW //

func NewApprovedVerifier(httpClient *http.Client, criaGroup string) (Verifier, error) {
	committerAllowed, err := allowed.NewAllowedFromChromeInfraAuth(httpClient, criaGroup)
	if err != nil {
		return nil, skerr.Fmt("Could not create an allowed from %s: %s", criaGroup, err)
	}
	return &ApprovedVerifier{
		criaGroupName:    criaGroup,
		committerAllowed: committerAllowed,
	}, nil
}

type ApprovedVerifier struct {
	criaGroupName    string
	committerAllowed *allowed.AllowedFromChromeInfraAuth
}

func (av *ApprovedVerifier) Name() string {
	return "[TreeStatusVerifier]"
}

func (av *ApprovedVerifier) Verify(ci *gerrit.ChangeInfo) (state VerifierState, reason string, err error) {
	committersWhoApproved := GetAllowedVoters(ci, av.committerAllowed, gerrit.LabelCodeReview, gerrit.LabelCodeReviewApprove)
	if len(committersWhoApproved) > 0 {
		return SuccessState, fmt.Sprintf("Approved by committers %s", strings.Join(committersWhoApproved, ",")), nil
	}
	// Implement waiting for approval only for owners who are committers?
	// Also, only if there are reviewers and if at least one is a committer.
	return FailureState, "This CL requires approval from a committer", nil
}

func (cv *ApprovedVerifier) Cleanup(ci *gerrit.ChangeInfo) error {
	return nil
}

// GetAllowedVoters is a utility function that looks through the labels on a gerrit change to gather the
// email addresses of voters who voted the specified labelValue and who are in the allowedCRIA group.
func GetAllowedVoters(ci *gerrit.ChangeInfo, allowedCRIA *allowed.AllowedFromChromeInfraAuth, labelName string, labelValue int) []string {
	allowedVoters := []string{}
	if val, ok := ci.Labels[labelName]; ok {
		for _, ld := range val.All {
			if ld.Value == labelValue {
				if allowedCRIA.Member(ld.Email) {
					allowedVoters = append(allowedVoters, ld.Email)
				}
			}
		}
	}
	return allowedVoters
}
