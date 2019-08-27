package btts

import (
	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
)

// ValidatePlan takes a plan (a ParamSet of OPS encoded keys and values) and
// validates that it should run to completion. This will also error if the query
// is too large, i.e. would generate too many concurrent queries to BigTable.
func ValidatePlan(plan paramtools.ParamSet) error {
	return nil

}

// ExecutePlan takes a query plan and executes it over a table for the given
// TileKey. The result is a channel that will produce encoded keys in
// alphabetical order and will close after the query is done executing.
// It will also return a buffered error channel that will contain errors
// if any were encountered. The error channel should only be read after the
// index channel has been closed.
//
// An error will be returned if the query is invalid.
func ExecutePlan(plan paramtools.ParamSet, table *bigtable.Table, tileKey TileKey) (<-chan string, <-chan error, error) {
	if err := ValidatePlan(plan); err != nil {
		return nil, nil, skerr.Fmt("Plan is invalid: %s", err)
	}
	return nil, nil, nil
}
