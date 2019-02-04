package tracesetbuilder

import (
	"sync"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/types"
)

const NUM_WORKERS = 64

type hashStageRequest struct {
	encodedKey string
	trace      []int32
	ops        paramtools.OrderedParamSet
	traceMap   map[int32]int32
}

type mergeStageRequest struct {
	key      string
	trace    []int32
	ops      paramtools.OrderedParamSet
	traceMap map[int32]int32
}

type mergeWorker struct {
	ch       chan *mergeStageRequest
	wg       *sync.WaitGroup
	traceSet types.TraceSet
	paramSet paramtools.ParamSet
}

func newMergeWorker(wg *sync.WaitGroup) *mergeWorker {
	m := mergeWorker{
		ch:       make(chan *mergeStageRequest),
		wg:       wg,
		traceSet: types.TraceSet{},
		paramSet: paramtools.ParamSet{},
	}
	go func() {
		for req := range m.ch {
			// Inlay the trace into traceSet and paramSet.
			m.wg.Done()
		}
	}()

	return m
}

func (m *mergeWorker) Process(req *mergeStageRequest) {
	m.ch <- req
}

func (m *mergeWorker) Close() {
	close(m.ch)
}

type TraceSetBuilder struct {
	wg           *sync.WaitGroup
	hashStageCh  chan *hashStageRequest
	mergeWorkers []*mergeWorker // There are NUM_WORKERS of these.
}

func New() *TraceSetBuilder {
	t := &TraceSetBuilder{
		wg:           &sync.WaitGroup{},
		hashStageCh:  make(chan *hashStageRequest),
		mergeWorkers: []*mergeWorker{},
	}
	// Build a pool of 64 merge workers.

	for i := 0; i < NUM_WORKERS; i++ {
		t.mergeWorkers = append(t.mergeWorkers, newMergeWorker(t.wg))
	}

	// Build a pool of 64 hash workers.
	for i := 0; i < NUM_WORKERS; i++ {
		go func() {
			for req := range t.hashStageCh {
				// Decode key
				// Hash key
				// Use has to direct work to a mergeWorker.
				index := 2
				t.mergeWorkers[index].Process(req)
			}
		}()
	}
}

// traces are key'd by encoded key and the traces are just a singe tile length.
func (t *TraceSetBuilder) Add(ops paramtools.OrderedParamSet, traceMap map[int32]int32, traces map[string][]float32) {
	t.wg.Add(len(traces))
}

// Don't call Build until Add() has been called for every tile to be added.
func (t *TraceSetBuilder) Build() (types.TraceSet, paramtools.ParamSet) {
	close(t.hashStageCh)
	t.wg.Wait()
	// Now merge all the traceSets and paramSets.
}
