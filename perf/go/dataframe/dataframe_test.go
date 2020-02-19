package dataframe

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/types"
)

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
)

func TestRangeImpl(t *testing.T) {
	unittest.SmallTest(t)

	expected_headers := []*ColumnHeader{
		{
			Offset:    0,
			Timestamp: ts0.Unix(),
		},
		{
			Offset:    1,
			Timestamp: ts1.Unix(),
		},
	}
	expected_pcommits := []*cid.CommitID{
		{
			Offset: 0,
		},
		{
			Offset: 1,
		},
	}

	headers, pcommits, _ := rangeImpl(commits, 0)
	assert.Equal(t, 2, len(headers))
	assert.Equal(t, 2, len(pcommits))
	assertdeep.Equal(t, expected_headers, headers)
	assertdeep.Equal(t, expected_pcommits, pcommits)

	headers, pcommits, _ = rangeImpl([]*vcsinfo.IndexCommit{}, 0)
	assert.Equal(t, 0, len(headers))
	assert.Equal(t, 0, len(pcommits))
}

func TestBuildParamSet(t *testing.T) {
	unittest.SmallTest(t)
	// Test the empty case first.
	df := &DataFrame{
		TraceSet: types.TraceSet{},
		ParamSet: paramtools.ParamSet{},
	}
	df.BuildParamSet()
	assert.Equal(t, 0, len(df.ParamSet))

	df = &DataFrame{
		TraceSet: types.TraceSet{
			",arch=x86,config=565,":  types.Trace([]float32{1.2, 2.1}),
			",arch=x86,config=8888,": types.Trace([]float32{1.3, 3.1}),
			",arch=x86,config=gpu,":  types.Trace([]float32{1.4, 4.1}),
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
	unittest.SmallTest(t)
	df := &DataFrame{
		TraceSet: types.TraceSet{
			",arch=x86,config=565,":  types.Trace([]float32{1.2, 2.1}),
			",arch=x86,config=8888,": types.Trace([]float32{1.3, 3.1}),
			",arch=x86,config=gpu,":  types.Trace([]float32{1.4, 4.1}),
		},
		ParamSet: paramtools.ParamSet{},
	}
	f := func(tr types.Trace) bool {
		return tr[0] > 1.25
	}
	df.FilterOut(f)
	assert.Equal(t, 1, len(df.TraceSet))
	assert.Equal(t, []string{"565"}, df.ParamSet["config"])

	df = &DataFrame{
		TraceSet: types.TraceSet{
			",arch=x86,config=565,":  types.Trace([]float32{1.2, 2.1}),
			",arch=x86,config=8888,": types.Trace([]float32{1.3, 3.1}),
			",arch=x86,config=gpu,":  types.Trace([]float32{1.4, 4.1}),
		},
		ParamSet: paramtools.ParamSet{},
	}
	f = func(tr types.Trace) bool {
		return true
	}
	df.FilterOut(f)
	assert.Equal(t, 0, len(df.TraceSet))
}

func TestSlice(t *testing.T) {
	unittest.SmallTest(t)
	df := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 10},
			{Offset: 12},
			{Offset: 14},
			{Offset: 15},
			{Offset: 16},
			{Offset: 17},
		},
		TraceSet: types.TraceSet{
			",arch=x86,config=565,":  types.Trace([]float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6}),
			",arch=x86,config=8888,": types.Trace([]float32{1.1, 1.2, 1.3, 1.4, 1.5, 1.6}),
			",arch=x86,config=gpu,":  types.Trace([]float32{2.1, 2.2, 2.3, 2.4, 2.5, 2.6}),
		},
		ParamSet: paramtools.ParamSet{},
	}

	// Test error conditions.
	_, err := df.Slice(0, 10)
	assert.Error(t, err)

	_, err = df.Slice(4, 3)
	assert.Error(t, err)

	// Test boundary conditions.
	sub, err := df.Slice(1, 0)
	assert.NoError(t, err)
	assert.Equal(t, []*ColumnHeader{}, sub.Header)
	assert.Len(t, sub.TraceSet, 3)
	assert.Len(t, sub.TraceSet[",arch=x86,config=gpu,"], 0)

	// Test the happy path.
	sub, err = df.Slice(0, 3)
	assert.NoError(t, err)
	assert.Equal(t, []*ColumnHeader{
		{Offset: 10},
		{Offset: 12},
		{Offset: 14},
	}, sub.Header)
	assert.Len(t, sub.TraceSet, 3)
	assert.Equal(t, sub.TraceSet[",arch=x86,config=gpu,"], types.Trace([]float32{2.1, 2.2, 2.3}))
	assert.Equal(t, sub.ParamSet, paramtools.ParamSet{"arch": []string{"x86"}, "config": []string{"565", "8888", "gpu"}})

	sub, err = df.Slice(1, 3)
	assert.NoError(t, err)
	assert.Equal(t, []*ColumnHeader{
		{Offset: 12},
		{Offset: 14},
		{Offset: 15},
	}, sub.Header)
	assert.Len(t, sub.TraceSet, 3)
	assert.Equal(t, sub.TraceSet[",arch=x86,config=gpu,"], types.Trace([]float32{2.2, 2.3, 2.4}))
	assert.Equal(t, sub.ParamSet, paramtools.ParamSet{"arch": []string{"x86"}, "config": []string{"565", "8888", "gpu"}})

}
