package regression

import (
	"context"
	"fmt"
	"net/url"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

// DataFrameIterator is an iterator that produces DataFrames.
//
// for it.Next() {
//   df, err := it.Value(ctx)
//   // Do something with df.
// }
type DataFrameIterator interface {
	Next() bool
	Value(ctx context.Context) (*dataframe.DataFrame, error)
}

// dataframeSlicer implements DataFrameIterator by slicing sub-dataframes from
// a larger dataframe.
type dataframeSlicer struct {
	df     *dataframe.DataFrame
	size   int
	offset int
}

// See DataFrameIterator.
func (d *dataframeSlicer) Next() bool {
	return d.offset+d.size <= len(d.df.Header)
}

// See DataFrameIterator.
func (d *dataframeSlicer) Value(ctx context.Context) (*dataframe.DataFrame, error) {
	// Slice off a sub-dataframe from d.df.
	df, err := d.df.Slice(d.offset, d.size)
	if err != nil {
		return nil, err
	}
	d.offset += 1
	return df, nil
}

// NewDataFrameIterator returns a DataFrameIterator that produces a set of
// dataframes for the given RegressionDetectionRequest.
func NewDataFrameIterator(ctx context.Context, progress types.Progress, req *RegressionDetectionRequest, dfBuilder dataframe.DataFrameBuilder) (DataFrameIterator, error) {
	u, err := url.ParseQuery(req.Alert.Query)
	if err != nil {
		return nil, err
	}
	q, err := query.New(u)
	if err != nil {
		return nil, err
	}
	df, err := dfBuilder.NewNFromQuery(ctx, req.Domain.End, q, req.Domain.N, progress)
	if err != nil {
		return nil, fmt.Errorf("Failed to build dataframe iterator: %s", err)
	}
	return &dataframeSlicer{
		df:     df,
		size:   req.Alert.Radius*2 + 1,
		offset: 0,
	}, nil
}
