package tracesetbuilder

import (
	"context"
	"hash/crc32"
	"sync"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/perf/go/types"
)

const (
	numWorkers        = 64
	channelBufferSize = 10000
)

// request is what flows through the TraceSetBuilder pipeline.
//
// See New() for more details.
type request struct {
	key      string
	params   paramtools.Params
	trace    []float32
	traceMap map[int32]int32
}

// mergeWorker merges the data in requests into a traceSet and paramSet.
type mergeWorker struct {
	ch       chan *request
	wg       *sync.WaitGroup // A pointer to the single sync.WaitGroup that TraceSetBuilder holds.
	traceSet types.TraceSet
	paramSet paramtools.ParamSet
}

// newMergeWorker creates a mergeWorker and starts its go routine.
func newMergeWorker(wg *sync.WaitGroup, size int) *mergeWorker {
	m := &mergeWorker{
		ch:       make(chan *request, channelBufferSize),
		wg:       wg,
		traceSet: types.TraceSet{},
		paramSet: paramtools.ParamSet{},
	}
	go func() {
		for req := range m.ch {
			trace, ok := m.traceSet[req.key]
			if !ok {
				trace = types.NewTrace(size)
			}
			for srcIndex, dstIndex := range req.traceMap {
				trace[dstIndex] = req.trace[srcIndex]
			}
			m.traceSet[req.key] = trace
			m.paramSet.AddParams(req.params)
			m.wg.Done()
		}
	}()

	return m
}

func (m *mergeWorker) Process(req *request) {
	m.ch <- req
}

func (m *mergeWorker) Close() {
	close(m.ch)
}

// TraceSetBuilder builds a TraceSet from traces found in Tiles.
//
//  The mergeWorkers are selected based on the md5 hash of the decoded key for
//  a trace, this ensures that the same trace id will always be processed by
//  the same mergeWorker. This way we ensure that each mergeWorker sees only
//  their subset of the traces and we can avoid mutexes.
//
// The Build() func will consolidate all the work of the mergeWorkers
// and shut down the worker pools. Because of that a TraceSetBuilder
// cannot be reused.
type TraceSetBuilder struct {
	wg           *sync.WaitGroup
	mergeWorkers []*mergeWorker // There are NUM_WORKERS of these.
}

// New creates a new TraceSetBuilder and starts the worker pools.
//
// size is the length of the traces in the final TraceSet.
//
// The caller should call Close() on the returned builder once they are done
// with it to close down all the workers.
func New(size int) *TraceSetBuilder {
	t := &TraceSetBuilder{
		wg:           &sync.WaitGroup{},
		mergeWorkers: []*mergeWorker{},
	}

	// Build a pool of merge workers.
	for i := 0; i < numWorkers; i++ {
		t.mergeWorkers = append(t.mergeWorkers, newMergeWorker(t.wg, size))
	}

	return t
}

// Add traces to the TraceSet.
//
// traceMap says where each trace value should be placed in the final trace.
// traces are keyed by traceId (unencoded) and the traces are just a single tile length.
func (t *TraceSetBuilder) Add(traceMap map[int32]int32, traces types.TraceSet) {
	defer timer.New("TraceSetBuilder.Add").Stop()
	t.wg.Add(len(traces))
	for key, trace := range traces {
		params, err := query.ParseKey(key)
		if err != nil {
			sklog.Warningf("Found invalid key %q: %s", key, err)
			continue
		}
		req := &request{
			key:      key,
			trace:    trace,
			traceMap: traceMap,
			params:   params,
		}
		index := crc32.ChecksumIEEE([]byte(req.key)) % numWorkers
		t.mergeWorkers[index].Process(req)
	}
}

// Build returns the built TraceSet and ParamSet for that TraceSet.
//
// Don't call Build until Add() has been called for every tile to be added.
func (t *TraceSetBuilder) Build(ctx context.Context) (types.TraceSet, paramtools.ReadOnlyParamSet) {
	ctx, span := trace.StartSpan(ctx, "TraceSetBuilder.Build")
	defer span.End()

	defer timer.New("TraceSetBuilder.Build").Stop()
	t.wg.Wait()
	defer timer.New("TraceSetBuilder.Build-after-wait").Stop()

	traceSet := types.TraceSet{}
	paramSet := paramtools.ParamSet{}
	// Now merge all the traceSets and paramSets.
	for _, mw := range t.mergeWorkers {
		for k, v := range mw.traceSet {
			traceSet[k] = v
		}
		paramSet.AddParamSet(mw.paramSet)
	}
	paramSet.Normalize()
	return traceSet, paramSet.Freeze()
}

// Close down all the workers.
//
// Always call this to clean up the workers.
func (t *TraceSetBuilder) Close() {
	for _, mw := range t.mergeWorkers {
		mw.Close()
	}
}
