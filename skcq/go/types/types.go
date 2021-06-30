package types

import (
	"context"
	"net/http"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
)

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

	// GetVerifiers returns all verifiers that need to be run on the specified change.
	GetVerifiers(ctx context.Context, httpClient *http.Client, cfg *config.SkCQCfg, cr codereview.CodeReview, ci *gerrit.ChangeInfo, isSubmittedTogetherChange bool, configReader config.ConfigReader) ([]Verifier, []string, error)

	// RunVerifiers runs all the specified verifiers and returns their statuses.
	RunVerifiers(ctx context.Context, ci *gerrit.ChangeInfo, verifiers []Verifier, startTime int64) []*VerifierStatus
}

// CurrentlyProcessingChange is the description of an entry that is currently
// being processed by SkCQ.
type CurrentlyProcessingChange struct {
	ChangeID             int64  `json:"change_id"`
	EquivalentPatchsetID int64  `json:"equivalent_patchset_id"`
	Repo                 string `json:"repo"`
	Branch               string `json:"branch"`

	// The time the CQ first looked at this change.
	// Uses unix epoch time.
	StartTime int64 `json:"start_time"`
	// Whether this change is from an internal repo.
	Internal bool `json:"internal"`

	// For displaying in UI
	ChangeSubject string `json:"change_subject"`
	ChangeOwner   string `json:"change_owner"`
	DryRun        bool   `json:"dry_run"`
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
	ChangeID           int64             `json:"change_id"`
	PatchsetID         int64             `json:"patchset_id"`
	DryRun             bool              `json:"dry_run"`
	Repo               string            `json:"repo"`
	Branch             string            `json:"branch"`
	PatchStart         int64             `json:"created"`
	PatchStop          int64             `json:"stop"`
	PatchCommitted     int64             `json:"committed"`
	CQAbandoned        bool              `json:"cq_abandoned"`
	SubmittableChanges []string          `json:"submittable_changes"`
	VerifiersStatuses  []*VerifierStatus `json:"verifiers_statuses"`
	OverallState       VerifierState     `json:"overall_status"`
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
	Name   string        `json:"name"`
	Start  int64         `json:"start"`
	Stop   int64         `json:"stop"`
	Reason string        `json:"reason"`
	State  VerifierState `json:"state"`
}
