package scrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFloatSliceToString_ReturnsStringWithFloatLiterals(t *testing.T) {
	test := func(name, expected string, vals []float32) {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, expected, floatSliceToString(vals))
		})
	}
	test("empty", "", []float32{})
	test("one", "1.0f", []float32{1})
	test("two", "1.0f, 2.0f", []float32{1, 2})
}

func TestUniformValueNumFloats_WithValidTypes_ReturnsCorrectCount(t *testing.T) {
	test := func(name string, expected int, u uniformValue) {
		t.Run(name, func(t *testing.T) {
			count, err := u.numFloats()
			assert.NoError(t, err)
			assert.Equal(t, expected, count)
		})
	}
	test("float", 1, uniformValue{Name: "iScalar", Type: "float"})
	test("float2", 2, uniformValue{Name: "iCoord", Type: "float2"})
	test("float3", 3, uniformValue{Name: "iColor", Type: "float3"})
	test("float4", 4, uniformValue{Name: "iColorWithAlpha", Type: "float4"})
	test("float2x2", 4, uniformValue{Name: "iMatrix", Type: "float2x2"})
}

func TestUniformValueNumFloats_WithInvalidTypes_ReturnsError(t *testing.T) {
	test := func(name string, u uniformValue) {
		t.Run(name, func(t *testing.T) {
			_, err := u.numFloats()
			assert.Error(t, err)
		})
	}
	test("TextBeforeFloat", uniformValue{Name: "iValue", Type: "Xfloat"})
	test("TextAfterFloat", uniformValue{Name: "iValue", Type: "float;"})
	test("FloatX", uniformValue{Name: "iValue", Type: "floatx"})
	test("FloatZeroDimension", uniformValue{Name: "iValue", Type: "float0"})
	test("MatrixZeroXDimension", uniformValue{Name: "iValue", Type: "float0x2"})
	test("MatrixZeroYDimension", uniformValue{Name: "iValue", Type: "float2x0"})
}

func TestUniformValueGetCppDefinitionString_WithSupportedTypes_ReturnsValidCPP(t *testing.T) {
	test := func(name, expected string, u uniformValue, vals []float32) {
		t.Run(name, func(t *testing.T) {
			values, err := u.getCppDefinitionString(vals, "Num")
			assert.NoError(t, err)
			assert.Equal(t, expected, values)
		})
	}
	test("float", `builderNum.uniform("iSomeSlider") = 1.0f;`,
		uniformValue{Name: "iSomeSlider", Type: "float"},
		[]float32{1.0},
	)
	test("float2", `builderNum.uniform("iCoordinate") = SkV2{0.5f, 0.6f};`,
		uniformValue{Name: "iCoordinate", Type: "float2"},
		[]float32{0.5, 0.6},
	)
	test("float3", `builderNum.uniform("iColor") = SkV3{0.5f, 0.6f, 0.7f};`,
		uniformValue{Name: "iColor", Type: "float3"},
		[]float32{0.5, 0.6, 0.7},
	)
	test("float4", `builderNum.uniform("iColorWithAlpha") = SkV4{0.5f, 0.6f, 0.7f, 1.0f};`,
		uniformValue{Name: "iColorWithAlpha", Type: "float4"},
		[]float32{0.5, 0.6, 0.7, 1.0},
	)
	test("float2x2", `builderNum.uniform("iMatrix") = SkV4{1.0f, 2.0f, 3.0f, 4.0f};`,
		uniformValue{Name: "iMatrix", Type: "float2x2"},
		[]float32{1, 2, 3, 4},
	)
}

func TestUniformValueGetCppDefinitionString_WithUnsupportedTypes_ReturnsError(t *testing.T) {
	test := func(name string, u uniformValue, vals []float32) {
		t.Run(name, func(t *testing.T) {
			_, err := u.getCppDefinitionString(vals, "")
			assert.Error(t, err)
		})
	}
	test("double",
		uniformValue{Name: "iSomeSlider", Type: "double"},
		[]float32{1.0},
	)
}
