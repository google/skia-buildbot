package preflightqueryprocessor

import "go.skia.org/infra/go/paramtools"

// Type that manages count of traces.
// Note that it uses the sharedMux from the Base struct.
// Implements both ParamSetAggregator and PreflightQueryResultCollector.
type preflightMainQueryProcessor struct {
	preflightQueryBaseProcessor
	sharedCount *int
}

func (p *preflightMainQueryProcessor) UpdateCount(tileOneCount int) {
	p.sharedMux.Lock()
	defer p.sharedMux.Unlock()

	// TODO(mordeckimarcin) To be honest, I'm not sure why are we selecting max.
	// Isn't it possible that the latest tile contains a trace X and the previous tile
	// contains a trace Y, both being matched by the query?
	if tileOneCount > *p.sharedCount {
		*p.sharedCount = tileOneCount
	}
}

func (p *preflightMainQueryProcessor) GetParamSet() *paramtools.ParamSet {
	p.sharedMux.Lock()
	defer p.sharedMux.Unlock()
	return p.sharedParamSet
}

func (p *preflightMainQueryProcessor) GetCount() int {
	p.sharedMux.Lock()
	defer p.sharedMux.Unlock()
	return *p.sharedCount
}

// For the main query, we just add collected params to our resulting paramset and count them.
func (p *preflightMainQueryProcessor) ProcessTraceIds(out <-chan paramtools.Params) []paramtools.Params {
	traceIdsForTile := []paramtools.Params{}
	count := 0
	for outParam := range out {
		count++
		p.AddParams(outParam)
		traceIdsForTile = append(traceIdsForTile, outParam)
	}

	p.UpdateCount(count)
	return traceIdsForTile
}

// Main query does not require any extra work after all tiles are queried.
func (p *preflightMainQueryProcessor) Finalize() {
}
