package search

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

type ExpSlice []types.Expectations

func (e ExpSlice) Classification(test types.TestName, digest types.Digest) types.Label {
	for _, exp := range e {
		if label := exp.Classification(test, digest); label != types.UNTRIAGED {
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
	test   types.TestName
	digest types.Digest
	traces map[tiling.TraceId]*types.GoldenTrace
	params paramtools.ParamSet
}

// newSrIntermediate creates a new srIntermediate for a digest and adds
// the given trace to it.
func newSrIntermediate(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace tiling.Trace, pset paramtools.ParamSet) *srIntermediate {
	ret := &srIntermediate{
		test:   test,
		digest: digest,
		params: paramtools.ParamSet{},
		traces: map[tiling.TraceId]*types.GoldenTrace{},
	}
	ret.add(traceID, trace, pset)
	return ret
}

// Add adds a new trace to an existing intermediate value for a digest
// found in search. If traceID or trace are "" or nil they will not be added.
// 'params' will always be added to the internal parameter set.
func (s *srIntermediate) add(traceID tiling.TraceId, trace tiling.Trace, pset paramtools.ParamSet) {
	if (traceID != "") && (trace != nil) {
		s.traces[traceID] = trace.(*types.GoldenTrace)
		s.params.AddParams(trace.Params())
	} else {
		s.params.AddParamSet(pset)
	}
}

// srInterMap maps [testName][Digest] to an srIntermediate instance that
// aggregates values during a search.
type srInterMap map[types.TestName]map[types.Digest]*srIntermediate

// Add adds the paramset associated with the given test and digest to the srInterMap instance.
func (sm srInterMap) Add(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace *types.GoldenTrace, pset paramtools.ParamSet) {
	if testMap, ok := sm[test]; !ok {
		sm[test] = map[types.Digest]*srIntermediate{digest: newSrIntermediate(test, digest, traceID, trace, pset)}
	} else if entry, ok := testMap[digest]; !ok {
		testMap[digest] = newSrIntermediate(test, digest, traceID, trace, pset)
	} else {
		entry.add(traceID, trace, pset)
	}
}

// AddTestParams adds the params associated with the given test and digest to the srInterMap instance.
func (sm srInterMap) AddTestParams(test types.TestName, digest types.Digest, params paramtools.Params) {
	if testMap, ok := sm[test]; !ok {
		ns := &srIntermediate{
			test:   test,
			digest: digest,
			params: paramtools.ParamSet{},
			traces: map[tiling.TraceId]*types.GoldenTrace{},
		}
		ns.params.AddParams(params)
		sm[test] = map[types.Digest]*srIntermediate{
			digest: ns,
		}
	} else if entry, ok := testMap[digest]; !ok {
		ns := &srIntermediate{
			test:   test,
			digest: digest,
			params: paramtools.ParamSet{},
			traces: map[tiling.TraceId]*types.GoldenTrace{},
		}
		ns.params.AddParams(params)
		testMap[digest] = ns
	} else {
		entry.params.AddParams(params)
	}
}

// digests is mostly used for debugging and returns all digests contained in
// a collection of intermediate search results.
func (sm srInterMap) numDigests() int {
	ret := 0
	for _, digests := range sm {
		ret += len(digests)
	}
	return ret
}
