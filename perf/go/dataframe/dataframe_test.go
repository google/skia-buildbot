package dataframe

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/ptracestore"
)

type mockVcs struct {
	commits    []*vcsinfo.IndexCommit
	updateFail bool
}

func (m *mockVcs) From(start time.Time) []string                     { return nil }
func (m *mockVcs) Range(begin, end time.Time) []*vcsinfo.IndexCommit { return nil }
func (m *mockVcs) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	return nil, nil
}

func (m *mockVcs) IndexOf(ctx context.Context, hash string) (int, error) {
	for i, c := range m.commits {
		if c.Hash == hash {
			return i, nil
		}
	}
	return 0, fmt.Errorf("Not found: %s", hash)
}

func (m *mockVcs) Update(ctx context.Context, pull, allBranches bool) error {
	if m.updateFail {
		return fmt.Errorf("Failed to update.")
	}
	return nil
}

func (m *mockVcs) LastNIndex(N int) []*vcsinfo.IndexCommit {
	return m.commits
}

func (m *mockVcs) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) {
	return nil, nil
}

func (m *mockVcs) GetFile(ctx context.Context, fileName, commitHash string) (string, error) {
	return "", nil
}
func (m *mockVcs) ResolveCommit(ctx context.Context, commitHash string) (string, error) {
	return "", nil
}

type mockPTraceStore struct {
	traceSet  ptracestore.TraceSet
	matchFail bool
}

func (m mockPTraceStore) Add(commitID *cid.CommitID, values map[string]float32, sourceFile string, ts time.Time) error {
	return nil
}

func (m mockPTraceStore) Details(commitID *cid.CommitID, traceID string) (string, float32, error) {
	return "", 0, nil
}

func (m mockPTraceStore) Match(commitIDs []*cid.CommitID, matches ptracestore.KeyMatches, progress ptracestore.Progress) (ptracestore.TraceSet, error) {
	if m.matchFail {
		return nil, fmt.Errorf("Failed to retrieve traces.")
	}
	return m.traceSet, nil
}

var (
	ts0 = time.Unix(1406721642, 0).UTC()
	ts1 = time.Unix(1406721715, 0).UTC()

	commits = []*vcsinfo.IndexCommit{
		{
			Hash:      "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f",
			Index:     0,
			Timestamp: ts0,
		},
		{
			Hash:      "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			Index:     1,
			Timestamp: ts1,
		},
	}

	store = mockPTraceStore{
		traceSet: ptracestore.TraceSet{
			",arch=x86,config=565,":  ptracestore.Trace([]float32{1.2, 2.1}),
			",arch=x86,config=8888,": ptracestore.Trace([]float32{1.3, 3.1}),
			",arch=x86,config=gpu,":  ptracestore.Trace([]float32{1.4, 4.1}),
		},
	}
)

func TestRangeImpl(t *testing.T) {
	testutils.SmallTest(t)

	expected_headers := []*ColumnHeader{
		{
			Source:    "master",
			Offset:    0,
			Timestamp: ts0.Unix(),
		},
		{
			Source:    "master",
			Offset:    1,
			Timestamp: ts1.Unix(),
		},
	}
	expected_pcommits := []*cid.CommitID{
		{
			Offset: 0,
			Source: "master",
		},
		{
			Offset: 1,
			Source: "master",
		},
	}

	headers, pcommits, _ := rangeImpl(commits, 0)
	assert.Equal(t, 2, len(headers))
	assert.Equal(t, 2, len(pcommits))
	deepequal.AssertDeepEqual(t, expected_headers, headers)
	deepequal.AssertDeepEqual(t, expected_pcommits, pcommits)

	headers, pcommits, _ = rangeImpl([]*vcsinfo.IndexCommit{}, 0)
	assert.Equal(t, 0, len(headers))
	assert.Equal(t, 0, len(pcommits))
}

func TestNew(t *testing.T) {
	testutils.SmallTest(t)
	colHeaders := []*ColumnHeader{
		{
			Source:    "master",
			Offset:    0,
			Timestamp: ts0.Unix(),
		},
		{
			Source:    "master",
			Offset:    1,
			Timestamp: ts1.Unix(),
		},
	}
	pcommits := []*cid.CommitID{
		{
			Offset: 0,
			Source: "master",
		},
		{
			Offset: 1,
			Source: "master",
		},
	}
	store.matchFail = false

	matches := func(key string) bool {
		return true
	}
	d, err := _new(colHeaders, pcommits, matches, store, nil, 1)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(d.TraceSet))
	assert.True(t, util.SSliceEqual(d.ParamSet["arch"], []string{"x86"}))
	assert.True(t, util.SSliceEqual(d.ParamSet["config"], []string{"8888", "565", "gpu"}))
	assert.Equal(t, 1, d.Skip)
}

func TestVCS(t *testing.T) {
	testutils.SmallTest(t)
	vcs := &mockVcs{
		commits: commits,
	}
	store.matchFail = false

	dfBuiler := NewDataFrameBuilderFromPTraceStore(vcs, store)

	d, err := dfBuiler.New(nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(d.TraceSet))

	d, err = dfBuiler.NewFromQueryAndRange(ts0, ts1.Add(time.Second), &query.Query{}, nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(d.TraceSet))

	// Test error conditions, i.e. that we log only and don't return an error.
	vcs.updateFail = true
	_, err = dfBuiler.New(nil)
	assert.NoError(t, err)
	_, err = dfBuiler.NewFromQueryAndRange(ts0, ts1.Add(time.Second), &query.Query{}, nil)
	assert.NoError(t, err)

	store.matchFail = true
	dfBuiler = NewDataFrameBuilderFromPTraceStore(vcs, store)
	// Test error conditions if the store fails.
	_, err = dfBuiler.New(nil)
	assert.Error(t, err)
	_, err = dfBuiler.NewFromQueryAndRange(ts0, ts1.Add(time.Second), &query.Query{}, nil)
	assert.Error(t, err)
}

func TestBuildParamSet(t *testing.T) {
	testutils.SmallTest(t)
	// Test the empty case first.
	df := &DataFrame{
		TraceSet: ptracestore.TraceSet{},
		ParamSet: paramtools.ParamSet{},
	}
	df.BuildParamSet()
	assert.Equal(t, 0, len(df.ParamSet))

	df = &DataFrame{
		TraceSet: ptracestore.TraceSet{
			",arch=x86,config=565,":  ptracestore.Trace([]float32{1.2, 2.1}),
			",arch=x86,config=8888,": ptracestore.Trace([]float32{1.3, 3.1}),
			",arch=x86,config=gpu,":  ptracestore.Trace([]float32{1.4, 4.1}),
		},
		ParamSet: paramtools.ParamSet{},
	}
	df.BuildParamSet()
	assert.Equal(t, 2, len(df.ParamSet))
	values, ok := df.ParamSet["arch"]
	assert.True(t, ok)
	assert.Equal(t, []string{"x86"}, values)
	values, ok = df.ParamSet["config"]
	assert.True(t, ok)
	assert.Equal(t, []string{"565", "8888", "gpu"}, values)
}

func TestFilter(t *testing.T) {
	testutils.SmallTest(t)
	df := &DataFrame{
		TraceSet: ptracestore.TraceSet{
			",arch=x86,config=565,":  ptracestore.Trace([]float32{1.2, 2.1}),
			",arch=x86,config=8888,": ptracestore.Trace([]float32{1.3, 3.1}),
			",arch=x86,config=gpu,":  ptracestore.Trace([]float32{1.4, 4.1}),
		},
		ParamSet: paramtools.ParamSet{},
	}
	f := func(tr ptracestore.Trace) bool {
		return tr[0] > 1.25
	}
	df.FilterOut(f)
	assert.Equal(t, 1, len(df.TraceSet))
	assert.Equal(t, []string{"565"}, df.ParamSet["config"])

	df = &DataFrame{
		TraceSet: ptracestore.TraceSet{
			",arch=x86,config=565,":  ptracestore.Trace([]float32{1.2, 2.1}),
			",arch=x86,config=8888,": ptracestore.Trace([]float32{1.3, 3.1}),
			",arch=x86,config=gpu,":  ptracestore.Trace([]float32{1.4, 4.1}),
		},
		ParamSet: paramtools.ParamSet{},
	}
	f = func(tr ptracestore.Trace) bool {
		return true
	}
	df.FilterOut(f)
	assert.Equal(t, 0, len(df.TraceSet))
}
