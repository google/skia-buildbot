package types

// StatusData is used in the response of the get_client_counts endpoint.
type StatusData struct {
	UntriagedCount int    `json:"untriaged_count"`
	Link           string `json:"link"`
}

type CQRecord struct {
	ID int64 `json:"id"`
	// The time the CQ first looked at this change.
	// Uses unix epoch time.
	StartTime int64 `json:"start_time"`
	// Whether this change is from an internal repo.
	Internal bool `json:"internal"`

	// For display in UI
	ChangeSubject string `json:"change_subject"`
	//For display in UI
ChangeOwner string `json:"change_owner"`
}

type ChangeAttempts struct {
	Attempts []*ChangeAttempt `json:"attempts"`
}

type ChangeAttempt struct {
	ID                 string            `json:"id"`
	DryRun             bool              `json:"dry_run"`
	RepoBranch         string            `json:"repo_branch"`
	PatchStart         int64             `json:"created"`
	PatchStop          int64             `json:"stop"`
	PatchCommitted     int64             `json:"committed"`
	SubmittableChanges []string          `json:"submittable_changes"`
	VerifiersStatuses  []*VerifierStatus `json:"verifiers_status"`
	CQAbandoned        bool              `json:"cq_abandoned"`
}

type VerifierState string

const VerifierSuccessState VerifierState = "SUCCESSFUL"
const VerifierWaitingState VerifierState = "WAITTING"
const VerifierFailureState VerifierState = "FAILURE"

type VerifierStatus struct {
	Name   string        `json:"name"`
	Start  int64         `json:"start"`
	Stop   int64         `json:"stop"`
	Reason string        `json:"reason"`
	State  VerifierState `json:"state"`
}
