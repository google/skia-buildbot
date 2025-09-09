package preflightqueryprocessor

import (
	"sync"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
)

// A type that manages the resulting ParamSet, that's shared among all goroutines,
// both evaluating the main query and subqueries.
type ParamSetAggregator interface {
	// Append to the shared paramset params that are corresponding to traceIDs for this query.
	AddParams(ps paramtools.Params)
	// Getter for the query.
	GetQuery() *query.Query
	// paramtools.Params and "TraceId" are used interchangibly.
	// TODO(mordeckimarcin) resolve the confusion.
	// Collects matching (traceId, Params) pairs for the query.
	ProcessTraceIds(<-chan paramtools.Params) []paramtools.Params
	// Set values for given key with the reference paramset's values.ś
	SetReferenceParamKey(key string, referenceParamSet paramtools.ReadOnlyParamSet)
	// After all tiles for this query have been processed, we might need to do some
	// further evaluation of collected Params, for instance collect all values for
	// the given key while evaluating a subquery.
	Finalize()
}

// A type that calculates the number of traces matching the main query,
// and collects both this number and the shared paramset after all query evaluations are done.
type PreflightQueryResultCollector interface {
	// Calculate number of matching traces.
	UpdateCount(int)
	GetParamSet() *paramtools.ParamSet
	GetCount() int
}

// Type that manages subquery evaluation.
// Again, we reuse the mutex from base struct to protect filteredValues map.
// Implements just ParamSetAggregator.
type preflightSubQueryProcessor struct {
	preflightQueryBaseProcessor
	// Key that has been removed from the main query.
	key string
	// All values collected across tiles for the key.
	filteredValuesFromTiles map[string]bool
}

func NewPreflightMainQueryProcessor(q *query.Query) *preflightMainQueryProcessor {
	var sharedCount int
	return &preflightMainQueryProcessor{
		preflightQueryBaseProcessor{q, &sync.Mutex{}, &paramtools.ParamSet{}},
		&sharedCount,
	}
}

func NewPreflightSubQueryProcessor(o *preflightMainQueryProcessor, subQuery *query.Query, key string) *preflightSubQueryProcessor {
	return &preflightSubQueryProcessor{
		preflightQueryBaseProcessor{subQuery, o.sharedMux, o.sharedParamSet},
		key,
		map[string]bool{},
	}
}

// Main Query Processor implements both interfaces.
var _ ParamSetAggregator = (*preflightMainQueryProcessor)(nil)
var _ PreflightQueryResultCollector = (*preflightMainQueryProcessor)(nil)

// Subquery processor implements just the ParamSetAggregator.
var _ ParamSetAggregator = (*preflightSubQueryProcessor)(nil)
