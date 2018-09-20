package dataframe

import (
	"context"
	"time"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/ptracestore"
)

// bttsDataFrameBuilder implements DataFrameBuilder using btts.
type bttsDataFrameBuilder struct {
	vcs   vcsinfo.VCS
	store btts.BigTableTraceStore
}

func NewDataFrameBuilderFromBTTS(vcs vcsinfo.VCS, store btts.BigTableTraceStore) DataFrameBuilder {
	return &bttsDataFrameBuilder{
		vcs:   vcs,
		store: store,
	}
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) New(progress ptracestore.Progress) (*DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewN(progress ptracestore.Progress, n int) (*DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewFromQueryAndRange(begin, end time.Time, q *query.Query, progress ptracestore.Progress) (*DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewFromKeysAndRange(keys []string, begin, end time.Time, progress ptracestore.Progress) (*DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewFromCommitIDsAndQuery(ctx context.Context, cids []*cid.CommitID, cidl *cid.CommitIDLookup, q *query.Query, progress ptracestore.Progress) (*DataFrame, error) {
	return nil, nil
}

// Validate that the concrete bttsDataFrameBuilder faithfully implements the DataFrameBuidler interface.
var _ DataFrameBuilder = (*bttsDataFrameBuilder)(nil)
