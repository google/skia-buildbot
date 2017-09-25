package clustering2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/ptracestore"
)

const (
	e = vec32.MISSING_DATA_SENTINEL
)

func TestTooMuchMissingData(t *testing.T) {
	testutils.SmallTest(t)
	testCases := []struct {
		value    ptracestore.Trace
		expected bool
		message  string
	}{
		{
			value:    ptracestore.Trace{e, e, 1, 1, 1},
			expected: true,
			message:  "missing one side",
		},
		{
			value:    ptracestore.Trace{1, e, 1, 1, 1},
			expected: false,
			message:  "exactly 50%",
		},
		{
			value:    ptracestore.Trace{1, 1, e, 1, 1},
			expected: true,
			message:  "missing midpoint",
		},
		{
			value:    ptracestore.Trace{e, e, 1, 1},
			expected: true,
			message:  "missing one side - even",
		},
		{
			value:    ptracestore.Trace{e, 1, 1, 1},
			expected: false,
			message:  "exactly 50% - even",
		},
		{
			value:    ptracestore.Trace{e, 1, 1},
			expected: true,
			message:  "Radius = 1",
		},
		{
			value:    ptracestore.Trace{1},
			expected: false,
			message:  "len(tr) < 3",
		},
	}

	for _, tc := range testCases {
		if got, want := tooMuchMissingData(tc.value), tc.expected; got != want {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}

func TestCalcCidsNotSparse(t *testing.T) {
	testutils.SmallTest(t)

	r := &ClusterRequest{
		Source: "master",
		Offset: 2000,
		Radius: 3,
		Query:  "config=8888",
		Sparse: false,
	}

	cids, err := calcCids(r, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "master-001997", cids[0].ID())
	assert.Equal(t, "master-002003", cids[6].ID())
}

type mockVcs struct {
}

func (m *mockVcs) LastNIndex(N int) []*vcsinfo.IndexCommit {
	return []*vcsinfo.IndexCommit{&vcsinfo.IndexCommit{Index: 2005}}
}
func (m *mockVcs) Update(pull, allBranches bool) error               { return nil }
func (m *mockVcs) From(start time.Time) []string                     { return nil }
func (m *mockVcs) Range(begin, end time.Time) []*vcsinfo.IndexCommit { return nil }
func (m *mockVcs) IndexOf(hash string) (int, error)                  { return 0, nil }
func (m *mockVcs) ByIndex(N int) (*vcsinfo.LongCommit, error)        { return nil, nil }
func (m *mockVcs) Details(hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	return nil, nil
}
func (m *mockVcs) GetFile(fileName, commitHash string) (string, error) { return "", nil }

func TestCalcCidsSparse(t *testing.T) {
	testutils.SmallTest(t)

	r := &ClusterRequest{
		Source: "master",
		Offset: 2000,
		Radius: 3,
		Query:  "config=8888",
		Sparse: true,
	}

	i := 0
	ends := []int{}
	begins := []int{}
	type cidSlice []*cid.CommitID
	rets := []cidSlice{
		cidSlice{&cid.CommitID{Source: "master", Offset: 2000}},
		cidSlice{
			&cid.CommitID{Source: "master", Offset: 2001},
			&cid.CommitID{Source: "master", Offset: 2002},
			&cid.CommitID{Source: "master", Offset: 2004},
		},
		cidSlice{
			&cid.CommitID{Source: "master", Offset: 1997},
			&cid.CommitID{Source: "master", Offset: 1998},
			&cid.CommitID{Source: "master", Offset: 1999},
		},
	}
	cidsWithDataInRange := func(begin, end int) ([]*cid.CommitID, error) {
		defer func() { i += 1 }()
		ends = append(ends, end)
		begins = append(begins, begin)
		return rets[i], nil
	}

	cids, err := calcCids(r, &mockVcs{}, cidsWithDataInRange)
	assert.NoError(t, err)
	assert.Equal(t, "master-001997", cids[0].ID())
	assert.Equal(t, "master-002004", cids[6].ID())
	assert.Equal(t, []int{2000, 2001, 1700}, begins)
	assert.Equal(t, []int{2001, 2005, 2000}, ends)
}

func TestCalcCidsSparseFails(t *testing.T) {
	testutils.SmallTest(t)

	r := &ClusterRequest{
		Source: "master",
		Offset: 2000,
		Radius: 3,
		Query:  "config=8888",
		Sparse: true,
	}

	cidsWithDataInRange := func(begin, end int) ([]*cid.CommitID, error) {
		return []*cid.CommitID{}, nil
	}

	_, err := calcCids(r, &mockVcs{}, cidsWithDataInRange)
	assert.Error(t, err)
}

func TestCidsWithData(t *testing.T) {
	testutils.SmallTest(t)

	e := vec32.MISSING_DATA_SENTINEL
	headers := []*dataframe.ColumnHeader{
		{Source: "master", Offset: 2000},
		{Source: "master", Offset: 2001},
		{Source: "master", Offset: 2002},
	}
	traceSet := ptracestore.TraceSet{
		",arch=x86,config=565,":  ptracestore.Trace([]float32{e, 2.1, e}),
		",arch=x86,config=8888,": ptracestore.Trace([]float32{e, 3.1, e}),
		",arch=x86,config=gpu,":  ptracestore.Trace([]float32{1.4, 4.1, e}),
	}
	d := &dataframe.DataFrame{
		TraceSet: traceSet,
		Header:   headers,
	}

	cids := cidsWithData(d)
	assert.Len(t, cids, 2)
	assert.Equal(t, "master-002000", cids[0].ID())
	assert.Equal(t, "master-002001", cids[1].ID())
}

func TestCidsWithDataEmpty(t *testing.T) {
	testutils.SmallTest(t)

	d := &dataframe.DataFrame{
		TraceSet: ptracestore.TraceSet{},
		Header:   []*dataframe.ColumnHeader{},
	}

	cids := cidsWithData(d)
	assert.Len(t, cids, 0)
}
