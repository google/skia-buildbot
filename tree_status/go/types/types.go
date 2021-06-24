package types

import "time"

const (
	OpenState    = "open"
	CautionState = "caution"
	ClosedState  = "closed"
)

// AutorollerSnapshot - contains the current state of an autoroller with
// it's display name (eg: "Chrome") and URL (eg: "https://autoroll.skia.org/r/skia-autoroll").
type AutorollerSnapshot struct {
	DisplayName string `json:"name"`
	NumFailed   int    `json:"num_failed"`
	Url         string `json:"url"`
}

// Status - A Tree status.
type Status struct {
	Date     time.Time `json:"date" datastore:"date"`
	Message  string    `json:"message" datastore:"message"`
	Rollers  string    `json:"rollers" datastore:"rollers"`
	Username string    `json:"username" datastore:"username"`

	// Only specified for backwards compatibility.
	FirstRev int `json:"first_rev,omitempty" datastore:"first_rev"`
	LastRev  int `json:"last_rev,omitempty" datastore:"last_rev"`

	// Should be one of open/closed/caution.
	GeneralState string `json:"general_state" datastore:"general_state,omitempty"`
}
