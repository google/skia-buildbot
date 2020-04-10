package regression

import (
	"context"
	"net/url"
	"time"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
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
//
// If req.Domain.Offset is non-zero then we want the iterator to return a single
// dataframe of req.Alert.Radius around the specified commit. Otherwise it
// returns a series of dataframes of size 2*req.Alert.Radius+1 sliced from a
// single dataframe of size req.Domain.N.
func NewDataFrameIterator(ctx context.Context, progress types.Progress, req *RegressionDetectionRequest, dfBuilder dataframe.DataFrameBuilder, perfGit *perfgit.Git) (DataFrameIterator, error) {
	u, err := url.ParseQuery(req.Query)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	q, err := query.New(u)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var df *dataframe.DataFrame
	if req.Domain.Offset == 0 {
		df, err = dfBuilder.NewNFromQuery(ctx, req.Domain.End, q, req.Domain.N, progress)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to build dataframe iterator source dataframe")
		}
	} else {
		// We can get an iterator that returns just a single dataframe by making
		// sure that the size of the origin dataframe is the same size as the
		// slicer size, so we set them both to 2*Radius+1.
		n := int32(2*req.Alert.Radius + 1)
		// Need to find an End time, which is the commit time of the commit at Offset+Radius.
		commit, err := perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(int(req.Domain.Offset)+req.Alert.Radius-1))
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to look up CommitNumber of a single cluster request")
		}
		df, err = dfBuilder.NewNFromQuery(ctx, time.Unix(commit.Timestamp, 0), q, n, progress)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to build dataframe iterator source dataframe.")
		}
	}
	return &dataframeSlicer{
		df:     df,
		size:   2*req.Alert.Radius + 1,
		offset: 0,
	}, nil
}
