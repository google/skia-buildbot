package verifiers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
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
	Cleanup(ci *gerrit.ChangeInfo)
}

// GetVerifiers returns all the verifiers that apply to the specified change using the specified config.
func GetVerifiers(ctx context.Context, httpClient *http.Client, cfg *config.SkCQCfg, cr codereview.CodeReview, ci *gerrit.ChangeInfo, isCQ, isDryRun bool) ([]Verifier, error) {
	clVerifiers := []Verifier{}
	if isCQ {
		// Verify the owner is a committer.
		committerVerifier, err := NewCommitterListVerifier(httpClient, cfg.CommitterList)
		if err != nil {
			return nil, skerr.Fmt("Error when creating CommitterVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, committerVerifier)

		// Verify the change has approval from a committer.
		approvedVerifier, err := NewApprovedVerifier(httpClient, cfg.CommitterList)
		if err != nil {
			return nil, skerr.Fmt("Error when creating ApprovedVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, approvedVerifier)

		// Verify all the submitted together changes (if any exist).
		togetherChanges, err := cr.GetSubmittedTogether(ctx, ci)
		if err != nil {
			return nil, skerr.Fmt("Error when getting submitted together chagnes for SubmittedTogetherVerifier: %s", err)
		}
		if len(togetherChanges) > 0 {
			fmt.Printf("\n%d HAS TOGETHER CHANGES!!!!!!!! THEY ARE-", ci.Issue)
			for _, tc := range togetherChanges {
				fmt.Println(tc.Issue)
			}
			fmt.Println("----")
			togetherChangesVerifier, err := NewSubmittedTogetherVerifier(ctx, togetherChanges, httpClient, cfg, cr, ci)
			if err != nil {
				return nil, skerr.Fmt("Error when creating SubmittedTogetherVerifier: %s", err)
			}
			clVerifiers = append(clVerifiers, togetherChangesVerifier)
		}

	} else if isDryRun {
		// Verify that the CL owner that run a dry run.
		dryRunVerifier, err := NewDryRunAccessListVerifier(httpClient, cfg.DryRunAccessList)
		if err != nil {
			return nil, skerr.Fmt("Error when creating DryRunVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, dryRunVerifier)
	}

	if cfg.TreeStatusURL != "" {
		// Verify that the tree is open.
		treeStatusVerifier, err := NewTreeStatusVerifier(httpClient, cfg.TreeStatusURL)
		if err != nil {
			return nil, skerr.Fmt("Error when creating TreeStatusVerifier: %s", err)
		}
		clVerifiers = append(clVerifiers, treeStatusVerifier)
	}

	return clVerifiers, nil
}

// RunVerifiers runs all the specified verifiers for the change.
func RunVerifiers(ci *gerrit.ChangeInfo, verifiers []Verifier) (successMsgsFromVerfiers, waitMsgsFromVerifiers, rejectMsgFromVerifiers []string) {
	// TODO(Rmistry): What happens if we comment out the below
	successMsgsFromVerfiers = []string{}
	waitMsgsFromVerifiers = []string{}
	rejectMsgFromVerifiers = []string{}
	for _, v := range verifiers {
		verifierState, reason, err := v.Verify(ci)
		if err != nil {
			// Should we always consider errors as transient errors? We will always log them for alerts.
			errMsg := fmt.Sprintf("%s: Hopefully a transient error: %s", v.Name(), err)
			sklog.Errorf(errMsg)
			waitMsgsFromVerifiers = append(waitMsgsFromVerifiers, errMsg)
		} else {
			switch verifierState {
			case SuccessState:
				successMsgsFromVerfiers = append(successMsgsFromVerfiers, reason)
			case WaitingState:
				waitMsgsFromVerifiers = append(waitMsgsFromVerifiers, reason)
			case FailureState:
				rejectMsgFromVerifiers = append(rejectMsgFromVerifiers, reason)
			}
		}
	}
	return successMsgsFromVerfiers, waitMsgsFromVerifiers, rejectMsgFromVerifiers
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

func (cv *CommitterListVerifier) Cleanup(ci *gerrit.ChangeInfo) {
	return
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

func (cv *DryRunAccessListVerifier) Cleanup(ci *gerrit.ChangeInfo) {
	return
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

func (cv *TreeStatusVerifier) Cleanup(ci *gerrit.ChangeInfo) {
	return
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
	return "[ApprovedVerifier]"
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

func (cv *ApprovedVerifier) Cleanup(ci *gerrit.ChangeInfo) {
	return
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

func NewSubmittedTogetherVerifier(ctx context.Context, togetherChanges []*gerrit.ChangeInfo, httpClient *http.Client, cfg *config.SkCQCfg, cr codereview.CodeReview, ci *gerrit.ChangeInfo) (Verifier, error) {
	togetherChangesToVerifiers := map[*gerrit.ChangeInfo][]Verifier{}
	for _, tc := range togetherChanges {
		tcVerifiers, err := GetVerifiers(ctx, httpClient, cfg, cr, tc, true /* isCQ */, false /* isDryRun */)
		if err != nil {
			return nil, skerr.Fmt("Could not get verifiers for the together change %d: %s", tc.Issue, err)
		}
		togetherChangesToVerifiers[tc] = tcVerifiers
	}
	return &SubmittedTogetherVerifier{
		togetherChangesToVerifiers: togetherChangesToVerifiers,
	}, nil
}

type SubmittedTogetherVerifier struct {
	togetherChangesToVerifiers map[*gerrit.ChangeInfo][]Verifier
}

func (stv *SubmittedTogetherVerifier) Name() string {
	return "[SubmittedTogetherVerifier]"
}

func (stv *SubmittedTogetherVerifier) Verify(ci *gerrit.ChangeInfo) (state VerifierState, reason string, err error) {
	successMsgsFromVerfiers := []string{}
	// Loop through all verifiers of all together changes and run them.
	fmt.Println("VERIFYING TOGETHER CHANGES!Q!!!!")
	for ts, tsVerifiers := range stv.togetherChangesToVerifiers {
		// FOR DEBUGGING!
		for _, tsVerifier := range tsVerifiers {
			fmt.Println(tsVerifier.Name())
		}
		sm, wm, rm := RunVerifiers(ts, tsVerifiers)
		if len(rm) > 0 {
			return FailureState, fmt.Sprintf("Submitted together change %d has failed verifiers: %s", ts.Issue, strings.Join(rm, "\n")), nil
		} else if len(wm) > 0 {
			return WaitingState, fmt.Sprintf("Submitted together change %d is waiting for verifiers: %s", ts.Issue, strings.Join(wm, "\n")), nil
		} else {
			successMsgsFromVerfiers = append(successMsgsFromVerfiers, sm...)
		}
	}
	return SuccessState, strings.Join(successMsgsFromVerfiers, "\n"), nil
}

func (stv *SubmittedTogetherVerifier) Cleanup(ci *gerrit.ChangeInfo) {
	// Loop through all verifiers of all together changes and run cleanup on them.
	for ts, tsVerifiers := range stv.togetherChangesToVerifiers {
		for _, tsVerifier := range tsVerifiers {
			tsVerifier.Cleanup(ts)
		}
	}
	return
}
