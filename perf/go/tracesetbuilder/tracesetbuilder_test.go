package tracesetbuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

const e = vec32.MISSING_DATA_SENTINEL

func encodeTraces(t *testing.T, traces types.TraceSet) (*paramtools.OrderedParamSet, map[string][]float32) {
	ret := map[string][]float32{}

	ops := paramtools.NewOrderedParamSet()
	ps := paramtools.ParamSet{}

	for key, value := range traces {
		p, err := query.ParseKey(key)
		assert.NoError(t, err)
		ps.AddParams(p)
		ops.Update(ps)
		encodedKey, err := ops.EncodeParamsAsString(p)
		assert.NoError(t, err)
		ret[encodedKey] = value
	}

	return ops, ret
}

func TestBuilder(t *testing.T) {
	testutils.SmallTest(t)

	traces1 := types.TraceSet{
		",arch=x86,name=foo,": []float32{1.0, 2.0},
		",arch=x86,name=bar,": []float32{3.0, 4.0},
		",arch=x86,name=baz,": []float32{5.0, 6.0},
	}
	ops1, encodedTraces1 := encodeTraces(t, traces1)
	traceMap1 := map[int32]int32{
		0: 0,
		1: 1,
	}

	traces2 := types.TraceSet{
		",arch=x86,name=foo,": []float32{3.3, 4.4},
		",arch=x86,name=bar,": []float32{5.5, 6.6},
		",arch=x86,name=baz,": []float32{7.7, 8.8},
	}
	ops2, encodedTraces2 := encodeTraces(t, traces2)
	traceMap2 := map[int32]int32{
		0: 3,
		1: 4,
	}

	builder := New(5)
	builder.Add(ops1, traceMap1, encodedTraces1)
	builder.Add(ops2, traceMap2, encodedTraces2)
	traceSet, ops := builder.Build(context.Background())
	assert.Len(t, traceSet, 3)
	assert.Len(t, ops, 2)
	assert.Equal(t, traceSet[",arch=x86,name=foo,"], types.Trace{1.0, 2.0, e, 3.3, 4.4})
	assert.Equal(t, traceSet[",arch=x86,name=bar,"], types.Trace{3.0, 4.0, e, 5.5, 6.6})
	assert.Equal(t, traceSet[",arch=x86,name=baz,"], types.Trace{5.0, 6.0, e, 7.7, 8.8})
	expectedParamSet := paramtools.ParamSet{
		"arch": []string{"x86"},
		"name": []string{"bar", "baz", "foo"},
	}
	expectedParamSet.Normalize()
	ops.Normalize()
	assert.Equal(t, expectedParamSet, ops)
}

func TestBuilderEmpty(t *testing.T) {
	testutils.SmallTest(t)

	builder := New(5)
	traceSet, ops := builder.Build(context.Background())
	assert.Len(t, traceSet, 0)
	assert.Len(t, ops, 0)
}
