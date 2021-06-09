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
	// Used to make sure footers.CQForceRerunFooter is processed exactly once
	// per CQ attempt.
	ForceRerunProcessed bool `json:"force_rerun_processed"`
	// Whether this change is from an internal repo.
	Internal bool `json:"internal"`
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
