package tracesetbuilder

import (
	"crypto/md5"
	"encoding/binary"
	"sync"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/types"
)

const NUM_WORKERS = 64

type request struct {
	key      string // In the first stage this is the encoded key, in the second stage it is the decoded key.
	params   paramtools.Params
	trace    []float32
	ops      paramtools.OrderedParamSet
	traceMap map[int32]int32
}

type mergeWorker struct {
	ch       chan *request
	wg       *sync.WaitGroup
	traceSet types.TraceSet
	paramSet paramtools.ParamSet
}

func newMergeWorker(wg *sync.WaitGroup, size int) *mergeWorker {
	m := &mergeWorker{
		ch:       make(chan *request),
		wg:       wg,
		traceSet: types.TraceSet{},
		paramSet: paramtools.ParamSet{},
	}
	go func() {
		for req := range m.ch {
			// Inlay the trace into traceSet and paramSet.
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

type TraceSetBuilder struct {
	wg           *sync.WaitGroup
	hashStageCh  chan *request
	mergeWorkers []*mergeWorker // There are NUM_WORKERS of these.
}

// size == len(indices)
func New(size int) *TraceSetBuilder {
	t := &TraceSetBuilder{
		wg:           &sync.WaitGroup{},
		hashStageCh:  make(chan *request),
		mergeWorkers: []*mergeWorker{},
	}
	// Build a pool of 64 merge workers.

	for i := 0; i < NUM_WORKERS; i++ {
		t.mergeWorkers = append(t.mergeWorkers, newMergeWorker(t.wg, size))
	}

	// Build a pool of 64 hash workers.
	for i := 0; i < NUM_WORKERS; i++ {
		go func() {
			for req := range t.hashStageCh {
				p, err := req.ops.DecodeParamsFromString(req.key)
				if err != nil {
					// It is possible we matched a trace that appeared after we grabbed the OPS,
					// so just ignore it.
					t.wg.Done()
					continue
				}
				req.params = p
				key, err := query.MakeKey(p)
				if err != nil {
					sklog.Warningf("Failed to make a key from %#v: %s", p, err)
					t.wg.Done()
					continue
				}
				req.key = key
				index := binary.BigEndian.Uint64(md5.New().Sum([]byte(key))) % NUM_WORKERS
				t.mergeWorkers[index].Process(req)
			}
		}()
	}
	return t
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
	return nil, nil
}
