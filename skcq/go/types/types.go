package types

// CQRecord is the description of an entry that is currently being processed
// by SkCQ.
type CQRecord struct {
	ChangeID   int64  `json:"change_id"`
	PatchsetID int64  `json:"patchset_id"`
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`

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
	Changes []*CQRecord `json:"changes"`
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
