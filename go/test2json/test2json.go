package test2json

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

/*
	Package test2json provides utilities for parsing Golang test output in
	JSON format. It mimics https://golang.org/src/cmd/internal/test2json
	which we are unable to use because it is internal.
*/

const (
	// Possible values for Event.Action.
	ActionBench  = "bench"
	ActionCont   = "cont"
	ActionFail   = "fail"
	ActionOutput = "output"
	ActionPass   = "pass"
	ActionPause  = "pause"
	ActionRun    = "run"
	ActionSkip   = "skip"
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

func ParseEvent(s string) (*Event, error) {
	var event Event
	if err := json.NewDecoder(bytes.NewReader([]byte(s))).Decode(&event); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode Event")
	}
	return &event, nil
}

// EventStream returns a channel which emits Events as they appear on the given
// io.Reader.
func EventStream(r io.Reader) <-chan *Event {
	rv := make(chan *Event)
	go func() {
		defer func() {
			close(rv)
		}()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			event, err := ParseEvent(line)
			if err != nil {
				sklog.Errorf("Failed to decode JSON from stream: %s", err)
				continue
			}
			rv <- event
		}
	}()
	return rv
}
