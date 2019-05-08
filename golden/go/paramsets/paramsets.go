// paramsets keeps a running summary of paramsets per test, digest pair.
package paramsets

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

// ParamSummary keep precalculated paramsets for each test, digest pair.
// It is not thread safe. The client of this package needs to make sure there
// are no conflicts.
type ParamSummary struct {
	byTest               map[types.TestName]map[types.Digest]paramtools.ParamSet
	byTestIncludeIgnored map[types.TestName]map[types.Digest]paramtools.ParamSet
}

// byTestForTile calculates all the paramsets from the given tile and tallies.
func byTestForTile(tile *tiling.Tile, digestCountsByTrace map[tiling.TraceId]digest_counter.DigestCount) map[types.TestName]map[types.Digest]paramtools.ParamSet {
	ret := map[types.TestName]map[types.Digest]paramtools.ParamSet{}

	for id, dc := range digestCountsByTrace {
		if tr, ok := tile.Traces[id]; ok {
			test := types.TestName(tr.Params()[types.PRIMARY_KEY_FIELD])
			for digest := range dc {
				if foundTest, ok := ret[test]; !ok {
					ret[test] = map[types.Digest]paramtools.ParamSet{
						digest: paramtools.NewParamSet(tr.Params()),
					}
				} else if foundDigest, ok := foundTest[digest]; !ok {
					foundTest[digest] = paramtools.NewParamSet(tr.Params())
				} else {
					foundDigest.AddParams(tr.Params())
				}
			}
		}
	}
	return ret
}

// New creates a new ParamSummary.
func New() *ParamSummary {
	return &ParamSummary{}
}

// Calculate sets the values the ParamSummary based on the given tile.
func (s *ParamSummary) Calculate(cpxTile types.ComplexTile, dCounter digest_counter.DigestCounter, dCounterWithIgnores digest_counter.DigestCounter) {
	defer shared.NewMetricsTimer("param_summary_calculate").Stop()
	s.byTest = byTestForTile(cpxTile.GetTile(false), dCounter.ByTrace())
	s.byTestIncludeIgnored = byTestForTile(cpxTile.GetTile(true), dCounterWithIgnores.ByTrace())
}

// Get returns the paramset for the given digest. If 'include' is true
// then the paramset is calculated including ignored traces.
func (s *ParamSummary) Get(test types.TestName, digest types.Digest, include bool) paramtools.ParamSet {
	useMap := s.byTest
	if include {
		useMap = s.byTestIncludeIgnored
	}

	if foundTest, ok := useMap[test]; ok {
		return foundTest[digest]
	}
	return nil
}

// GetByTest returns the parameter sets organized by tests and digests:
//      map[test_name]map[digest]ParamSet
func (s *ParamSummary) GetByTest(includeIngores bool) map[types.TestName]map[types.Digest]paramtools.ParamSet {
	if includeIngores {
		return s.byTestIncludeIgnored
	}
	return s.byTest
}
