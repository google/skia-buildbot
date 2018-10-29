package dataframe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

func TestMerge(t *testing.T) {
	testutils.SmallTest(t)
	// Simple
	a := []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	b := []*ColumnHeader{
		{Offset: 3},
		{Offset: 4},
	}
	m, aMap, bMap := merge(a, b)
	expected := []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 3},
		{Offset: 4},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 3}, aMap)
	assert.Equal(t, map[int]int{0: 2, 1: 3}, bMap)

	// Skips
	a = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	b = []*ColumnHeader{
		{Offset: 5},
		{Offset: 7},
	}
	m, aMap, bMap = merge(a, b)
	expected = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
		{Offset: 5},
		{Offset: 7},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 2}, aMap)
	assert.Equal(t, map[int]int{0: 3, 1: 4}, bMap)

	// Empty b
	a = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	b = []*ColumnHeader{}
	m, aMap, bMap = merge(a, b)
	expected = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 2}, aMap)
	assert.Equal(t, map[int]int{}, bMap)

	// Empty a
	a = []*ColumnHeader{}
	b = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	m, aMap, bMap = merge(a, b)
	expected = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{}, aMap)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 2}, bMap)

	// Empty a and b.
	a = []*ColumnHeader{}
	b = []*ColumnHeader{}
	m, aMap, bMap = merge(a, b)
	expected = []*ColumnHeader{}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{}, aMap)
	assert.Equal(t, map[int]int{}, bMap)
}

func TestDFAppend(t *testing.T) {
	testutils.SmallTest(t)
	a := DataFrame{
		Header: []*ColumnHeader{
			{Offset: 1},
			{Offset: 2},
			{Offset: 4},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{0.1, 0.2, 0.4},
			",config=8888,arch=arm,": []float32{1.1, 1.2, 1.4},
		},
	}
	b := DataFrame{
		Header: []*ColumnHeader{
			{Offset: 3},
			{Offset: 4},
		},
		TraceSet: types.TraceSet{
			",config=565,arch=x86,": []float32{3.3, 3.4},
			",config=565,arch=arm,": []float32{4.3, 4.4},
		},
	}
	a.BuildParamSet()
	b.BuildParamSet()
	r := Join(&a, &b)

	expectedHeader := []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 3},
		{Offset: 4},
	}

	assert.Equal(t, expectedHeader, r.Header)
	assert.Len(t, r.TraceSet, 4)
	e := vec32.MISSING_DATA_SENTINEL
	assert.Equal(t, types.Trace{0.1, 0.2, e, 0.4}, r.TraceSet[",config=8888,arch=x86,"])
	assert.Equal(t, types.Trace{1.1, 1.2, e, 1.4}, r.TraceSet[",config=8888,arch=arm,"])
	assert.Equal(t, types.Trace{e, e, 4.3, 4.4}, r.TraceSet[",config=565,arch=arm,"])
	assert.Equal(t, types.Trace{e, e, 3.3, 3.4}, r.TraceSet[",config=565,arch=x86,"])
}
