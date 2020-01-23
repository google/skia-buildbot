package test2json

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"time"

	"go.skia.org/infra/go/sklog"
)

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

// EventStream returns a channel which emits Events as they appear on the given
// io.Reader.
func EventStream(r io.Reader) <-chan *Event {
	rv := make(chan *Event)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			var event Event
			if err := json.NewDecoder(bytes.NewReader([]byte(line))).Decode(&event); err != nil {
				sklog.Errorf("Failed to decode JSON from stream: %s", err)
				continue
			}
			rv <- &event
		}
		close(rv)
	}()
	return rv
}
