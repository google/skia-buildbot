package test2json

import "time"

/*
	Package test2json provides utilities for parsing Golang test output in
	JSON format. It mimics https://golang.org/src/cmd/internal/test2json
	which we are unable to use because it is internal.
*/

const (
	// Possible values for Event.Action.
	ACTION_BENCH  = "bench"
	ACTION_CONT   = "cont"
	ACTION_FAIL   = "fail"
	ACTION_OUTPUT = "output"
	ACTION_PASS   = "pass"
	ACTION_PAUSE  = "pause"
	ACTION_RUN    = "run"
	ACTION_SKIP   = "skip"
)

// Event represents a test event.
type Event struct {
	Time    time.Time `json:",omitempty"`
	Action  string
	Package string  `json:",omitempty"`
	Test    string  `json:",omitempty"`
	Elapsed float64 `json:",omitempty"`
	Output  string  `json:",omitempty"`
}
