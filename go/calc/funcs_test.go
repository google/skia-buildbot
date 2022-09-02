package calc

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/types"
)

func TestStdDevFuncImpl_WellKnownStdDeviations_Success(t *testing.T) {
	// Via the example on https://en.wikipedia.org/wiki/Standard_deviation we
	// know that {2, 4, 4, 4, 5, 5, 7, 9} has a stddev of 2.
	//
	// Values that don't ever change, ala {2,2,2,..} will have stddev of 0.
	tr := StdDevFuncImpl(types.TraceSet{
		"a": []float32{e, 2, 1, 2},
		"b": []float32{e, 4, 1, 2},
		"c": []float32{e, 4, 1, 2},
		"d": []float32{e, 4, 1, 2},
		"e": []float32{e, 5, 1, 2},
		"f": []float32{e, 5, 1, 2},
		"g": []float32{e, 7, 1, 2},
		"h": []float32{e, 9, 1, 2},
	})
	assert.Equal(t, types.Trace{e, 2, 0, 0}, tr)
}

func TestStdDevFuncImpl_EmptyTraceSet_ReturnsEmptyTrace(t *testing.T) {
	assert.Equal(t, types.Trace{}, StdDevFuncImpl(types.TraceSet{}))
}

func TestMaxFuncImpl(t *testing.T) {
	tr := MaxFuncImpl(types.TraceSet{
		"a": []float32{e, 0, 1, 2},
		"b": []float32{e, e, 1, 3},
	})
	assert.Equal(t, types.Trace{-math.MaxFloat32, 0, 1, 3}, tr)
}

func TestMaxFuncImpl_EmptyTraceSet_ReturnsEmptyTrace(t *testing.T) {
	assert.Equal(t, types.Trace{}, MaxFuncImpl(types.TraceSet{}))
}

func TestMinFuncImpl(t *testing.T) {
	tr := MinFuncImpl(types.TraceSet{
		"a": []float32{e, 0, 1, 2},
		"b": []float32{e, e, 1, 3},
	})
	assert.Equal(t, types.Trace{math.MaxFloat32, 0, 1, 2}, tr)
}

func TestMinFuncImpl_EmptyTraceSet_ReturnsEmptyTrace(t *testing.T) {
	assert.Equal(t, types.Trace{}, MinFuncImpl(types.TraceSet{}))
}
