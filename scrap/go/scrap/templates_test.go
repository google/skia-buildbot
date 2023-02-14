package scrap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestTemplateExpand_SkSLToCPPWithMetadata_ResponseMatchesExpected(t *testing.T) {
	tmplMap, err := loadTemplates()
	require.NoError(t, err)
	var b bytes.Buffer
	body := ScrapBody{
		Type:         SKSL,
		Body:         "// Inputs supplied by user:\nuniform float iSomeSlider;\n\nhalf4 main(in vec2 fragCoord ) {\n    return vec4( result, iSomeSlider );\n}",
		SKSLMetaData: &SKSLMetaData{Uniforms: []float32{0.5}},
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

        // Inputs supplied by user:
        uniform float iSomeSlider;

        half4 main(in vec2 fragCoord ) {
            return vec4( result, iSomeSlider );
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
    // Inputs supplied by user:
    builder.uniform("iSomeSlider") = 0.5;

    sk_sp<SkShader> myShader = builder.makeShader();

    // Fill the surface with |myShader|:
    SkPaint p;
    p.setShader(myShader);
    canvas->drawPaint(p);
}`

	require.Equal(t, expected, b.String())
}

func TestTemplateExpand_SkSLToJavaScript_ResponseMatchesExpected(t *testing.T) {
	tmplMap, err := loadTemplates()
	require.NoError(t, err)
	var b bytes.Buffer
	body := ScrapBody{
		Type: SKSL,
		Body: "half4 main(in vec2 fragCoord ) {\n    return vec4( result, 1.0 );\n}",
	}
	err = tmplMap[JS][SKSL].Execute(&b, body)
	require.NoError(t, err)
	expected := `const loadImage = fetch("https://shaders.skia.org/img/mandrill.png")
  .then((response) => response.arrayBuffer());

Promise.all([loadImage]).then((values) => {
  const [imageData] = values;
  const img = CanvasKit.MakeImageFromEncoded(imageData);
  const imgShader = img.makeShaderCubic(
    CanvasKit.TileMode.Clamp, CanvasKit.TileMode.Clamp, 1 / 3, 1 / 3);

  const surface = CanvasKit.MakeCanvasSurface(canvas.id);
  if (!surface) {
    throw "Could not make surface";
  }
  const skcanvas = surface.getCanvas();
  const paint = new CanvasKit.Paint();
  const startTimeMs = Date.now();
  let mouseClickX = 250;
  let mouseClickY = 250;
  let mouseDragX = 250;
  let mouseDragY = 250;
  let lastMousePressure = 0;

  const prog = ` + "`" + `
    // Inputs supplied by shaders.skia.org:
    uniform float3 iResolution;      // Viewport resolution (pixels)
    uniform float  iTime;            // Shader playback time (s)
    uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
    uniform float3 iImageResolution; // iImage1 resolution (pixels)
    uniform shader iImage1;          // An input image.

    half4 main(in vec2 fragCoord ) {
        return vec4( result, 1.0 );
    }
    ` + "`" + `;

  const effect = CanvasKit.RuntimeEffect.Make(prog);

  function drawFrame(canvas) {
    const uniforms = [
      512, 512, 1,                                      // iResolution
      (Date.now() - startTimeMs) / 1000,                // iTime
      mouseDragX, mouseDragY, mouseClickX, mouseClickY, // iMouse
      img.width(), img.height(), 1,                     // iImageResolution

    ];
    const children = [
      imgShader                                         // iImage1
    ];
    const shader = effect.makeShaderWithChildren(uniforms, children);
    paint.setShader(shader);
    skcanvas.drawPaint(paint);
    shader.delete();
    surface.requestAnimationFrame(drawFrame);
  }
  surface.requestAnimationFrame(drawFrame);

  canvas.addEventListener("pointermove", (e) => {
    if (e.pressure && !lastMousePressure) {
      mouseClickX = e.offsetX;
      mouseClickY = e.offsetY;
    }
    lastMousePressure = e.pressure;
    if (!e.pressure) {
      return;
    }
    mouseDragX = e.offsetX;
    mouseDragY = e.offsetY;
  });
}); // from the Promise.all
`

	require.Equal(t, expected, b.String())
}

func TestTemplateExpand_SkSLWithMetadataToJavaScript_ResponseIncludesMetadata(t *testing.T) {
	tmplMap, err := loadTemplates()
	require.NoError(t, err)
	var b bytes.Buffer
	body := ScrapBody{
		Type:         SKSL,
		Body:         "half4 main(in vec2 fragCoord ) {\n    uniform float  iSomeSlider;\n    return vec4( result, iSomeSlider );\n}",
		SKSLMetaData: &SKSLMetaData{Uniforms: []float32{0.5}},
	}
	err = tmplMap[JS][SKSL].Execute(&b, body)
	require.NoError(t, err)
	expected := `const loadImage = fetch("https://shaders.skia.org/img/mandrill.png")
  .then((response) => response.arrayBuffer());

Promise.all([loadImage]).then((values) => {
  const [imageData] = values;
  const img = CanvasKit.MakeImageFromEncoded(imageData);
  const imgShader = img.makeShaderCubic(
    CanvasKit.TileMode.Clamp, CanvasKit.TileMode.Clamp, 1 / 3, 1 / 3);

  const surface = CanvasKit.MakeCanvasSurface(canvas.id);
  if (!surface) {
    throw "Could not make surface";
  }
  const skcanvas = surface.getCanvas();
  const paint = new CanvasKit.Paint();
  const startTimeMs = Date.now();
  let mouseClickX = 250;
  let mouseClickY = 250;
  let mouseDragX = 250;
  let mouseDragY = 250;
  let lastMousePressure = 0;

  const prog = ` + "`" + `
    // Inputs supplied by shaders.skia.org:
    uniform float3 iResolution;      // Viewport resolution (pixels)
    uniform float  iTime;            // Shader playback time (s)
    uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
    uniform float3 iImageResolution; // iImage1 resolution (pixels)
    uniform shader iImage1;          // An input image.

    half4 main(in vec2 fragCoord ) {
        uniform float  iSomeSlider;
        return vec4( result, iSomeSlider );
    }
    ` + "`" + `;

  const effect = CanvasKit.RuntimeEffect.Make(prog);

  function drawFrame(canvas) {
    const uniforms = [
      512, 512, 1,                                      // iResolution
      (Date.now() - startTimeMs) / 1000,                // iTime
      mouseDragX, mouseDragY, mouseClickX, mouseClickY, // iMouse
      img.width(), img.height(), 1,                     // iImageResolution

      // User supplied uniform values:
      0.5
    ];
    const children = [
      imgShader                                         // iImage1
    ];
    const shader = effect.makeShaderWithChildren(uniforms, children);
    paint.setShader(shader);
    skcanvas.drawPaint(paint);
    shader.delete();
    surface.requestAnimationFrame(drawFrame);
  }
  surface.requestAnimationFrame(drawFrame);

  canvas.addEventListener("pointermove", (e) => {
    if (e.pressure && !lastMousePressure) {
      mouseClickX = e.offsetX;
      mouseClickY = e.offsetY;
    }
    lastMousePressure = e.pressure;
    if (!e.pressure) {
      return;
    }
    mouseDragX = e.offsetX;
    mouseDragY = e.offsetY;
  });
}); // from the Promise.all
`

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

func TestTemplateHelper_indentMultilineString_ReturnsExpectedIndent(t *testing.T) {
	test := func(name, expected, input string, indent int) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, expected, indentMultilineString(input, indent))
		})
	}
	test("OneLine", "    foo", "foo", 4)
	test("TwoLines", "    foo\n    bar", "foo\nbar", 4)
	test("OneLineTrailingWhitespace", "    foo", "foo\t", 4)
	test("MultilineOneEmptyLine", "    foo\n\n    bar", "foo\n\nbar", 4)
}

func TestGetSkSLImageURL_WithValidPathsAndURLs_ReturnsObjectsShaderURL(t *testing.T) {
	test := func(name, expected string, body ScrapBody) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, expected, getSkSLImageURL(body))
		})
	}
	test("DistNotModified", "https://shaders.skia.org/dist/soccer.png",
		ScrapBody{
			Type:         SKSL,
			Body:         "",
			SKSLMetaData: &SKSLMetaData{ImageURL: "/dist/soccer.png"}})
	test("RelativeImgUnicode", "https://shaders.skia.org/img/世界.png",
		ScrapBody{
			Type:         SKSL,
			Body:         "",
			SKSLMetaData: &SKSLMetaData{ImageURL: "/img/世界.png"}})
	test("NonRelativeURL", "https://example.com/my_image.png",
		ScrapBody{
			Type:         SKSL,
			Body:         "",
			SKSLMetaData: &SKSLMetaData{ImageURL: "https://example.com/my_image.png"}})
}

func TestGetSkSLImageURL_WithInvalidSkSLScrap_ReturnsDefaultShaderURL(t *testing.T) {
	test := func(name, expected string, body ScrapBody) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, expected, getSkSLImageURL(body))
		})
	}
	test("NotSkSL", "https://shaders.skia.org/img/mandrill.png",
		ScrapBody{
			Type:              Particle,
			Body:              "",
			ParticlesMetaData: &ParticlesMetaData{}})
	test("NilSKSLMetaData", "https://shaders.skia.org/img/mandrill.png",
		ScrapBody{
			Type: SKSL,
			Body: ""})
	test("EmptyImageURL", "https://shaders.skia.org/img/mandrill.png",
		ScrapBody{
			Type:         SKSL,
			SKSLMetaData: &SKSLMetaData{ImageURL: ""}})
}

func TestGetSkSLCustomUniforms_WithUniformValues_ReturnsCorrectStringForm(t *testing.T) {
	test := func(name, expected string, body ScrapBody) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, expected, getSkSLCustomUniforms(body))
		})
	}
	test("OneValue", "\n      // User supplied uniform values:\n      0.1",
		ScrapBody{
			Type:         SKSL,
			Body:         "",
			SKSLMetaData: &SKSLMetaData{Uniforms: []float32{0.1}}})
	test("TwoValues", "\n      // User supplied uniform values:\n      0.1, 0.2",
		ScrapBody{
			Type:         SKSL,
			Body:         "",
			SKSLMetaData: &SKSLMetaData{Uniforms: []float32{0.1, 0.2}}})
}

func TestGetSkSLCustomUniforms_WithoutUniformValues_ReturnsEmptyString(t *testing.T) {
	test := func(name string, body ScrapBody) {
		t.Run(name, func(t *testing.T) {
			require.Empty(t, getSkSLCustomUniforms(body))
		})
	}
	test("NotSkSL",
		ScrapBody{
			Type:              Particle,
			Body:              "",
			ParticlesMetaData: &ParticlesMetaData{}})
	test("NilSKSLMetaData",
		ScrapBody{
			Type: SKSL,
			Body: ""})
	test("NoUniformValues",
		ScrapBody{
			Type:         SKSL,
			SKSLMetaData: &SKSLMetaData{Uniforms: []float32{}}})
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
			values, err := u.getCppDefinitionString(vals)
			assert.NoError(t, err)
			assert.Equal(t, expected, values)
		})
	}
	test("float", `builder.uniform("iSomeSlider") = 1;`,
		uniformValue{Name: "iSomeSlider", Type: "float"},
		[]float32{1.0},
	)
	test("float2", `builder.uniform("iCoordinate") = SkV2{0.5, 0.6};`,
		uniformValue{Name: "iCoordinate", Type: "float2"},
		[]float32{0.5, 0.6},
	)
	test("float3", `builder.uniform("iColor") = SkV3{0.5, 0.6, 0.7};`,
		uniformValue{Name: "iColor", Type: "float3"},
		[]float32{0.5, 0.6, 0.7},
	)
	test("float4", `builder.uniform("iColorWithAlpha") = SkV4{0.5, 0.6, 0.7, 1};`,
		uniformValue{Name: "iColorWithAlpha", Type: "float4"},
		[]float32{0.5, 0.6, 0.7, 1.0},
	)
	test("float2x2", `builder.uniform("iMatrix") = SkV4{1, 2, 3, 4};`,
		uniformValue{Name: "iMatrix", Type: "float2x2"},
		[]float32{1, 2, 3, 4},
	)
}

func TestUniformValueGetCppDefinitionString_WithUnsupportedTypes_ReturnsError(t *testing.T) {
	test := func(name, expected string, u uniformValue, vals []float32) {
		t.Run(name, func(t *testing.T) {
			_, err := u.getCppDefinitionString(vals)
			assert.Error(t, err)
		})
	}
	test("float", `builder.uniform("iSomeSlider") = 1;`,
		uniformValue{Name: "iSomeSlider", Type: "double"},
		[]float32{1.0},
	)
}

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

func TestCppCustomUniformValues_WithUniformValue_ReturnsCPPCode(t *testing.T) {
	test := func(name, expected string, body ScrapBody) {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, expected, cppCustomUniformValues(body))
		})
	}
	test("OneFloat", `    // Inputs supplied by user:
    builder.uniform("iSomeSlider") = 0.5;
`,
		ScrapBody{
			Type: SKSL,
			Body: `
uniform float  iSomeSlider;
half4 main(float2 fragCoord) {
  return half4(iColorWithAlpha);
}`,
			SKSLMetaData: &SKSLMetaData{Uniforms: []float32{0.5}},
		})
}
