package scrap

import (
	"fmt"
	"strings"
	"text/template"

	"go.skia.org/infra/go/skerr"
)

type templateMap map[Lang]map[Type]*template.Template

func templateName(t Type, lang Lang) string {
	return fmt.Sprintf("%s-%s", t, lang)
}

// bodyAsQuotedStringSlice is a template helper function that breaks up a ScrapBody
// into a form suitable as a multi-line C++ const char * string.
//
// That is, it takes "<svg> \n</svg>"
//
// And turns it into:
//
//	[]string{
//	  `"<svg> \n"`,
//	  `"</svg>";`,
//	}
func bodyAsQuotedStringSlice(body string) []string {
	// Escape all the double quotes.
	body = strings.ReplaceAll(body, `"`, `\"`)

	// Break into individual lines.
	lines := strings.Split(body, "\n")
	numLines := len(lines)
	ret := make([]string, 0, numLines)
	for i, line := range lines {
		// Quote each line.
		quotedLine := `"` + line
		if i == numLines-1 {
			// Since this is the very last line, add end quote and terminating semicolon.
			quotedLine += `";`
		} else {
			// Add newline and end quote.
			quotedLine += `\n"`
		}

		ret = append(ret, quotedLine)
	}
	return ret
}

// bodyStringSlice is a template helper function that splits up a ScrapBody
// into a slice of strings representing each line of the ScrapBody.
//
// That is, it takes "foo \nbar"
//
// And turns it into:
//
//	[]string{
//	  "foo ",
//	  "bar",
//	}
func bodyStringSlice(body string) []string {
	return strings.Split(body, "\n")
}

func getSkSLImageURL(body ScrapBody) string {
	const defaultShaderImageURL = "https://shaders.skia.org/img/mandrill.png"
	if body.Type != SKSL || body.SKSLMetaData == nil || len(body.SKSLMetaData.ImageURL) == 0 {
		return defaultShaderImageURL
	}

	if body.SKSLMetaData.ImageURL[0] == '/' {
		return "https://shaders.skia.org" + body.SKSLMetaData.ImageURL
	}

	return body.SKSLMetaData.ImageURL
}

// Return the SkSL scrap custom uniform values as a C++/JavaScript array
// literal subset. For example, the template will have a template similar to:
//
// const uniforms = [
//
//	  0.5, 0.5,
//		... // Other values.
//		{{ getSkSLCustomUniforms . }}
//
// ];
//
// and this function will return a string similar to:
// `// A comment:
//
//	val1, val2, val3`
//
// If the scrap contains no custom uniforms then an ampty string will be returned.
func getSkSLCustomUniforms(body ScrapBody) string {
	if body.Type != SKSL || body.SKSLMetaData == nil || len(body.SKSLMetaData.Uniforms) == 0 {
		return ""
	}
	vals := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(body.SKSLMetaData.Uniforms)), ", "), "[]")
	return fmt.Sprintf("\n      // User supplied uniform values:\n      %s", vals)
}

// funcMap are the template helper functions available in each template.
var funcMap = template.FuncMap{
	"bodyAsQuotedStringSlice": bodyAsQuotedStringSlice,
	"bodyStringSlice":         bodyStringSlice,
	"getSkSLImageURL":         getSkSLImageURL,
	"getSkSLCustomUniforms":   getSkSLCustomUniforms,
}

func loadTemplates() (templateMap, error) {
	ret := templateMap{}
	for _, lang := range AllLangs {
		ret[lang] = map[Type]*template.Template{}
		for _, t := range AllTypes {
			tmpl, err := template.New("").Funcs(funcMap).Parse(templates[templateName(t, lang)])
			if err != nil {
				return nil, skerr.Wrapf(err, "Failed to parse template %v %v", lang, t)
			}
			ret[lang][t] = tmpl
		}
	}

	return ret, nil
}

// TODO(cmumford) Fill in the rest of the templates.
var templates = map[string]string{
	"svg-cpp":       svgCpp,
	"svg-js":        "",
	"sksl-cpp":      skslCpp,
	"sksl-js":       skslJavaScript,
	"particles-cpp": "",
	"particles-js":  "",
}

const svgCpp = `void draw(SkCanvas* canvas) {
    const char * svg ={{ range bodyAsQuotedStringSlice .Body }}
        {{.}}{{end}}

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

const skslCpp = `void draw(SkCanvas *canvas) {
    canvas->clear(SK_ColorBLACK);

    constexpr char sksl[] = R"(
        // Inputs supplied by shaders.skia.org:
        uniform float3 iResolution;      // Viewport resolution (pixels)
        uniform float  iTime;            // Shader playback time (s)
        uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
        uniform float3 iImageResolution; // iImage1 resolution (pixels)
        uniform shader iImage1;          // An input image.
{{ range bodyStringSlice .Body }}
        {{.}}{{end}}
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

const skslJavaScript = `const loadImage = fetch("{{ getSkSLImageURL . }}")
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
{{ range bodyStringSlice .Body }}
    {{.}}{{end}}
    ` + "`" + `;

  const effect = CanvasKit.RuntimeEffect.Make(prog);

  function drawFrame(canvas) {
    const uniforms = [
      512, 512, 1,                                      // iResolution
      (Date.now() - startTimeMs) / 1000,                // iTime
      mouseDragX, mouseDragY, mouseClickX, mouseClickY, // iMouse
      img.width(), img.height(), 1,                     // iImageResolution
{{ getSkSLCustomUniforms . }}
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
