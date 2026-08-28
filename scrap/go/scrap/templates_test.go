package scrap

import (
	"bytes"
	"strings"
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
	err = tmplMap[CPP][SVG].Execute(&b, scrapNode{Name: "Test", Scrap: body})
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
	err = tmplMap[CPP][SKSL].Execute(&b, scrapNode{Scrap: body})
	require.NoError(t, err)
	expected := `void draw(SkCanvas *canvas) {
    constexpr SkV4 mousePos = SkV4{0.0f, 0.0f, 0.0f, 0.0f};
    constexpr SkV3 viewportResolution = SkV3{256, 256, 1.0f};
    const SkSamplingOptions shaderOptions(SkFilterMode::kLinear);
    const float playbackTime = duration != 0.0 ? frame * duration : 0.0;

    constexpr char prog[] = R"SkSLScrap(
        // Inputs supplied by shaders.skia.org:
        uniform float3 iResolution;      // Viewport resolution (pixels)
        uniform float  iTime;            // Shader playback time (s)
        uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
        uniform float3 iImageResolution; // iImage1 resolution (pixels)
        uniform shader iImage1;          // An input image.

        half4 main(in vec2 fragCoord ) {
            return vec4( result, 1.0 );
        }
    )SkSLScrap";
    auto [effect, err] = SkRuntimeEffect::MakeForShader(SkString(prog));
    if (!effect) {
        SkDebugf("Cannot create effect");
        return;
    }
    SkRuntimeShaderBuilder builder(effect);
    builder.uniform("iResolution") = viewportResolution;
    builder.uniform("iTime") = playbackTime;
    builder.uniform("iMouse") = mousePos;
    builder.uniform("iImageResolution") =
        SkV3{static_cast<float>(image->width()),
             static_cast<float>(image->height()), 1.0f};
    builder.child("iImage1") = image->makeShader(shaderOptions);
    sk_sp<SkShader> shader = builder.makeShader();
    if (!shader) {
        SkDebugf("Cannot create shader");
        return;
    }

    canvas->clear(SK_ColorBLACK);

    // Fill the surface with |shader|:
    SkPaint p;
    p.setShader(shader);
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
	err = tmplMap[CPP][SKSL].Execute(&b, scrapNode{Name: "Test", Scrap: body})
	require.NoError(t, err)
	expected := `void draw(SkCanvas *canvas) {
    constexpr SkV4 mousePos = SkV4{0.0f, 0.0f, 0.0f, 0.0f};
    constexpr SkV3 viewportResolution = SkV3{256, 256, 1.0f};
    const SkSamplingOptions shaderOptions(SkFilterMode::kLinear);
    const float playbackTime = duration != 0.0 ? frame * duration : 0.0;

    // Shader "Test":
    constexpr char progTest[] = R"SkSLScrap(
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
    )SkSLScrap";
    auto [effectTest, errTest] = SkRuntimeEffect::MakeForShader(SkString(progTest));
    if (!effectTest) {
        SkDebugf("Cannot create effectTest");
        return;
    }
    SkRuntimeShaderBuilder builderTest(effectTest);
    builderTest.uniform("iResolution") = viewportResolution;
    builderTest.uniform("iTime") = playbackTime;
    builderTest.uniform("iMouse") = mousePos;
    builderTest.uniform("iImageResolution") =
        SkV3{static_cast<float>(image->width()),
             static_cast<float>(image->height()), 1.0f};
    builderTest.child("iImage1") = image->makeShader(shaderOptions);
    // Inputs supplied by user:
    builderTest.uniform("iSomeSlider") = 0.5f;
    sk_sp<SkShader> shaderTest = builderTest.makeShader();
    if (!shaderTest) {
        SkDebugf("Cannot create shaderTest");
        return;
    }

    canvas->clear(SK_ColorBLACK);

    // Fill the surface with |shaderTest|:
    SkPaint p;
    p.setShader(shaderTest);
    canvas->drawPaint(p);
}`

	require.Equal(t, expected, b.String())
}

// Test a simple scrap with no child shaders or custom uniform values.
func TestTemplateExpand_SkSLToJavaScript_ResponseMatchesExpected(t *testing.T) {
	tmplMap, err := loadTemplates()
	require.NoError(t, err)
	var b bytes.Buffer
	body := ScrapBody{
		Type: SKSL,
		Body: "half4 main(in vec2 fragCoord ) {\n    return vec4( result, 1.0 );\n}",
	}
	// Create a scrap node with no name to test simpler variable names
	err = tmplMap[JS][SKSL].Execute(&b, scrapNode{Scrap: body})
	require.NoError(t, err)
	expected := `const shaderWidth = 512;
const shaderHeight = 512;
const loadImage1 = fetch("https://shaders.skia.org/img/mandrill.png")
  .then((response) => response.arrayBuffer());

Promise.all([loadImage1]).then((values) => {
  const [imageData1] = values;
  const img1 = CanvasKit.MakeImageFromEncoded(imageData1);
  const imgShader1 = img1.makeShaderCubic(
    CanvasKit.TileMode.Clamp, CanvasKit.TileMode.Clamp, 1/3, 1/3);

  const surface = CanvasKit.MakeCanvasSurface(canvas.id);
  if (!surface) {
    throw "Could not make surface";
  }
  const skcanvas = surface.getCanvas();
  const paint = new CanvasKit.Paint();
  const startTimeMs = Date.now();
  let mouseClickX = 0;
  let mouseClickY = 0;
  let mouseDragX = 0;
  let mouseDragY = 0;
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
    const iTime = (Date.now() - startTimeMs) / 1000;
    const uniforms = [
      shaderWidth, shaderHeight, 1,                     // iResolution
      iTime,                                            // iTime
      mouseDragX, mouseDragY, mouseClickX, mouseClickY, // iMouse
      img1.width(), img1.height(), 1,                   // iImageResolution
    ];
    const children = [
      imgShader1,                                       // iImage1
    ];
    const shader = effect.makeShaderWithChildren(uniforms, children);
    if (!shader) {
      throw "Could not make shader";
    }
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

// Test a more complex shader with custom uniforms and child shaders
func TestTemplateExpand_SkSLWithChildNodesAndMetadata_ResponseIncludesMetadata(t *testing.T) {
	tmplMap, err := loadTemplates()
	require.NoError(t, err)
	var b bytes.Buffer
	childA := scrapNode{
		Name: "A",
		Scrap: ScrapBody{
			Type:         SKSL,
			Body:         "uniform float  iValueA;\n\nhalf4 main(in vec2 fragCoord ) {\n    return vec4( result, iValueA );\n}",
			SKSLMetaData: &SKSLMetaData{Uniforms: []float32{0.1}, ImageURL: "https://example.com/A.png"},
		},
	}
	childB := scrapNode{
		Name: "B",
		Scrap: ScrapBody{
			Type:         SKSL,
			Body:         "uniform float  iValueB;\n\nhalf4 main(in vec2 fragCoord ) {\n    return vec4( result, iValueB );\n}",
			SKSLMetaData: &SKSLMetaData{Uniforms: []float32{0.2}, ImageURL: "https://example.com/B.png"},
		},
	}
	rootNode := scrapNode{
		Name: "Root",
		Scrap: ScrapBody{
			Type: SKSL,
			Body: `uniform float  iRootVal;

half4 main(in vec2 fragCoord ) {
    return vec4( result, iRootVal );
}`,
			SKSLMetaData: &SKSLMetaData{
				Uniforms: []float32{0.3},
				ImageURL: "https://example.com/A.png", // <- Second use of A.png
				Children: []ChildShader{
					{UniformName: "childA", ScrapHashOrName: "unused"},
					{UniformName: "childB", ScrapHashOrName: "unused"},
				},
			},
		},
		Children: []scrapNode{childA, childB},
	}
	err = tmplMap[JS][SKSL].Execute(&b, rootNode)
	require.NoError(t, err)
	expected := `const shaderWidth = 512;
const shaderHeight = 512;
const loadImage1 = fetch("https://example.com/A.png")
  .then((response) => response.arrayBuffer());
const loadImage2 = fetch("https://example.com/B.png")
  .then((response) => response.arrayBuffer());

Promise.all([loadImage1, loadImage2]).then((values) => {
  const [imageData1, imageData2] = values;
  const img1 = CanvasKit.MakeImageFromEncoded(imageData1);
  const imgShader1 = img1.makeShaderCubic(
    CanvasKit.TileMode.Clamp, CanvasKit.TileMode.Clamp, 1/3, 1/3);
  const img2 = CanvasKit.MakeImageFromEncoded(imageData2);
  const imgShader2 = img2.makeShaderCubic(
    CanvasKit.TileMode.Clamp, CanvasKit.TileMode.Clamp, 1/3, 1/3);

  const surface = CanvasKit.MakeCanvasSurface(canvas.id);
  if (!surface) {
    throw "Could not make surface";
  }
  const skcanvas = surface.getCanvas();
  const paint = new CanvasKit.Paint();
  const startTimeMs = Date.now();
  let mouseClickX = 0;
  let mouseClickY = 0;
  let mouseDragX = 0;
  let mouseDragY = 0;
  let lastMousePressure = 0;

  // Shader "A"
  const progA = ` + "`" + `
    // Inputs supplied by shaders.skia.org:
    uniform float3 iResolution;      // Viewport resolution (pixels)
    uniform float  iTime;            // Shader playback time (s)
    uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
    uniform float3 iImageResolution; // iImage1 resolution (pixels)
    uniform shader iImage1;          // An input image.

    uniform float  iValueA;

    half4 main(in vec2 fragCoord ) {
        return vec4( result, iValueA );
    }
    ` + "`" + `;
  const effectA = CanvasKit.RuntimeEffect.Make(progA);

  // Shader "B"
  const progB = ` + "`" + `
    // Inputs supplied by shaders.skia.org:
    uniform float3 iResolution;      // Viewport resolution (pixels)
    uniform float  iTime;            // Shader playback time (s)
    uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
    uniform float3 iImageResolution; // iImage1 resolution (pixels)
    uniform shader iImage1;          // An input image.

    uniform float  iValueB;

    half4 main(in vec2 fragCoord ) {
        return vec4( result, iValueB );
    }
    ` + "`" + `;
  const effectB = CanvasKit.RuntimeEffect.Make(progB);

  // Shader "Root"
  const progRoot = ` + "`" + `
    // Inputs supplied by shaders.skia.org:
    uniform float3 iResolution;      // Viewport resolution (pixels)
    uniform float  iTime;            // Shader playback time (s)
    uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
    uniform float3 iImageResolution; // iImage1 resolution (pixels)
    uniform shader iImage1;          // An input image.
    uniform shader childA;
    uniform shader childB;

    uniform float  iRootVal;

    half4 main(in vec2 fragCoord ) {
        return vec4( result, iRootVal );
    }
    ` + "`" + `;
  const effectRoot = CanvasKit.RuntimeEffect.Make(progRoot);

  function drawFrame(canvas) {
    const iTime = (Date.now() - startTimeMs) / 1000;
    const uniformsA = [
      shaderWidth, shaderHeight, 1,                     // iResolution
      iTime,                                            // iTime
      mouseDragX, mouseDragY, mouseClickX, mouseClickY, // iMouse
      img1.width(), img1.height(), 1,                   // iImageResolution
      // User supplied uniform values:
      0.1
    ];
    const childrenA = [
      imgShader1,                                       // iImage1
    ];
    const shaderA = effectA.makeShaderWithChildren(uniformsA, childrenA);
    if (!shaderA) {
      throw "Could not make shaderA";
    }
    const uniformsB = [
      shaderWidth, shaderHeight, 1,                     // iResolution
      iTime,                                            // iTime
      mouseDragX, mouseDragY, mouseClickX, mouseClickY, // iMouse
      img2.width(), img2.height(), 1,                   // iImageResolution
      // User supplied uniform values:
      0.2
    ];
    const childrenB = [
      imgShader2,                                       // iImage1
    ];
    const shaderB = effectB.makeShaderWithChildren(uniformsB, childrenB);
    if (!shaderB) {
      throw "Could not make shaderB";
    }
    const uniformsRoot = [
      shaderWidth, shaderHeight, 1,                     // iResolution
      iTime,                                            // iTime
      mouseDragX, mouseDragY, mouseClickX, mouseClickY, // iMouse
      img1.width(), img1.height(), 1,                   // iImageResolution
      // User supplied uniform values:
      0.3
    ];
    const childrenRoot = [
      imgShader1,                                       // iImage1
      shaderA,
      shaderB,
    ];
    const shaderRoot = effectRoot.makeShaderWithChildren(uniformsRoot, childrenRoot);
    if (!shaderRoot) {
      throw "Could not make shaderRoot";
    }
    paint.setShader(shaderRoot);
    skcanvas.drawPaint(paint);
    shaderA.delete();
    shaderB.delete();
    shaderRoot.delete();
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
			url, err := getSkSLImageURL(body)
			assert.NoError(t, err)
			require.Equal(t, expected, url)
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

func TestGetSkSLImageURL_NoScrapURL_ReturnsDefaultShaderURL(t *testing.T) {
	test := func(name, expected string, body ScrapBody) {
		t.Run(name, func(t *testing.T) {
			url, err := getSkSLImageURL(body)
			assert.NoError(t, err)
			require.Equal(t, expected, url)
		})
	}
	test("NilSKSLMetaData", "https://shaders.skia.org/img/mandrill.png",
		ScrapBody{
			Type: SKSL,
			Body: ""})
	test("EmptyImageURL", "https://shaders.skia.org/img/mandrill.png",
		ScrapBody{
			Type:         SKSL,
			SKSLMetaData: &SKSLMetaData{ImageURL: ""}})
}

func TestGetSkSLImageURL_WithInvalidSkSLScrap_ReturnsError(t *testing.T) {
	test := func(name string, body ScrapBody) {
		t.Run(name, func(t *testing.T) {
			_, err := getSkSLImageURL(body)
			require.Error(t, err)
		})
	}
	test("NotSkSL",
		ScrapBody{
			Type:              Particle,
			Body:              "",
			ParticlesMetaData: &ParticlesMetaData{}})
}

func TestGetSkSLCustomUniforms_WithUniformValues_ReturnsCorrectStringForm(t *testing.T) {
	test := func(name, expected string, body ScrapBody) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, expected, getSkSLCustomUniforms(body))
		})
	}
	test("OneValue", "      // User supplied uniform values:\n      0.1",
		ScrapBody{
			Type:         SKSL,
			Body:         "",
			SKSLMetaData: &SKSLMetaData{Uniforms: []float32{0.1}}})
	test("TwoValues", "      // User supplied uniform values:\n      0.1, 0.2",
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

func TestWriteCustomUniformSetup_WithUniformValue_ReturnsCPPCode(t *testing.T) {
	test := func(name, expected string, node scrapNode) {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			err := writeCustomUniformSetup(&b, node)
			assert.NoError(t, err)
			assert.Equal(t, expected, b.String())
		})
	}
	test("OneFloat", `    // Inputs supplied by user:
    builder.uniform("iSomeSlider") = 0.5f;
`,
		scrapNode{
			Scrap: ScrapBody{
				Type: SKSL,
				Body: `
uniform float  iSomeSlider;
half4 main(float2 fragCoord) {
  return half4(iColorWithAlpha);
}`,
				SKSLMetaData: &SKSLMetaData{Uniforms: []float32{0.5}},
			}})
}

func assertInRawString(t *testing.T, code string, token string) {
	start := strings.Index(code, `R"SkSLScrap(`)
	require.NotEqual(t, -1, start, "Could not find raw string start")
	start += len(`R"SkSLScrap(`)

	end := strings.Index(code[start:], `)SkSLScrap"`)
	require.NotEqual(t, -1, end, "Could not find raw string end")
	end += start

	searchStart := 0
	foundAny := false
	for {
		idx := strings.Index(code[searchStart:], token)
		if idx == -1 {
			break
		}
		foundAny = true
		actualIdx := searchStart + idx
		assert.True(t, actualIdx >= start && actualIdx+len(token) <= end,
			"Vulnerability: %s at index %d is outside the raw string literal boundaries!", token, actualIdx)
		searchStart = actualIdx + len(token)
	}
	require.True(t, foundAny, "Token %q not found in code", token)
}

func assertInDoubleQuotedString(t *testing.T, code string, token string) {
	foundAny := false
	for _, line := range strings.Split(code, "\n") {
		if strings.Contains(line, "builder") && strings.Contains(line, ".child(") {
			firstQuote := strings.Index(line, `"`)
			lastQuote := strings.LastIndex(line, `"`)
			require.True(t, firstQuote != -1 && lastQuote != -1 && firstQuote < lastQuote)

			searchStart := 0
			for {
				idx := strings.Index(line[searchStart:], token)
				if idx == -1 {
					break
				}
				foundAny = true
				actualIdx := searchStart + idx
				assert.True(t, actualIdx > firstQuote && actualIdx+len(token) <= lastQuote,
					"Vulnerability: %s broke out of the double-quoted string literal on line: %s", token, line)
				searchStart = actualIdx + len(token)
			}
		}
	}
	require.True(t, foundAny, "Could not find line containing token %q", token)
}

func assertInSVGLineQuotes(t *testing.T, block string, token string) {
	foundAny := false
	for _, line := range strings.Split(block, "\n") {
		if strings.Contains(line, token) {
			firstQuote := strings.Index(line, `"`)
			lastQuote := strings.LastIndex(line, `"`)
			require.True(t, firstQuote != -1 && lastQuote != -1 && firstQuote < lastQuote, "Quotes not found or mismatched on line: %s", line)

			searchStart := 0
			for {
				idx := strings.Index(line[searchStart:], token)
				if idx == -1 {
					break
				}
				foundAny = true
				actualIdx := searchStart + idx
				assert.True(t, actualIdx > firstQuote && actualIdx+len(token) <= lastQuote,
					"Vulnerability: %s broke out of standard quotes in line: %s", token, line)
				searchStart = actualIdx + len(token)
			}
		}
	}
	require.True(t, foundAny, "Could not find any line containing token %q", token)
}

func TestTemplateExpand_SkSLToCPP_RawStringBreakout_Reproduction(t *testing.T) {
	tmplMap, err := loadTemplates()
	require.NoError(t, err)

	testCases := []struct {
		name          string
		body          string
		token         string
		expectError   bool
		expectedError string
	}{
		{
			name:  "raw string breakout",
			body:  `half4 main() { return half4(1); })" ; int injected_var = 123; // R"(`,
			token: "injected_var",
		},
		{
			name:  "raw string terminator without payload is allowed and trapped",
			body:  `half4 main() { return half4(1); })" injected_var`,
			token: "injected_var",
		},
		{
			name:          "custom delimiter sequence is rejected with error",
			body:          `half4 main() { return half4(1); })SkSLScrap" ; int injected_var = 123; //`,
			token:         "injected_var",
			expectError:   true,
			expectedError: "SkSL body may not contain the raw-string delimiter sequence",
		},
		{
			name:  "partial custom delimiter sequence is allowed and trapped",
			body:  `half4 main() { return half4(1); })SkSLScrap ; int injected_var = 123;`,
			token: "injected_var",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := ScrapBody{
				Type: SKSL,
				Body: tc.body,
			}
			var b bytes.Buffer
			err = tmplMap[CPP][SKSL].Execute(&b, scrapNode{Scrap: body})
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
				assertInRawString(t, b.String(), tc.token)
			}
		})
	}
}

func TestBodyAsQuotedStringSlice_BackslashQuoteBreakout_Reproduction(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		token string
	}{
		{
			name:  "backslash quote breakout",
			input: `<svg attr="\\\"; int injected_svg_var = 123; //">`,
			token: "injected_svg_var",
		},
		{
			name:  "single backslash followed by quote",
			input: `<svg attr="\"; int injected_svg_var = 123; //">`,
			token: "injected_svg_var",
		},
		{
			name:  "multiple backslashes and quotes",
			input: `\\\\\" injected_svg_var`,
			token: "injected_svg_var",
		},
		{
			name:  "trailing backslash on last line",
			input: `injected_svg_var \`,
			token: "injected_svg_var",
		},
		{
			name:  "trailing backslash on non-last line in multiline SVG",
			input: "injected_svg_var \\\n<svg>",
			token: "injected_svg_var",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lines := bodyAsQuotedStringSlice(tc.input)
			outputBlock := strings.Join(lines, "\n")
			assertInSVGLineQuotes(t, outputBlock, tc.token)
		})
	}

}

func TestTemplateExpand_SkSLToCPP_ChildUniformNameBreakout_Reproduction(t *testing.T) {
	tmplMap, err := loadTemplates()
	require.NoError(t, err)

	testCases := []struct {
		name        string
		uniformName string
		token       string
	}{
		{
			name:        "child uniform name breakout",
			uniformName: `myChild"); int injected_child_var = 123; //`,
			token:       "injected_child_var",
		},
		{
			name:        "already escaped quote breakout",
			uniformName: `myChild\"); int injected_child_var = 123; //`,
			token:       "injected_child_var",
		},
		{
			name:        "multiple backslashes breakout",
			uniformName: `myChild\\"); int injected_child_var = 123; //`,
			token:       "injected_child_var",
		},
		{
			name:        "trailing backslash inside child uniform",
			uniformName: `myChild\`,
			token:       "myChild",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := ScrapBody{
				Type: SKSL,
				Body: "half4 main() { return half4(1); }",
				SKSLMetaData: &SKSLMetaData{
					Children: []ChildShader{
						{
							UniformName:     tc.uniformName,
							ScrapHashOrName: "@some_child",
						},
					},
				},
			}

			rootNode := scrapNode{
				Name:  "1",
				Scrap: body,
				Children: []scrapNode{
					{Name: "2", Scrap: ScrapBody{Type: SKSL, Body: "half4 main() { return half4(1); }"}},
				},
			}

			var b bytes.Buffer
			err = tmplMap[CPP][SKSL].Execute(&b, rootNode)
			require.NoError(t, err)

			assertInDoubleQuotedString(t, b.String(), tc.token)
		})
	}
}
