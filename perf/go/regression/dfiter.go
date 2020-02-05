package regression

import (
	"context"
	"fmt"
	"net/url"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/cid"
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

// NewDataFrameIterator retuns a DataFrameIterator that produces a set of
// dataframes for the given RegressionDetectionRequest.
func NewDataFrameIterator(ctx context.Context, progress types.Progress, req *RegressionDetectionRequest, dfBuilder dataframe.DataFrameBuilder) (DataFrameIterator, error) {
	u, err := url.ParseQuery(req.Query)
	if err != nil {
		return nil, err
	}
	q, err := query.New(u)
	if err != nil {
		return nil, err
	}
	df, err := dfBuilder.NewNFromQuery(ctx, req.End, q, req.N, progress)
	if err != nil {
		return nil, fmt.Errorf("Failed to build dataframe iterator: %s", err)
	}
	return &dataframeSlicer{
		df:     df,
		size:   req.Radius*2 + 1,
		offset: 0,
	}, nil
}

// singleIterator is a DataFrameIterator that produces a single DataFrame.
//
// This code is transitional to move regression detection to using
// DateFrameIterators for all regression detection.
type singleIterator struct {
	started   bool
	q         *query.Query
	begin     int
	end       int
	progress  types.Progress
	cidl      *cid.CommitIDLookup
	v         vcsinfo.VCS
	request   *RegressionDetectionRequest
	dfBuilder dataframe.DataFrameBuilder
}

// See DataFrameIterator.
func (s *singleIterator) Next() bool {
	ret := !s.started
	s.started = true
	return ret
}

// See DataFrameIterator.
func (s *singleIterator) Value(ctx context.Context) (*dataframe.DataFrame, error) {
	parsedQuery, err := url.ParseQuery(s.request.Query)
	if err != nil {
		return nil, fmt.Errorf("Invalid URL query: %s", err)
	}
	q, err := query.New(parsedQuery)
	if err != nil {
		return nil, fmt.Errorf("Invalid URL query: %s", err)
	}
	cidsWithDataInRange := func(begin, end int) ([]*cid.CommitID, error) {
		c := []*cid.CommitID{}
		for i := begin; i < end; i++ {
			c = append(c, &cid.CommitID{
				Offset: i,
			})
		}
		df, err := s.dfBuilder.NewFromCommitIDsAndQuery(ctx, c, s.cidl, q, nil)
		if err != nil {
			return nil, fmt.Errorf("Failed to load data searching for commit ids: %s", err)
		}
		return cidsWithData(df), nil
	}

	cids, err := calcCids(s.request, s.v, cidsWithDataInRange)
	if err != nil {
		return nil, fmt.Errorf("Could not calculate the commits to run a cluster over: %s", err)
	}
	df, err := s.dfBuilder.NewFromCommitIDsAndQuery(ctx, cids, s.cidl, q, s.progress)
	if err != nil {
		return nil, fmt.Errorf("Invalid range of commits: %s", err)
	}
	return df, err
}

// NewSingleDataFrameIterator creates a singeIterator instance.
func NewSingleDataFrameIterator(progress types.Progress, cidl *cid.CommitIDLookup, v vcsinfo.VCS, request *RegressionDetectionRequest, dfBuilder dataframe.DataFrameBuilder) *singleIterator {
	return &singleIterator{
		started:   false,
		progress:  progress,
		cidl:      cidl,
		v:         v,
		request:   request,
		dfBuilder: dfBuilder,
	}
}

// CidsWithDataInRange is passed to calcCids, and returns all
// the commit ids in [begin, end) that have data.
type CidsWithDataInRange func(begin, end int) ([]*cid.CommitID, error)

// cidsWithData returns the commit ids in the dataframe that have non-missing
// data in at least one trace.
func cidsWithData(df *dataframe.DataFrame) []*cid.CommitID {
	ret := []*cid.CommitID{}
	for i, h := range df.Header {
		for _, tr := range df.TraceSet {
			if tr[i] != vec32.MISSING_DATA_SENTINEL {
				ret = append(ret, &cid.CommitID{
					Offset: int(h.Offset),
				})
				break
			}
		}
	}
	return ret
}

// calcCids returns a slice of CommitID's that clustering should be run over.
func calcCids(request *RegressionDetectionRequest, v vcsinfo.VCS, cidsWithDataInRange CidsWithDataInRange) ([]*cid.CommitID, error) {
	cids := []*cid.CommitID{}
	if request.Sparse {
		// Sparse means data might not be available for every commit, so we need to scan
		// the data and gather up +/- Radius commits from the target commit that actually
		// do have data.

		// Start by checking center point as a quick exit strategy.
		withData, err := cidsWithDataInRange(request.Offset, request.Offset+1)
		if err != nil {
			return nil, err
		}
		if len(withData) == 0 {
			return nil, fmt.Errorf("No data at the target commit id.")
		}
		cids = append(cids, withData...)

		// Then check from the target forward in time.
		lastCommit := v.LastNIndex(1)
		lastIndex := lastCommit[0].Index
		finalIndex := request.Offset + 1 + SPARSE_BLOCK_SEARCH_MULT*request.Radius
		if finalIndex > lastIndex {
			finalIndex = lastIndex
		}
		withData, err = cidsWithDataInRange(request.Offset+1, finalIndex)
		if err != nil {
			return nil, err
		}
		if len(withData) < request.Radius {
			return nil, fmt.Errorf("Not enough sparse data after the target commit.")
		}
		cids = append(cids, withData[:request.Radius]...)

		// Finally check backward in time.
		backward := request.Radius
		startIndex := request.Offset - SPARSE_BLOCK_SEARCH_MULT*backward
		withData, err = cidsWithDataInRange(startIndex, request.Offset)
		if err != nil {
			return nil, err
		}
		if len(withData) < backward {
			return nil, fmt.Errorf("Not enough sparse data before the target commit.")
		}
		withData = withData[len(withData)-backward:]
		cids = append(withData, cids...)
	} else {
		if request.Radius <= 0 {
			request.Radius = 1
		}
		if request.Radius > MAX_RADIUS {
			request.Radius = MAX_RADIUS
		}
		from := request.Offset - request.Radius
		to := request.Offset + request.Radius
		for i := from; i <= to; i++ {
			cids = append(cids, &cid.CommitID{
				Offset: i,
			})
		}
	}
	return cids, nil
}
