package preflightqueryprocessor

import "go.skia.org/infra/go/paramtools"

// In subquery, we collect params matching the query and cache it afterwards.
// The shared paramset object, however, is populated only by values for the key
// that's related to the subquery.
// Also, we don't count the number of subquery matching traces.
// For each tile, we collect values in a map, and populate the paramset for this key
// only after all tiles are processed.
// Checking a map is faster than iterating over a ParamSet trying to
// assess whether a value is present, so this should save us quite some time.
func (p *preflightSubQueryProcessor) ProcessTraceIds(out <-chan paramtools.Params) []paramtools.Params {
	key := p.key
	traceIdsForTile := []paramtools.Params{}

	// First, collects values for this tile. Afterwards, populate the shared map in one go.
	// This reduces the number of mutex lock/unlocks, which should save some time.
	valsForTile := []string{}
	for outParam := range out {
		if !FilterParams(outParam, p.filterMap) {
			continue
		}
		traceIdsForTile = append(traceIdsForTile, outParam)
		if val, ok := outParam[key]; ok {
			valsForTile = append(valsForTile, val)
		}
	}

	// All values are collected, populate the shared map under one mutex lock.
	p.sharedMux.Lock()
	defer p.sharedMux.Unlock()
	for _, v := range valsForTile {
		p.filteredValuesFromTiles[v] = true
	}

	return traceIdsForTile
}

// After querying all tiles for a subquery, we should populate the paramset
// with values for the key related to the subquery.
func (p *preflightSubQueryProcessor) Finalize() {
	key := p.key
	for v := range p.filteredValuesFromTiles {
		p.AddParams(paramtools.Params{key: v})
	}
}
