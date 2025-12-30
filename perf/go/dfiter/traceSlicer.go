package dfiter

import (
	"context"
	"sort"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

type dfTraceSlicer struct {
	df           *dataframe.DataFrame
	keys         []string
	currentIndex int
}

// See DataFrameIterator.
func (d *dfTraceSlicer) Next() bool {
	return d.currentIndex < len(d.keys)
}

// See DataFrameIterator.
func (d *dfTraceSlicer) Value(ctx context.Context) (*dataframe.DataFrame, error) {
	traceID := d.keys[d.currentIndex]
	trace := d.df.TraceSet[traceID]
	d.currentIndex++

	paramsForTrace := paramtools.NewParams(traceID)
	return &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			traceID: trace,
		},
		ParamSet: paramtools.ReadOnlyParamSet(paramtools.NewParamSet(paramsForTrace)),
		Header:   d.df.Header,
	}, nil
}

func NewDfTraceSlicer(df *dataframe.DataFrame) *dfTraceSlicer {
	keys := make([]string, 0, len(df.TraceSet))
	// Store the trace keys and the dataframe. Let's not split the dataframe at this stage
	// since that will consume a lot more memory. We create the individual frames when
	// we need them in the Value() function.
	for k := range df.TraceSet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return &dfTraceSlicer{
		df:   df,
		keys: keys,
	}
}
