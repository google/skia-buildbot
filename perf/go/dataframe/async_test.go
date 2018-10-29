package dataframe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/types"
)

func TestMerge(t *testing.T) {
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
}

func TestDFAppend(t *testing.T) {
	a := DataFrame{
		Header: []*ColumnHeader{
			{
				Source: "master",
				Offset: 1,
			},
			{
				Source: "master",
				Offset: 2,
			},
			{
				Source: "master",
				Offset: 4,
			},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{0.1, 0.2, 0.4},
			",config=8888,arch=arm,": []float32{1.1, 1.2, 1.4},
		},
	}
	b := DataFrame{
		Header: []*ColumnHeader{
			{
				Source: "master",
				Offset: 3,
			},
			{
				Source: "master",
				Offset: 4,
			},
		},
		TraceSet: types.TraceSet{
			",config=565,arch=x86,": []float32{3.3, 3.4},
			",config=565,arch=arm,": []float32{4.3, 4.4},
		},
	}
	a.BuildParamSet()
	b.BuildParamSet()
	dfAppend(&a, &b)
}
