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

type TraceSetBuilder struct {
	wg           sync.WaitGroup
	traceSet     types.TraceSet
	hashStageCh  chan *hashStageRequest
	mergeStageCh []chan *mergeStageRequest // There are NUM_WORKERS of these.
	traceSets    []types.TraceSet
}

func New() *TraceSetBuilder {
	// Build a pool of 64 hash workers.
}

// traces are key'd by encoded key and the traces are just a singe tile length.
func (t *TraceSetBuilder) Add(ops paramtools.OrderedParamSet, traceMap map[int32]int32, traces map[string][]float32) {
	// for each trace we wg.Add()
}

// Don't call Build until Add() has been called for every tile to be added.
func (t *TraceSetBuilder) Build() types.TraceSet {
	close(t.hashStageCh)
	t.wg.Wait()
}
