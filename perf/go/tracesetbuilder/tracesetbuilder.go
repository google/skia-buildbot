package tracesetbuilder

import (
	"hash/crc32"
	"sync"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/perf/go/types"
)

const NUM_WORKERS = 64
const CHANNEL_BUFFER_SIZE = 10000

// request is what flows through the TraceSetBuilder pipeline.
//
// See New() for more details.
type request struct {
	key      string // In the first stage this is the encoded key, in the second stage it is the decoded key.
	params   paramtools.Params
	trace    []float32
	ops      *paramtools.OrderedParamSet
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
		ch:       make(chan *request, CHANNEL_BUFFER_SIZE),
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
// The process is a two stage pipeline, with the first stage key
// decoding, and the second stage the actual merging of the trace data
// into a TraceSet. The second stage of the pipeline, the mergeWorkers,
// are selected based on the md5 hash of the decoded key for a trace,
// this ensures that the same trace id will always be processed by the
// same mergeWorker. This way we ensure that each mergeWorker sees only
// their subset of the traces and we can avoid mutexes.
//
//           +->  hashWorker -+   mergeWorker
//           |                |
//  request -+    hashWorker  |   mergeWorker
//                            |
//                hashWorker  +-> mergeWorker
//
// The Build() func will consolidate all the work of the mergeWorkers
// and shut down the worker pools. Because of that a TraceSetBuilder
// cannot be reused.
type TraceSetBuilder struct {
	wg           *sync.WaitGroup
	hashStageCh  chan *request
	mergeWorkers []*mergeWorker // There are NUM_WORKERS of these.
}

// New creates a new TraceSetBuilder and starts the worker pools.
//
// size is the length of the traces in the final TraceSet.
func New(size int) *TraceSetBuilder {
	t := &TraceSetBuilder{
		wg:           &sync.WaitGroup{},
		hashStageCh:  make(chan *request, CHANNEL_BUFFER_SIZE),
		mergeWorkers: []*mergeWorker{},
	}

	// Build a pool of merge workers.
	for i := 0; i < NUM_WORKERS; i++ {
		t.mergeWorkers = append(t.mergeWorkers, newMergeWorker(t.wg, size))
	}

	// Build a pool of hash workers.
	for i := 0; i < NUM_WORKERS; i++ {
		go func() {
			var err error
			for req := range t.hashStageCh {
				req.params, err = req.ops.DecodeParamsFromString(req.key)
				if err != nil {
					// It is possible we matched a trace that appeared after we grabbed the OPS,
					// so just ignore it.
					t.wg.Done()
					continue
				}
				req.key, err = query.MakeKeyFast(req.params)
				if err != nil {
					sklog.Errorf("Failed to make a key from %#v: %s", req.params, err)
					t.wg.Done()
					continue
				}
				// Calculate which merge worker should handle this request.
				index := crc32.ChecksumIEEE([]byte(req.key)) % NUM_WORKERS
				t.mergeWorkers[index].Process(req)
			}
		}()
	}
	return t
}

// Add traces to the TraceSet.
//
// ops is the OrderedParamSet for the tile that the traces came from.
// traceMap says where each trace value should be placed in the final trace.
// traces are keyed by encoded key and the traces are just a single tile length.
func (t *TraceSetBuilder) Add(ops *paramtools.OrderedParamSet, traceMap map[int32]int32, traces map[string][]float32) {
	defer timer.New("TraceSetBuilder.Add").Stop()
	t.wg.Add(len(traces))
	for encodedKey, trace := range traces {
		req := &request{
			key:      encodedKey,
			trace:    trace,
			ops:      ops,
			traceMap: traceMap,
		}
		t.hashStageCh <- req
	}
}

// Build returns the built TraceSet and ParamSet for that TraceSet.
//
// Build will also shut down the worker pools.
//
// Don't call Build until Add() has been called for every tile to be added.
func (t *TraceSetBuilder) Build() (types.TraceSet, paramtools.ParamSet) {
	defer timer.New("TraceSetBuilder.Build").Stop()
	t.wg.Wait()
	defer timer.New("TraceSetBuilder.Build-after-wait").Stop()
	// Shut down the first stage.
	close(t.hashStageCh)

	traceSet := types.TraceSet{}
	paramSet := paramtools.ParamSet{}
	// Now merge all the traceSets and paramSets.
	for _, mw := range t.mergeWorkers {
		for k, v := range mw.traceSet {
			traceSet[k] = v
		}
		paramSet.AddParamSet(mw.paramSet)
		// Shut down the worker.
		mw.Close()
	}
	return traceSet, paramSet
}
