package scrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractBodyUniforms_WithValidUniforms_ReturnsExtractedValues(t *testing.T) {
	test := func(name string, expected []uniformValue, body ScrapBody) {
		t.Run(name, func(t *testing.T) {
			values, err := extractBodyUniforms(body)
			assert.NoError(t, err)
			assert.Equal(t, expected, values)
		})
	}
	test("SingleFloat", []uniformValue{{Name: "iSomeSlider", Type: "float"}},
		ScrapBody{
			Type: SKSL,
			Body: `
uniform float  iSomeSlider;

half4 main(float2 fragCoord) {
  return half4(iColorWithAlpha);
}`,
		})
	test("MultipleUniforms", []uniformValue{
		{Name: "iSomeSlider", Type: "float"},
		{Name: "iColor", Type: "float3"},
		{Name: "iColorWithAlpha", Type: "float4"},
		{Name: "iSomeCoordinate", Type: "float2"},
		{Name: "iMatrix", Type: "float2x2"},
	},
		ScrapBody{
			Type: SKSL,
			Body: `
uniform float  iSomeSlider;

// A float3 with 'color' in the name
// (case insensitive) will have a color picker control.
uniform float3 iColor;

// A float4 with 'color' in the name will also have a
// slider for the alpha channel.
uniform float4 iColorWithAlpha;

// uniforms of any other size and shape will have
// a table of inputs as a control.
uniform float2 iSomeCoordinate;
uniform float2x2 iMatrix;

half4 main(float2 fragCoord) {
  return half4(iColorWithAlpha);
}`,
		})
}

func TestExtractBodyUniforms_WithoutUniformDefinitions_ReturnsNilSlice(t *testing.T) {
	test := func(name string, body ScrapBody) {
		t.Run(name, func(t *testing.T) {
			values, err := extractBodyUniforms(body)
			assert.NoError(t, err)
			assert.Nil(t, values)
		})
	}
	test("NoUniforms",
		ScrapBody{
			Type: SKSL,
			Body: `
half4 main(float2 fragCoord) {
  return half4(iColorWithAlpha);
}`,
		})
	test("CommentedOutUniform",
		ScrapBody{
			Type: SKSL,
			Body: `
// uniform float  iSomeSlider;
half4 main(float2 fragCoord) {
  return half4(iColorWithAlpha);
}`,
		})
	test("NoUniformName",
		ScrapBody{
			Type: SKSL,
			Body: `
uniform float ;
half4 main(float2 fragCoord) {
  return half4(iColorWithAlpha);
}`,
		})
	test("ExtraSemicolon",
		ScrapBody{
			Type: SKSL,
			Body: `
uniform float iSomeSlider;;

half4 main(float2 fragCoord) {
  return half4(iColorWithAlpha);
}`,
		})
}
