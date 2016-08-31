// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tracer

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

// Scope is used with Instant event to determine the scope of the instantaneous
// event.
type Scope string

// Possible scopes that can be passed to Instant.
const (
	Global  Scope = "g"
	Process Scope = "p"
	Thread  Scope = "t"
)

// Args is user-defined arguments for an event. It can be anything as long as
// it is JSON serializable.
type Args map[string]interface{}

// Start starts the trace.
//
// There can be only one trace at a time. If a trace was already started, the
// current trace will not be affected and an error will be returned.
//
// Initial context has pid 1 and tid 1. Stop() must be called on exit to
// generate a valid JSON trace file.
//
// If stackDepth is non-zero, up to 'stackDepth' PC entries are kept for each
// log entry.
//
// Tracing events before this call are ignored.
//
// TODO(maruel): Implement stackDepth.
func Start(w io.Writer, stackDepth int) error {
	globals.lockWriter.Lock()
	defer globals.lockWriter.Unlock()
	if globals.out != nil {
		return errors.New("tracer was already started")
	}

	globals.lockContexts.Lock()
	defer globals.lockContexts.Unlock()
	globals.contexts = map[interface{}]*context{}
	globals.nextPID = 2 // Initial context (the current one) has PID 1.
	globals.lockID.Lock()
	defer globals.lockID.Unlock()

	globals.out = w
	globals.encoder = json.NewEncoder(globals.out)
	globals.first = true
	wd, _ := os.Getwd()
	args := Args{
		"args":   os.Args,
		"cwd":    wd,
		"goroot": runtime.GOROOT(),
	}

	// {
	//   "context": { ... },
	//   "traceEvents": [
	//     { ..., "ph": "B", "name": "A", "sf": 7},
	//     { ..., "ph": "E", "name": "A", "sf": 9}
	//   ],
	//   "stackFrames": {
	//     5: { "name": "main", "category": "my app" },
	//     7: { "parent": 5, "name": "SomeFunction", "category": "my app" },
	//     9: { "parent": 5, "name": "SomeFunction", "category": "my app" }
	//   }
	// }
	var err error
	if _, err = globals.out.Write(headerContext); err == nil {
		if err = globals.encoder.Encode(args); err == nil {
			_, err = globals.out.Write(headerEvents)
		}
	}
	if err != nil {
		// Unroll initialization.
		globals.out = nil
		globals.contexts = nil
		globals.nextPID = 0
		globals.nextID = 0
	} else {
		// Improve measurements when tracing is enabled.
		increaseClockFrequency()
	}
	return err
}

// Stop stops the trace.
//
// It is important to call it so the trace file is properly formatted. Tracing
// events after this call are ignored.
func Stop() {
	// Wait for on-going traces.
	globals.wg.Wait()
	globals.lockWriter.Lock()
	defer globals.lockWriter.Unlock()
	globals.lockContexts.Lock()
	defer globals.lockContexts.Unlock()
	if globals.out != nil {
		// TODO(maruel): Dump all the stack frames.
		_, _ = globals.out.Write(footerEvents)
	}
	globals.lockID.Lock()
	defer globals.lockID.Unlock()
	globals.out = nil
	globals.contexts = nil
	globals.nextPID = 0
	globals.nextID = 0

	// Lower back clock frequency once we're done.
	lowerClockFrequency()
}

// Span defines an event with a duration.
//
// The caller MUST call the returned callback to 'close' the event. The
// callback doesn't need to be called from the same goroutine as the initial
// caller.
func Span(marker interface{}, name string, args Args) func(args Args) {
	c := getContext(marker)
	if c == nil {
		return dummy
	}
	tsStart := time.Since(consts.start)
	return func(argsEnd Args) {
		tsEnd := time.Since(consts.start)
		if tsEnd == tsStart {
			// Make sure a duration event lasts at least one nanosecond.
			// It is a problem on systems with very low resolution clock
			// like Windows where the clock is so coarse that a large
			// number of events would not show up on the UI.
			tsEnd++
		}
		// Use a pair of eventBegin/eventEnd.
		// getID() is a locking call.
		id := getID()
		// Remove once https://github.com/google/trace-viewer/issues/963 is rolled
		// into Chrome stable.
		if args == nil {
			args = consts.fakeArgs
		}
		if argsEnd == nil {
			argsEnd = consts.fakeArgs
		}
		c.emit(&event{
			Type:      eventNestableBegin,
			Category:  "ignored",
			Name:      name,
			Args:      args,
			Timestamp: fromDuration(tsStart),
			ID:        id,
		})
		c.emit(&event{
			Type:      eventNestableEnd,
			Category:  "ignored",
			Name:      name,
			Args:      argsEnd,
			Timestamp: fromDuration(tsEnd),
			ID:        id,
		})
	}
}

// Instant registers an intantaneous event that has no duration.
func Instant(marker interface{}, name string, s Scope, args Args) {
	if c := getContext(marker); c != nil {
		if args == nil {
			args = consts.fakeArgs
		}
		// getID() is a locking call.
		c.emit(&event{
			Type:     eventNestableInstant,
			Category: "ignored",
			Name:     name,
			Scope:    s,
			Args:     args,
			ID:       getID(),
		})
	}
}

// CounterSet registers a new value for a counter.
//
// The values will be grouped inside the PID and each name displayed as a
// separate line.
func CounterSet(marker interface{}, name string, value float64) {
	if c := getContext(marker); c != nil {
		// Sets ID so operations can be ordered.
		c.lock.Lock()
		c.counters[name] = value
		c.lock.Unlock()
		c.emit(&event{
			Type: eventCounter,
			Name: name,
			Args: Args{"value": value},
			ID:   getID(),
		})
	}
}

// CounterAdd increments a value for a counter.
//
// The values will be grouped inside the PID and each name displayed as a
// separate line.
func CounterAdd(marker interface{}, name string, value float64) {
	if c := getContext(marker); c != nil {
		// Sets ID so operations can be ordered.
		c.lock.Lock()
		value += c.counters[name]
		c.counters[name] = value
		c.lock.Unlock()
		c.emit(&event{
			Type: eventCounter,
			Name: name,
			Args: Args{"value": value},
			ID:   getID(),
		})
	}
}

// NewPID assigns a pseudo-process ID for this marker and TID 1.
//
// Optionally assigns name to the 'process'. The main use is to create a
// logical group for events.
func NewPID(marker interface{}, pname string) {
	globals.lockContexts.Lock()
	defer globals.lockContexts.Unlock()
	if globals.contexts == nil {
		return
	}
	newPID := globals.nextPID
	globals.nextPID++
	c := &context{pid: newPID, counters: map[string]float64{}}
	globals.contexts[marker] = c
	if pname != "" {
		c.metadata(processName, Args{"name": pname})
	}
}

// Discard forgets a context association created with NewPID.
//
// If not called, contexts accumulates and form a memory leak.
func Discard(marker interface{}) {
	globals.lockContexts.Lock()
	defer globals.lockContexts.Unlock()
	delete(globals.contexts, marker)
}

// Private stuff.

var headerContext = []byte("{\"context\":")
var headerEvents = []byte(",\"traceEvents\":[")
var footerEvents = []byte("]}")

// consts is all the constants relating to this package. Having the constants
// not in a struct makes the code unreadable.
var consts = struct {
	start          time.Time
	defaultContext context
	// Remove once https://github.com/google/trace-viewer/issues/963 is rolled
	// into Chrome stable.
	fakeArgs map[string]interface{}
}{
	start:          time.Now().UTC(),
	defaultContext: context{pid: 1, counters: map[string]float64{}},
	fakeArgs:       map[string]interface{}{"ignored": 0.},
}

// globals is all the globals relating to this package. Having the globals not
// in a struct makes the code unreadable.
var globals struct {
	lockContexts sync.Mutex
	contexts     map[interface{}]*context
	nextPID      int
	wg           sync.WaitGroup // Used to wait for all goroutines to complete on Stop().

	lockWriter sync.Mutex
	out        io.Writer
	encoder    *json.Encoder
	first      bool

	lockID sync.Mutex
	nextID int
}

// eventType is one of the supported event type by
// https://github.com/google/trace-viewer.
type eventType string

const (
	// Duration Events. Duration events provide a way to mark a duration of work
	// on a given thread. These can be nested in the same Tid but must not be
	// overlapped.
	eventBegin eventType = "B"
	eventEnd   eventType = "E"

	// Complete Events. Each complete event logically combines a pair of duration
	// (B and E) events.
	eventComplete eventType = "X"

	// Instant Events. Thread/process/global instantaneous event. The instant
	// events correspond to something that happens but has no time associated
	// with it. For example, vblank events are considered instant events. Using
	// Scope.
	eventInstant eventType = "i"

	// Counter Events. The counter events can track a value or multiple values as
	// they change over time. Used to count values via Args.
	eventCounter eventType = "C"

	// Async Events. Events that flows multiple threads, referenced by ID instead
	// of relying on Tid. They can overload within the same thread.
	eventNestableBegin   eventType = "b"
	eventNestableEnd     eventType = "e"
	eventNestableInstant eventType = "n"

	// Flow Events. Essentially an arrow between two Duration events.
	eventFlowStart eventType = "s"
	eventFlowEnd   eventType = "f"
	eventFlowStep  eventType = "t"

	// Sample Events. Sample events provide a way of adding sampling-profiler
	// based results in a trace.
	eventSample eventType = "P"

	// Object Events. Used to track object lifetime.
	eventCreated   eventType = "N"
	eventSnapshot  eventType = "O"
	eventDestroyed eventType = "D"

	// Metadata Events. Metadata events are used to associate extra information
	// with the events in the trace file. Using metadataType.
	eventMetadata eventType = "M"

	// Memory Dump Events
	//
	// Global memory dump events, which contain system memory information such as
	// the size of RAM.
	eventGlobal eventType = "V"
	// Process memory dump events, which contain information about a single
	// process’s memory usage (e.g. total allocated memory).
	eventProcess eventType = "v"
)

// metadataType is used with Metadata events.
type metadataType string

const (
	// Sets the display name for the provided pid. The name is provided in a name
	// argument.
	processName metadataType = "process_name"
	// Sets the extra process labels for the provided pid. The label is provided
	// in a labels argument.
	processLabels metadataType = "process_labels"
	// Sets the process sort order position. The sort index is provided in a
	// sort_index argument.
	processSortIndex metadataType = "process_sort_index"
	// Sets the name for the given tid. The name is provided in a name argument.
	threadName metadataType = "thread_name"
	// Sets the thread sort order position. The sort index is provided in a
	// sort_index argument.
	threadSortIndex metadataType = "thread_sort_index"
)

// event is a single trace line.
//
// See format description at
// https://docs.google.com/document/d/1CvAClvFfyA5R-PhYUmn5OOQtYMH4h6I0nSsKchNAySU/preview
type event struct {
	Pid       int          `json:"pid"`            // Required. Process ID.
	Tid       int          `json:"tid"`            // Required. Thread ID. It is implicitly used to set start/end.
	Timestamp microseconds `json:"ts"`             // From process start.
	Type      eventType    `json:"ph"`             // Required. The event type. This is a single character which changes depending on the type of event being output.
	Category  string       `json:"cat,omitempty"`  // Optional. The event categories. This is a comma separated list of categories for the event. The categories can be used to hide events in the Trace Viewer UI.
	Name      string       `json:"name,omitempty"` // Optional. The name of the event, as displayed in Trace Viewer.
	Args      Args         `json:"args,omitempty"` // Optional. Cannot be used with Object. Any arguments provided for the event. Some of the event types have required argument fields, otherwise, you can put any information you wish in here. The arguments are displayed in Trace Viewer when you view an event in the analysis section.
	Duration  microseconds `json:"dur,omitempty"`  // Optional. Only for Complete.
	Scope     Scope        `json:"s,omitempty"`    // Optional. Only for Instant. Defaults to ScopeThread.
	ID        int          `json:"id,omitempty"`   // Optional. Only for Async or Object. The ID is not unique, it is meant to group multiple events together.
	/* TODO(maruel): Add these if ever used, commented out for performance.
	StackID         int          `json:"sf,omitempty"`     // Optional. Stack ID found in stackFrames section.
	Stack           []string     `json:"stack,omitempty"`  // Optional. Raw stack.
	EndStackID      int          `json:"esf,omitempty"`    // Optional. Only for Complete for end stack. Stack ID found in stackFrames section.
	EndStack        []string     `json:"estack,omitempty"` // Optional. Only for Complete for end stack. Raw stack.
	ThreadTimestamp microseconds `json:"tts,omitempty"`    // Undocumented.
	ThreadDuration  microseconds `json:"tdur,omitempty"`   // Undocumented.
	*/
}

// stackFrame is used in 'stackFrames' section.
// TODO(maruel): Use it.
type stackFrame struct {
	Parent   int    `json:"parent,omitempty"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// microseconds is used to convert from time.Duration.
type microseconds float64

// fromDuration converts a time in nanosecond to time in µs.
func fromDuration(t time.Duration) microseconds {
	return microseconds(float64(t) / float64(time.Microsecond))
}

// context embeds a pseudo thread id for this context. It's useful to keep
// context, as runtime doesn't expose the goroutine id.
type context struct {
	pid      int
	lock     sync.Mutex
	counters map[string]float64
}

// getContext returns a context if tracing is enabled.
//
// If the marker is unknown, the default {1, 1} context is returned.
// It is implicitly locking.
func getContext(marker interface{}) *context {
	globals.lockContexts.Lock()
	defer globals.lockContexts.Unlock()
	if globals.contexts == nil {
		return nil
	}
	c, ok := globals.contexts[marker]
	if !ok {
		return &consts.defaultContext
	}
	return c
}

// emit asynchronously emits a trace event.
func (c *context) emit(e *event) {
	if e.Timestamp == 0 {
		e.Timestamp = fromDuration(time.Since(consts.start))
	}
	// sync.WaitGroup.Add() use atomic.AddUint64().
	globals.wg.Add(1)
	go func() {
		defer globals.wg.Done()
		e.Pid = c.pid
		e.Tid = 1
		// Locking is done in a goroutine to not create implicit synchronization
		// when tracing.
		globals.lockWriter.Lock()
		defer globals.lockWriter.Unlock()
		if globals.out != nil {
			if globals.first {
				globals.first = false
			} else {
				// Writing is done in a goroutine to reduce cost.
				if _, err := globals.out.Write([]byte(",")); err != nil {
					log.Printf("failed writing to trace: %s", err)
					go Stop()
					return
				}
			}
			if err := globals.encoder.Encode(e); err != nil {
				log.Printf("failed writing to trace: %s", err)
				go Stop()
			}
		}
	}()
}

// metadata registers metadata in the trace. For example putting a name on the
// current pseudo process id or pseudo thread id.
func (c *context) metadata(m metadataType, args Args) {
	c.emit(&event{Type: eventMetadata, Name: string(m), Args: args})
}

func getID() int {
	globals.lockID.Lock()
	defer globals.lockID.Unlock()
	globals.nextID++
	return globals.nextID
}

func dummy(Args) {
}
