package types

import (
	"time"
)

// StatusData is used in the response of the get_client_counts endpoint.
type StatusData struct {
	UntriagedCount int    `json:"untriaged_count"`
	Link           string `json:"link"`
}

type CQRecord struct {
	// The time the CQ first looked at this change.
	// Uses unix epoch time.
	StartTime int64
}

type ChangeData struct {
	PatchStart         time.Time         `json:"created"`
	PatchStop          time.Time         `json:"stop"`
	PatchCommitted     time.Time         `json:"committed"`
	SubmittableChanges []string          `json:"submittable_changes"`
	VerifiersStatuses  []*VerifierStatus `json:"verifiers_status"`
}

type VerifierState string

const VerifierSuccessState = "SUCCESSFUL"
const VerifierWaitingState = "WAITTING"
const VerifierFailureState = "FAILURE"

type VerifierStatus struct {
	Name   string        `json:"name"`
	Start  time.Time     `json:"start"`
	Stop   time.Time     `json:"stop"`
	Reason string        `json:"reason"`
	State  VerifierState `json:"state"`
}
