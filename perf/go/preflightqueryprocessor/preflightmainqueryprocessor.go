package preflightqueryprocessor

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
)

// Type that manages count of traces.
// Note that it uses the sharedMux from the Base struct.
// Implements both ParamSetAggregator and PreflightQueryResultCollector.
type preflightMainQueryProcessor struct {
	preflightQueryBaseProcessor
	sharedCount         *int
	keysToDetectMissing []string
	uniqueTraceIDs      map[string]bool
	filterMap           map[string]map[string]bool
}

func (p *preflightMainQueryProcessor) SetKeysToDetectMissing(keys []string) {
	p.keysToDetectMissing = keys
}

func (p *preflightMainQueryProcessor) UpdateCount(tileOneCount int) {
	// No-op, count is tracked via uniqueTraceIDs
}

func (p *preflightMainQueryProcessor) GetParamSet() *paramtools.ParamSet {
	p.sharedMux.Lock()
	defer p.sharedMux.Unlock()
	return p.sharedParamSet
}

func (p *preflightMainQueryProcessor) GetCount() int {
	p.sharedMux.Lock()
	defer p.sharedMux.Unlock()
	return len(p.uniqueTraceIDs)
}

// For the main query, we just add collected params to our resulting paramset and count them.
func (p *preflightMainQueryProcessor) ProcessTraceIds(out <-chan paramtools.Params) []paramtools.Params {
	traceIdsForTile := []paramtools.Params{}
	for outParam := range out {
		// Filter traces based on the sentinel logic.
		if !FilterParams(outParam, p.filterMap) {
			continue
		}

		p.AddParams(outParam)

		if len(p.keysToDetectMissing) > 0 {
			missingParams := paramtools.Params{}
			for _, key := range p.keysToDetectMissing {
				if _, ok := outParam[key]; !ok {
					missingParams[key] = ""
				}
			}
			if len(missingParams) > 0 {
				p.AddParams(missingParams)
			}
		}

		key, _ := query.MakeKey(outParam)
		p.sharedMux.Lock()
		p.uniqueTraceIDs[key] = true
		p.sharedMux.Unlock()

		traceIdsForTile = append(traceIdsForTile, outParam)
	}

	return traceIdsForTile
}

// Main query does not require any extra work after all tiles are queried.
func (p *preflightMainQueryProcessor) Finalize() {
}
