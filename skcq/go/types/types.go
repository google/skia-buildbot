package types

// StatusData is used in the response of the get_client_counts endpoint.
type StatusData struct {
	UntriagedCount int    `json:"untriaged_count"`
	Link           string `json:"link"`
}

type CQRecord struct {
	// ID         string `json:"id"` // DO YOU REALLY NEED THIS??
	ChangeID   int64  `json:"change_id"`
	PatchsetID int64  `json:"patchset_id"`
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	// The time the CQ first looked at this change.
	// Uses unix epoch time.
	StartTime int64 `json:"start_time"`
	// Whether this change is from an internal repo.
	Internal bool `json:"internal"`

	// For display in UI
	ChangeSubject string `json:"change_subject"`
	//For display in UI
	ChangeOwner string `json:"change_owner"`
	// For display in UI
	DryRun bool `json:"dry_run"`
}

type GetCurrentChangesRequest struct {
	IsDryRun bool `json:"is_dry_run"`
}

type GetCurrentChangesResponse struct {
	Changes []*CQRecord `json:"changes"`
}

type GetChangeAttemptsRequest struct {
	ChangeID   int64 `json:"change_id"`
	PatchsetID int64 `json:"patchset_id"`
}

type GetChangeAttemptsResponse struct {
	ChangeAttempts *ChangeAttempts `json:"change_attempts"`
}

type ChangeAttempts struct {
	Attempts []*ChangeAttempt `json:"attempts"`
}

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

type VerifierState string

const VerifierSuccessState VerifierState = "SUCCESSFUL"
const VerifierWaitingState VerifierState = "WAITING"
const VerifierFailureState VerifierState = "FAILURE"
const VerifierAbortedState VerifierState = "ABORTED"

type VerifierStatus struct {
	Name   string        `json:"name"`
	Start  int64         `json:"start"`
	Stop   int64         `json:"stop"`
	Reason string        `json:"reason"`
	State  VerifierState `json:"state"`
}
