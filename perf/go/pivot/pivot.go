// Package pivot provides the ability to pivot dataframes.
//
// That is, given a set of traces:
//
//    types.TraceSet{
//      ",arch=arm,config=8888,":   types.Trace{1, 0, 0},
//      ",arch=arm,config=565,":    types.Trace{0, 2, 0},
//      ",arch=arm,config=gles,":   types.Trace{0, 0, 3},
//      ",arch=intel,config=8888,": types.Trace{1, 2, 3},
//      ",arch=intel,config=565,":  types.Trace{1, 2, 3},
//      ",arch=intel,config=gles,": types.Trace{1, 2, 3},
//    }
//
// You may want to compare how 'arm' machines compare to 'intel' machines. If
// the traces were stored in a spreadsheet then answering that question would
// require a pivot table, where you would pivot over the 'arch' key. This is
// also similar to a GROUP BY operation in SQL, and for such queries you also
// need to supply the type of operation to apply to all the values that appear
// in each group.
//
// So if we created a pivot Request of the form:
//
//     req := Request {
//       GroupBy:   []string{"arch"},
//       Operation: Sum,
//     }
//
// it would pivot those traces and return summary traces:
//
//    types.TraceSet{
//      ",arch=arm,":   types.Trace{1, 2, 3},
//      ",arch=intel,": types.Trace{3, 6, 9},
//    }
//
// Note how the trace ids only contain keys that appear in the GroupBy list, as
// these new traces represent each group.
//
// The above set of generated traces could be plotted. But we may want to
// summarize the data further into a table, so we can optionally apply Summary
// operations that will be applied to the resulting traces:
//
//     req := Request {
//       GroupBy:   []string{"arch"},
//       Operation: Sum,
//       Summary: []Operation{Avg},
//     }
//
// Applied to the same traces above we now get:
//
//    types.TraceSet{
//      ",arch=arm,":   types.Trace{2}, // (1+2+3)/3
//      ",arch=intel,": types.Trace{6}, // (3+6+9)/3
//    }
//
// Note that muliple Summary operations can be applied, and each one will
// generate its own column in the resulting TraceSet.
package pivot

import (
	"context"

	"go.skia.org/infra/go/calc"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

// Operation that can be applied to pivot values.
type Operation string

// Operation constants.
const (
	Sum Operation = "sum"
	Avg Operation = "avg"
)

// AllOperations for exporting to TypeScript.
var AllOperations = []Operation{Sum, Avg}

// Request controls how a pivot is done.
type Request struct {
	// Which keys to group by.
	GroupBy []string `json:"group_by"`

	// Operation to apply when grouping.
	Operation Operation `json:"operation"`

	// If Summary is the empty slice then the Summary is commits, i.e. a plot.
	// otherwise produce one column for each Operation in Summary.
	Summary []Operation `json:"summary"`
}

type groupByOperation func(types.TraceSet) types.Trace

type summaryOperation func([]float32) float32

// For each type of operation store both the group by and the summary
// operation functions.
type operationFunctions struct {
	groupByOperation groupByOperation
	summaryOperation summaryOperation
}

// opMap contains all the known operation implementations for both GroupBy and
// Summary operations. Keeping it in a table like this ensures that we always
// have both groupBy and summary functions available.
var opMap map[Operation]operationFunctions = map[Operation]operationFunctions{
	Sum: {
		groupByOperation: calc.SumFuncImpl,
		summaryOperation: vec32.Sum,
	},
	Avg: {
		groupByOperation: calc.AveFuncImpl,
		summaryOperation: vec32.Mean,
	},
}

// Valid returns an error if the Request is not valid.
func (o Request) Valid() error {
	if len(o.GroupBy) == 0 {
		return skerr.Fmt("at least one GroupBy value must be supplied.")
	}

	valid := false
	for _, op := range AllOperations {
		if op == o.Operation {
			valid = true
			break
		}
	}
	if !valid {
		return skerr.Fmt("invalid Operation value: %q", o.Operation)
	}

	valid = false
	for _, incomingOp := range o.Summary {
		for _, op := range AllOperations {
			if op == incomingOp {
				valid = true
				break
			}
		}
		if !valid {
			return skerr.Fmt("invalid Summary value: %q", incomingOp)
		}
	}
	return nil
}

// Returns nil if a groupBy key is missing from fullKey.
func groupKeyFromTraceKey(fullKeyAsParam paramtools.Params, groupBy []string) string {
	ret := paramtools.Params{}
	for _, group := range groupBy {
		value, ok := fullKeyAsParam[group]
		if !ok {
			return ""
		}
		ret[group] = value
	}
	key, err := query.MakeKeyFast(ret)
	if err != nil {
		return ""
	}
	return key
}

// Pivot returns a new Dataframe with the pivot described in Request applied.
func Pivot(ctx context.Context, req Request, df *dataframe.DataFrame) (*dataframe.DataFrame, error) {
	if err := req.Valid(); err != nil {
		return nil, skerr.Wrap(err)
	}
	ret := dataframe.NewEmpty()

	// Pre-populate groupedTraceSets with empty types.TraceSet{}s.
	groupedTraceSets := map[string]types.TraceSet{}
	cpCh, err := df.ParamSet.CartesianProduct(req.GroupBy)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	for p := range cpCh {
		groupID, err := query.MakeKeyFast(p)
		if err != nil {
			continue
		}
		groupedTraceSets[groupID] = types.TraceSet{}
	}

	// Loop over all members of TraceSet and put them into groups.
	for traceID, trace := range df.TraceSet {
		p, err := query.ParseKeyFast(traceID)
		if err != nil {
			continue
		}

		groupKey := groupKeyFromTraceKey(p, req.GroupBy)

		// If the trace doesn't fit in any group then ignore it.
		if groupKey == "" {
			continue
		}
		groupedTraceSets[groupKey][traceID] = trace
	}

	// Do the GroupBy Operation.
	for groupID, traces := range groupedTraceSets {
		ret.TraceSet[groupID] = opMap[req.Operation].groupByOperation(traces)
		if ctx.Err() != nil {
			return nil, skerr.Wrap(ctx.Err())
		}
	}

	// Return now if there aren't any Summary operations.
	if len(req.Summary) == 0 {
		// Use the original Header from the DataFrame.
		ret.Header = df.Header
		return ret, nil
	}

	// Make summary columns.
	for groupKey, trace := range ret.TraceSet {
		summaryValues := make(types.Trace, len(req.Summary))
		for i, op := range req.Summary {
			summaryValues[i] = opMap[op].summaryOperation(trace)
		}
		ret.TraceSet[groupKey] = summaryValues
		if ctx.Err() != nil {
			return nil, skerr.Wrap(ctx.Err())
		}

	}

	// Adjust Header to match the Summary columns.
	ret.Header = make([]*dataframe.ColumnHeader, len(req.Summary))
	for i := 0; i < len(req.Summary); i++ {
		ret.Header[i] = &dataframe.ColumnHeader{
			Offset: types.CommitNumber(i),
		}
	}

	ret.BuildParamSet()

	return ret, nil
}
