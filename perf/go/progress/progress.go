// Package progress is for tracking the progress of long running tasks on the
// backend in a way that can be reflected in the UI.
//
// We have multiple long running queries like /frame/start and /dryrun/start
// that start those long running processes and we need to give feedback to the
// user on how they are proceeding.
//
// The dryrun progress information contains different info with different stages
// and steps. For example, dryrun progress looks like this:
//
//   Step: 1/1
//   Query: "sub_result=max_rss_mb"
//   Stage: Looking for regressions in query results.
//   Commit: 51643
//   Details: "Filtered Traces: Num Before: 95 Num After: 92 Delta: 3"
//
// Which is just a series of key/value pairs of strings. So our common Progress
// interface allows for creating a set of key/value pairs to be displayed, along
// with the Status of the current process, and any results once the process has
// finished.
package progress

import (
	"encoding/json"
	"io"
	"sync"
)

// Status of a process.
type Status string

const (
	// Running mean a process is still running.
	Running Status = "Running"

	// Finished means the process has finished.
	Finished Status = "Finished"
)

// AllStatus contains all values of type State.
var AllStatus = []Status{Running, Finished}

// Message is a key value pair of strings, used in SerializedProgress.
type Message struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SerializedProgress is the shape of the JSON emitted from Progress.JSON().
type SerializedProgress struct {
	Status    Status      `json:"status"`
	Messsages []*Message  `json:"messages"`
	Results   interface{} `json:"results,omitempty"`

	// URL to use in the next polling step.
	URL string `json:"url"`
}

// Progress is the interface for reporting on the progress of a long running
// process.
type Progress interface {
	// Message adds or updates a message in a progress recorder. If the key
	// matches an existing message it will replace that key's value.
	Message(key, value string)

	// When the process is complete call Finished with the Results that are to
	// be serialized via SerializedProgress.
	Finished(interface{})

	// JSON writes the data serialized as JSON. The shape is SerializedProgress.
	JSON(w io.Writer) error
}

// progress implements Progress.
type progress struct {
	mutex sync.Mutex
	state SerializedProgress
}

// New returns a new Progress in the Running state.
func New() *progress {
	return &progress{
		state: SerializedProgress{
			Status:    Running,
			Messsages: []*Message{},
		},
	}
}

// Message implements the Progress interface.
func (p *progress) Message(key, value string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for _, m := range p.state.Messsages {
		if m.Key == key {
			m.Value = value
			return
		}
	}
	p.state.Messsages = append(p.state.Messsages, &Message{
		Key:   key,
		Value: value,
	})
}

// Message implements the Progress interface.
func (p *progress) Finished(results interface{}) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.state.Status = Finished
	p.state.Results = results
}

// Message implements the Progress interface.
func (p *progress) JSON(w io.Writer) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return json.NewEncoder(w).Encode(p.state)
}

// Assert that progress implements the Progress interface.
var _ Progress = (*progress)(nil)
