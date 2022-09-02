package tracesetbuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

const e = vec32.MissingDataSentinel

func TestBuilder(t *testing.T) {

	traces1 := types.TraceSet{
		",arch=x86,name=foo,": []float32{1.0, 2.0},
		",arch=x86,name=bar,": []float32{3.0, 4.0},
		",arch=x86,name=baz,": []float32{5.0, 6.0},
	}
	traceMap1 := map[int32]int32{
		0: 0,
		1: 1,
	}

	traces2 := types.TraceSet{
		",arch=x86,name=foo,": []float32{3.3, 4.4},
		",arch=x86,name=bar,": []float32{5.5, 6.6},
		",arch=x86,name=baz,": []float32{7.7, 8.8},
	}
	traceMap2 := map[int32]int32{
		0: 3,
		1: 4,
	}

	builder := New(5)
	builder.Add(traceMap1, traces1)
	builder.Add(traceMap2, traces2)
	traceSet, ops := builder.Build(context.Background())
	builder.Close()
	assert.Len(t, traceSet, 3)
	assert.Equal(t, traceSet[",arch=x86,name=foo,"], types.Trace{1.0, 2.0, e, 3.3, 4.4})
	assert.Equal(t, traceSet[",arch=x86,name=bar,"], types.Trace{3.0, 4.0, e, 5.5, 6.6})
	assert.Equal(t, traceSet[",arch=x86,name=baz,"], types.Trace{5.0, 6.0, e, 7.7, 8.8})
	assert.Len(t, ops, 2)
	expectedParamSet := paramtools.ParamSet{
		"arch": []string{"x86"},
		"name": []string{"bar", "baz", "foo"},
	}
	expectedParamSet.Normalize()
	assert.Equal(t, expectedParamSet.Freeze(), ops)
}

func TestBuilderEmpty(t *testing.T) {

	builder := New(5)
	defer builder.Close()
	traceSet, ops := builder.Build(context.Background())
	assert.Len(t, traceSet, 0)
	assert.Len(t, ops, 0)
}
