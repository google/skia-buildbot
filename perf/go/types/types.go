package types

import "go.skia.org/infra/go/vec32"

// Trace is just a slice of float32s.
type Trace []float32

// NewTrace returns a Trace of length 'traceLen' initialized to vec32.MISSING_DATA_SENTINEL.
func NewTrace(traceLen int) Trace {
	return Trace(vec32.New(traceLen))
}

// TraceSet is a set of Trace's, keyed by trace id.
type TraceSet map[string]Trace

// Progress is a func that is called to update the progress on a computation.
type Progress func(step, totalSteps int)
