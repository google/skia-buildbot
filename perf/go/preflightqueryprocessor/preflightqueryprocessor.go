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
	// Set keys that should be checked for presence. If a key is missing in a trace,
	// an empty string value will be recorded for it.
	SetKeysToDetectMissing(keys []string)
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
	// Map of keys to allowed values for filtering (handling sentinel).
	filterMap map[string]map[string]bool
}

func (p *preflightSubQueryProcessor) SetKeysToDetectMissing(keys []string) {
	// No-op for subqueries as we rely on the main query processor for missing key detection
	// on nextParamList.
}

// Helpers for handling MissingValueSentinel logic.

const MissingValueSentinel = "__missing__"

// PrepareQueryWithSentinel analyzes the query for keys that contain the MissingValueSentinel.
// If such a sentinel is found for a key, it means the user wants to select traces that either
// match specific values for that key OR are missing that key entirely (parent traces).
//
// Since the backend store typically doesn't support querying for "missing keys" directly alongside
// specific values in a single query (it usually does exact match filtering), we adopt a strategy
// of fetching a superset of traces and filtering them in memory.
//
// This function:
//  1. Identifies keys with the sentinel.
//  2. Removes those keys from the query object (modifying it in place or creating a new list of params).
//     Removing the key effectively tells the store to fetch all traces regardless of that key's value (or lack thereof).
//  3. Returns a filterMap which maps the key to the set of allowed values (excluding the sentinel).
//     This map is then used by FilterParams to verify if a fetched trace matches the criteria:
//     - It is missing the key (matches sentinel).
//     - OR it has the key and the value is in the allowed set.
func PrepareQueryWithSentinel(q *query.Query) map[string]map[string]bool {
	filterMap := map[string]map[string]bool{}
	if q != nil {
		newParams := []query.QueryParam{}
		for i := range q.Params {
			p := &q.Params[i]
			hasSentinel := false
			newValues := []string{}
			validValues := map[string]bool{}
			for _, v := range p.Values {
				if v == MissingValueSentinel {
					hasSentinel = true
				} else {
					newValues = append(newValues, v)
					validValues[v] = true
				}
			}

			if hasSentinel {
				key := p.Key()
				filterMap[key] = validValues
				// Always remove this param from query to fetch superset.
				// This ensures we get traces that are missing the key, as well as traces
				// that have the key with allowed values.
				continue
			}
			newParams = append(newParams, *p)
		}
		q.Params = newParams
	}
	return filterMap
}

func FilterParams(outParam paramtools.Params, filterMap map[string]map[string]bool) bool {
	if len(filterMap) == 0 {
		return true
	}
	for k, allowedValues := range filterMap {
		val, ok := outParam[k]
		if !ok || val == "" {
			// Key is missing or empty. This matches the sentinel.
			continue
		}
		// Key is present.
		if len(allowedValues) > 0 && allowedValues[val] {
			// Value is explicitly allowed.
			continue
		}
		return false
	}
	return true
}

func NewPreflightMainQueryProcessor(q *query.Query) *preflightMainQueryProcessor {
	var sharedCount int
	// Clone q to avoid modifying the caller's query object, as it might be used
	// to construct subqueries.
	var processQ *query.Query
	if q != nil {
		processQ = &query.Query{
			Params: make([]query.QueryParam, len(q.Params)),
		}
		copy(processQ.Params, q.Params)
	}

	filterMap := PrepareQueryWithSentinel(processQ)

	return &preflightMainQueryProcessor{
		preflightQueryBaseProcessor{processQ, &sync.Mutex{}, &paramtools.ParamSet{}},
		&sharedCount,
		[]string{},
		map[string]bool{},
		filterMap,
	}
}

func NewPreflightSubQueryProcessor(o *preflightMainQueryProcessor, subQuery *query.Query, key string) *preflightSubQueryProcessor {
	// Clone subQuery as well.
	var processQ *query.Query
	if subQuery != nil {
		processQ = &query.Query{
			Params: make([]query.QueryParam, len(subQuery.Params)),
		}
		copy(processQ.Params, subQuery.Params)
	}

	filterMap := PrepareQueryWithSentinel(processQ)
	return &preflightSubQueryProcessor{
		preflightQueryBaseProcessor{processQ, o.sharedMux, o.sharedParamSet},
		key,
		map[string]bool{},
		filterMap,
	}
}

// Main Query Processor implements both interfaces.
var _ ParamSetAggregator = (*preflightMainQueryProcessor)(nil)
var _ PreflightQueryResultCollector = (*preflightMainQueryProcessor)(nil)

// Subquery processor implements just the ParamSetAggregator.
var _ ParamSetAggregator = (*preflightSubQueryProcessor)(nil)
