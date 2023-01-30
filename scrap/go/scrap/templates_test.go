package scrap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemplateExpand_SVGToCPP_Success(t *testing.T) {
	tmplMap, err := loadTemplates()
	require.NoError(t, err)
	var b bytes.Buffer
	body := ScrapBody{
		Type: SVG,
		Body: "<svg> \n</svg>",
	}
	err = tmplMap[CPP][SVG].Execute(&b, body)
	require.NoError(t, err)
	expected := `void draw(SkCanvas* canvas) {
    const char * svg =
        "<svg> \n"
        "</svg>";

    sk_sp<SkData> data(SkData::MakeWithoutCopy(svg, strlen(svg)));
    if (!data) {
        SkDebugf("Failed to load SVG.");
        return;
    }

    SkMemoryStream stream(std::move(data));
    sk_sp<SkSVGDOM> svgDom = SkSVGDOM::MakeFromStream(stream);
    if (!svgDom) {
        SkDebugf("Failed to parse SVG.");
        return;
    }

    // Use the intrinsic SVG size if available, otherwise fall back to a default value.
    static const SkSize kDefaultContainerSize = SkSize::Make(128, 128);
    if (svgDom->containerSize().isEmpty()) {
        svgDom->setContainerSize(kDefaultContainerSize);
    }

    svgDom->render(canvas);
}`
	require.Equal(t, expected, b.String())
}

func TestTemplateExpand_SkSLToCPP_ResponseMatchesExpected(t *testing.T) {
	tmplMap, err := loadTemplates()
	require.NoError(t, err)
	var b bytes.Buffer
	body := ScrapBody{
		Type: SKSL,
		Body: "half4 main(in vec2 fragCoord ) {\n    return vec4( result, 1.0 );\n}",
	}
	err = tmplMap[CPP][SKSL].Execute(&b, body)
	require.NoError(t, err)
	expected := `void draw(SkCanvas *canvas) {
    canvas->clear(SK_ColorBLACK);

    constexpr char sksl[] = R"(
        // Inputs supplied by shaders.skia.org:
        uniform float3 iResolution;      // Viewport resolution (pixels)
        uniform float  iTime;            // Shader playback time (s)
        uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
        uniform float3 iImageResolution; // iImage1 resolution (pixels)
        uniform shader iImage1;          // An input image.

        half4 main(in vec2 fragCoord ) {
            return vec4( result, 1.0 );
        }
    )";

    // Parse the SkSL, and create an SkRuntimeEffect object:
    auto [effect, err] = SkRuntimeEffect::MakeForShader(SkString(sksl));
    SkRuntimeShaderBuilder builder(effect);
    builder.uniform("iResolution") =
        SkV3{(float)canvas->imageInfo().width(),
             (float)canvas->imageInfo().height(), 1.0f};
    builder.uniform("iTime") = 1.0f;
    builder.uniform("iMouse") = SkV4{0.0f, 0.0f, 0.0f, 0.0f};
    builder.uniform("iImageResolution") =
        SkV3{(float)image->width(), (float)image->height(), 1.0f};
    builder.child("iImage1") =
        image->makeShader(SkSamplingOptions(SkFilterMode::kLinear));
    sk_sp<SkShader> myShader = builder.makeShader();

    // Fill the surface with |myShader|:
    SkPaint p;
    p.setShader(myShader);
    canvas->drawPaint(p);
}`

	require.Equal(t, expected, b.String())
}

func TestTemplateHelper_bodyAsQuotedStringSlice_ReturnsExpectedSlice(t *testing.T) {
	test := func(name string, expected []string, input string) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, expected, bodyAsQuotedStringSlice(input))
		})
	}
	test("OneLine", []string{`" <svg> ";`}, " <svg> ")
	test("TwoLines", []string{`"<svg> \n"`, `"</svg>";`}, "<svg> \n</svg>")
	test("EmptyBody", []string{`"";`}, "")
}

func TestTemplateHelper_bodyStringSlice_ReturnsExpectedSlice(t *testing.T) {
	test := func(name string, expected []string, input string) {
		t.Run(name, func(t *testing.T) {
			actual := bodyStringSlice(input)
			require.Equal(t, expected, actual)
		})
	}
	test("OneLine", []string{"foo"}, "foo")
	test("TwoLines", []string{"foo ", " bar"}, "foo \n bar")
	test("EmptyBody", []string{""}, "")
}
