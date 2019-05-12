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
// It is considered immutable and thread-safe.
type ParamSummary interface {
	// Get returns the paramset for the given digest. If 'noIgnoreRules' is true
	// then the paramset is calculated without applying ignores.
	Get(test types.TestName, digest types.Digest, noIgnoreRules bool) paramtools.ParamSet

	// GetByTest returns the parameter sets organized by tests and digests.
	// If 'noIgnoreRules' is true, then the paramset is calculated
	// without applying ignores.
	GetByTest(noIgnoreRules bool) map[types.TestName]map[types.Digest]paramtools.ParamSet
}

// ParamSummaryImpl implements the ParamSummary interface.
type ParamSummaryImpl struct {
	byTestWithIgnoreRules    map[types.TestName]map[types.Digest]paramtools.ParamSet
	byTestWithoutIgnoreRules map[types.TestName]map[types.Digest]paramtools.ParamSet
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

// NewParamSummary creates a new ParamSummaryImpl.
func NewParamSummary(cpxTile types.ComplexTile, countsWithIgnoreRules digest_counter.DigestCounter, countsWithoutIgnoreRules digest_counter.DigestCounter) *ParamSummaryImpl {
	defer shared.NewMetricsTimer("param_summary_calculate").Stop()
	p := &ParamSummaryImpl{
		byTestWithIgnoreRules:    byTestForTile(cpxTile.GetTile(false), countsWithIgnoreRules.ByTrace()),
		byTestWithoutIgnoreRules: byTestForTile(cpxTile.GetTile(true), countsWithoutIgnoreRules.ByTrace()),
	}
	return p
}

// Get implements the ParamSummaryImpl interface.
func (s *ParamSummaryImpl) Get(test types.TestName, digest types.Digest, noIgnoreRules bool) paramtools.ParamSet {
	useMap := s.byTestWithIgnoreRules
	if noIgnoreRules {
		useMap = s.byTestWithoutIgnoreRules
	}

	if foundTest, ok := useMap[test]; ok {
		return foundTest[digest]
	}
	return nil
}

// GetByTest implements the ParamSummaryImpl interface.
func (s *ParamSummaryImpl) GetByTest(noIgnoreRules bool) map[types.TestName]map[types.Digest]paramtools.ParamSet {
	if noIgnoreRules {
		return s.byTestWithoutIgnoreRules
	}
	return s.byTestWithIgnoreRules
}
