package types

import (
	"context"
	"net/http"
	"time"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
)

// ThrottlerManager is used to manage the rate of commits.
type ThrottlerManager interface {
	// Throttle looks at the specified commit time and determines if the
	// commit should be blocked because it violates the throttler config.
	// Eg:
	//     If the throttler config has MaxBurst=2 and BurstDelaySecs=120
	//     That means that 2 commits are allowed every 2 mins. Throttle
	//     will return true if a 3rd commit comes in within that 2 min
	//     window. Once the window slides Throttle will return false for
	//     the next commit.
	Throttle(repoBranch string, commitTime time.Time) bool

	// UpdateThrottler adds the specified commit to the throttler cache.
	UpdateThrottler(repoBranch string, commitTime time.Time, throttlerCfg *config.ThrottlerCfg)
}

// Verifier is the interface implemented by all verifiers.
type Verifier interface {
	// Name of the verifier.
	Name() string

	// Verify runs the verifier and returns a VerifierState with a string
	// explaining why it is in that state.
	Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state VerifierState, reason string, err error)

	// Cleanup runs any cleanup tasks that the verifier needs to execute
	// when a change is removed from the CQ. Does not return an error
	// but all errors will be logged.
	Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64)
}

// VerifiersManager helps callers find verifiers and then run them. Useful for
// mocking out functions for testing.
type VerifiersManager interface {

	// GetVerifiers returns all verifiers that need to be run on the specified
	// change. Returns a slice of verifiers and a slice of all changes that
	// will be submitted together with this change.
	GetVerifiers(ctx context.Context, httpClient *http.Client, cfg *config.SkCQCfg, cr codereview.CodeReview, ci *gerrit.ChangeInfo, isSubmittedTogetherChange bool, configReader config.ConfigReader) (verifiers []Verifier, togetherChanges []string, err error)

	// RunVerifiers runs all the specified verifiers and returns their statuses.
	// Does not return an error because errors during verification of individual
	// verifiers are assumed to be transient and are logged and
	// VerifierWaitingState is returned for that particular verifier.
	//
	// TODO(rmistry): Should we instead record and limit the number of
	// consecutive errors allowed for a verifier and return an error if that
	// limit is exceeded?
	RunVerifiers(ctx context.Context, ci *gerrit.ChangeInfo, verifiers []Verifier, startTime int64) []*VerifierStatus
}

// CurrentlyProcessingChange is the description of an entry that is currently
// being processed by SkCQ.
type CurrentlyProcessingChange struct {
	ChangeID         int64  `json:"change_id"`
	LatestPatchsetID int64  `json:"latest_patchset_id"`
	Repo             string `json:"repo"`
	Branch           string `json:"branch"`
	ChangeSubject    string `json:"change_subject"`
	ChangeOwner      string `json:"change_owner"`
	DryRun           bool   `json:"dry_run"`

	// The time the CQ first looked at this change.
	// Uses unix epoch time.
	StartTs int64 `json:"start_ts"`
	// Whether this change is from an internal repo.
	Internal bool `json:"internal"`
}

// GetCurrentChangesRequest is the request used by the get_current_changes
// endpoint.
type GetCurrentChangesRequest struct {
	IsDryRun bool `json:"is_dry_run"`
}

// GetCurrentChangesRequest is the response used by the get_current_changes
// endpoint.
type GetCurrentChangesResponse struct {
	Changes []*CurrentlyProcessingChange `json:"changes"`
}

// GetChangeAttemptsRequest is the request used by the get_change_attempts
// endpoint.
type GetChangeAttemptsRequest struct {
	ChangeID   int64 `json:"change_id"`
	PatchsetID int64 `json:"patchset_id"`
}

// GetChangeAttemptsRequest is the response used by the get_change_attempts
// endpoint.
type GetChangeAttemptsResponse struct {
	ChangeAttempts *ChangeAttempts `json:"change_attempts"`
}

// ChangeAttempts contains a slice of ChangeAttempt(s).
type ChangeAttempts struct {
	Attempts []*ChangeAttempt `json:"attempts"`
}

// ChangeAttempt describes each attempt by the CQ on a change+patchset.
type ChangeAttempt struct {
	ChangeID   int64  `json:"change_id"`
	PatchsetID int64  `json:"patchset_id"`
	DryRun     bool   `json:"dry_run"`
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`

	// The time the CQ first started looking at this attempt.
	PatchStartTs int64 `json:"start_ts"`
	// When the CQ stopped processing this attempt. If the attempt is still
	// being processed then this value will be 0.
	PatchStopTs int64 `json:"stop_ts"`
	// When the CQ committed this attempt. If the attempt is not committed yet
	// then this value will be 0.
	PatchCommittedTs int64 `json:"committed_ts"`
	// Whether this attempt was abandoned. Attempts are abandoned when new code
	// change patchsets are uploaded even though the previous patchset was
	// still running in the CQ.
	CQAbandoned bool `json:"cq_abandoned"`
	// The list of changes that will be submitted at the same time as this
	// change.
	SubmittableChanges []string `json:"submittable_changes"`
	// The current statuses of the verifiers that apply to this attempt.
	VerifiersStatuses []*VerifierStatus `json:"verifiers_statuses"`
	// The overall state of this attempt. This value is directly computed from
	// the value of VerifiersStatuses. If all VerifierStatuses are successful
	// then this state will be VerifierSuccessState. If atleast one VerifierStatus
	// is failure then this state will be VerifierFailureState. If atleast one
	// VerifierStatus is waiting then this state will be VerifierWaitingState.
	OverallState VerifierState `json:"overall_status"`
}

// VerifierState describes the state of the verifier.
type VerifierState string

const VerifierSuccessState VerifierState = "SUCCESSFUL"
const VerifierWaitingState VerifierState = "WAITING"
const VerifierFailureState VerifierState = "FAILURE"
const VerifierAbortedState VerifierState = "ABORTED"

// VerifierStates contains the status of the verify. Includes the name,
// timestamps and the state.
type VerifierStatus struct {
	// The name of the verifier. Eg: TreeStatusVerifier.
	Name string `json:"name"`
	// When the CQ started this verifier.
	StartTs int64 `json:"start_ts"`
	// When the CQ stopped this verifier. Will be 0 if the verifier is still
	// running.
	StopTs int64 `json:"stop_ts"`
	// The current state of the verifier.
	State VerifierState `json:"state"`
	// An explanation of why the verifier is in this state.
	Reason string `json:"reason"`
}
