package search

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

type ExpSlice []*expstorage.Expectations

func (e ExpSlice) Classification(test, digest string) types.Label {
	for _, exp := range e {
		if label, ok := exp.Tests[test][digest]; ok {
			return label
		}
	}
	return types.UNTRIAGED
}

// srIntermediate is the intermediate representation of a single digest
// found by the search. It is used to avoid multiple passes through the tile
// by accumulating the parameters that generated a specific digest and by
// capturing the traces.
type srIntermediate struct {
	test   string
	digest string
	traces map[string]*types.GoldenTrace
	params paramtools.ParamSet
}

// newSrIntermediate creates a new srIntermediate for a digest and adds
// the given trace to it.
func newSrIntermediate(test, digest, traceID string, trace tiling.Trace) *srIntermediate {
	ret := &srIntermediate{
		test:   test,
		digest: digest,
		params: paramtools.ParamSet{},
		traces: map[string]*types.GoldenTrace{},
	}
	ret.add(traceID, trace, nil)
	return ret
}

// Add adds a new trace to an existing intermediate value for a digest
// found in search. If traceID or trace are "" or nil they will not be added.
// 'params' will always be added to the internal parameter set.
func (s *srIntermediate) add(traceID string, trace tiling.Trace, params paramtools.ParamSet) {
	if (traceID != "") && (trace != nil) {
		s.traces[traceID] = trace.(*types.GoldenTrace)
		s.params.AddParams(trace.Params())
	} else {
		s.params.AddParamSet(params)
	}
}

// srInterMap maps [testName][Digest] to an srIntermediate instance that
// aggregates values during a search.
type srInterMap map[string]map[string]*srIntermediate

// add adds the given information to the srInterMap instance.
func (sm srInterMap) add(test, digest, traceID string, trace *types.GoldenTrace, params paramtools.ParamSet) {
	if testMap, ok := sm[test]; !ok {
		sm[test] = map[string]*srIntermediate{digest: newSrIntermediate(test, digest, traceID, trace)}
	} else if entry, ok := testMap[digest]; !ok {
		testMap[digest] = newSrIntermediate(test, digest, traceID, trace)
	} else {
		entry.add(traceID, trace, params)
	}
}
