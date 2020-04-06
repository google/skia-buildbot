package regression

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
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
func NewDataFrameIterator(ctx context.Context, progress types.Progress, req *RegressionDetectionRequest, dfBuilder dataframe.DataFrameBuilder, perfGit *perfgit.Git) (DataFrameIterator, error) {
	u, err := url.ParseQuery(req.Alert.Query)
	if err != nil {
		return nil, err
	}
	q, err := query.New(u)
	if err != nil {
		return nil, err
	}
	var df *dataframe.DataFrame
	if req.Domain.Offset == 0 {
		df, err = dfBuilder.NewNFromQuery(ctx, req.Domain.End, q, req.Domain.N, progress)
		if err != nil {
			return nil, fmt.Errorf("Failed to build dataframe iterator source dataframe: %s", err)
		}
	} else {
		// We can get an iterator that returns just a single dataframe by making
		// sure that the size of the origin dataframe is the same size as the
		// slicer size, so we set them both to 2*Radius+1.
		n := int32(2*req.Alert.Radius + 1)
		// Need to find an End time, which is the commit time of the commit at Offset+Radius.
		commit, err := perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(int(req.Domain.Offset)+req.Alert.Radius-1))
		if err != nil {
			return nil, fmt.Errorf("Failed to look up Offset of a single cluster request: %s", err)
		}
		df, err = dfBuilder.NewNFromQuery(ctx, time.Unix(commit.Timestamp, 0), q, n, progress)
		if err != nil {
			return nil, fmt.Errorf("Failed to build dataframe iterator source dataframe: %s", err)
		}
	}
	return &dataframeSlicer{
		df:     df,
		size:   2*req.Alert.Radius + 1,
		offset: 0,
	}, nil
}
