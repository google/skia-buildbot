package btts

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/btts/engine"
)

const (
	// MAX_PARALLEL_PARAM_INDEX is the maxumum number of key=value pairs that
	// can appear in a Query Plan.
	MAX_PARALLEL_PARAM_INDEX = 200
)

// sizeOfPlan returns the number of key=value pairs in the ParamSet.
func sizeOfPlan(plan paramtools.ParamSet) int {
	count := 0
	for _, values := range plan {
		count += len(values)
	}
	return count
}

// validatePlan takes a plan (a ParamSet of OPS encoded keys and values) and
// validates that it should run to completion. This will also error if the query
// is too large, i.e. would generate too many concurrent queries to BigTable.
func validatePlan(plan paramtools.ParamSet) error {
	count := sizeOfPlan(plan)
	if count > MAX_PARALLEL_PARAM_INDEX {
		return fmt.Errorf("Plan is too large, found %d > %d key,value pairs.", count, MAX_PARALLEL_PARAM_INDEX)
	}
	return nil
}

// ExecutePlan takes a query plan and executes it over a table for the given
// TileKey. The result is a channel that will produce encoded keys in
// alphabetical order and will close after the query is done executing.
//
// An error will be returned if the query is invalid.
//
// The desc is a descriptive string to add to any error logs this function produces.
//
// See Query Engine in BIGTABLE.md for the design.
func ExecutePlan(ctx context.Context, plan paramtools.ParamSet, table *bigtable.Table, tileKey TileKey, desc string) (<-chan string, error) {
	if err := validatePlan(plan); err != nil {
		return nil, skerr.Wrapf(err, "Plan is invalid")
	}

	intersectChannels := make([]<-chan string, 0, len(plan))
	for key, values := range plan {
		unionChannels := make([]<-chan string, len(values))
		for i, value := range values {
			unionChannels[i] = ParamIndex(ctx, table, tileKey, key, value, desc)
		}
		intersectChannels = append(intersectChannels, engine.NewUnion(ctx, unionChannels))
	}
	out := engine.NewIntersect(ctx, intersectChannels)
	return out, nil
}
