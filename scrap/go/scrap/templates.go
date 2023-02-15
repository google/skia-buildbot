// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"go.skia.org/infra/go/skerr"
)

const whitespaceChars = "\n\r\t "

var (
	uniformFloatScalarRegex = regexp.MustCompile(`^float([234]?)$`)
	uniformFloatMatrixRegex = regexp.MustCompile(`^float([234])x([234])$`)
	uniformDefinitionRegex  = regexp.MustCompile(`^\s*uniform\s+(?P<type>float[234x]*)\s+(?P<name>i[^\s;]+)\s*;$`)
)

type templateMap map[Lang]map[Type]*template.Template

// floatSliceToString will convert a slice of float32's to a string
// representation. e.g. []float{1,2,3} will become "1, 2, 3"
func floatSliceToString(floats []float32) string {
	return strings.Trim(strings.Join(strings.Fields(fmt.Sprint(floats)), ", "), "[]")
}

// mustWriteStringf will format a string and write it to the supplied StringWriter.
// If the write fails then it will panic.
//
// Note: This is intended to be used for writes to a memory buffer where the only
// cause of failure will be an OOM condition. Writes to any other device,
// file, network, etc., should use a different approach and properly handle
// any errors.
func mustWriteStringf(w io.StringWriter, msg string, args ...interface{}) {
	if _, err := w.WriteString(fmt.Sprintf(msg, args...)); err != nil {
		panic(err)
	}
}

// uniformValue represents a single instance of a uniform definition
// in a shader script. For example a definition like:
//
// uniform float3 iColor;
//
// will have a Name of "iColor", and a Type of "float3".
type uniformValue struct {
	Name string
	Type string
}

// numFloats will return the number of floats needed to represent a
// uniformValue.
func (u uniformValue) numFloats() (int, error) {
	if u.Type == "float" {
		return 1, nil
	}
	m := uniformFloatScalarRegex.FindStringSubmatch(u.Type)
	if m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		return n, nil
	}
	if m = uniformFloatMatrixRegex.FindStringSubmatch(u.Type); m != nil {
		x, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		y, err := strconv.Atoi(m[2])
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		return x * y, nil
	}
	return 0, skerr.Fmt("Can't determine size of %q", u.Type)
}

func (u uniformValue) getCppDefinitionString(vals []float32) (string, error) {
	switch u.Type {
	case "float":
		return fmt.Sprintf(`builder.uniform("%s") = %s;`, u.Name, floatSliceToString(vals[0:1])), nil
	case "float2":
		return fmt.Sprintf(`builder.uniform("%s") = SkV2{%s};`, u.Name, floatSliceToString(vals[0:2])), nil
	case "float3":
		return fmt.Sprintf(`builder.uniform("%s") = SkV3{%s};`, u.Name, floatSliceToString(vals[0:3])), nil
	case "float4":
		return fmt.Sprintf(`builder.uniform("%s") = SkV4{%s};`, u.Name, floatSliceToString(vals[0:4])), nil
	case "float2x2":
		return fmt.Sprintf(`builder.uniform("%s") = SkV4{%s};`, u.Name, floatSliceToString(vals[0:4])), nil
	}
	return "", skerr.Fmt("Unsupported uniform type %q", u.Type)
}

func extractBodyUniforms(body ScrapBody) ([]uniformValue, error) {
	var uniforms []uniformValue
	for _, line := range strings.Split(body.Body, "\n") {
		if m := uniformDefinitionRegex.FindStringSubmatch(line); m != nil {
			uniforms = append(uniforms, uniformValue{
				Name: m[2],
				Type: m[1],
			})
		}
	}
	return uniforms, nil
}

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

// indentMultilineString will indent each line of a multiline string,
// if that line is not empty, by the number of spaces specified by |indent|.
// Each line will also be trimmed of all trailing whitespace characters.
func indentMultilineString(body string, indent int) string {
	var buffer bytes.Buffer
	first := true
	for _, line := range strings.Split(body, "\n") {
		if !first {
			buffer.WriteString("\n")
		}
		first = false
		line = strings.TrimRight(line, whitespaceChars)
		if len(line) > 0 {
			buffer.WriteString(strings.Repeat(" ", indent))
			buffer.WriteString(line)
		}
	}
	return buffer.String()
}

func getSkSLImageURL(body ScrapBody) (string, error) {
	if body.Type != SKSL {
		return "", skerr.Fmt("Scrap is not of type SkSL: %v", body.Type)
	}
	const defaultShaderImageURL = "https://shaders.skia.org/img/mandrill.png"
	if body.SKSLMetaData == nil || len(body.SKSLMetaData.ImageURL) == 0 {
		return defaultShaderImageURL, nil
	}

	if body.SKSLMetaData.ImageURL[0] == '/' {
		return "https://shaders.skia.org" + body.SKSLMetaData.ImageURL, nil
	}

	return body.SKSLMetaData.ImageURL, nil
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
	return fmt.Sprintf("      // User supplied uniform values:\n      %s", vals)
}

// cppCustomUniformValues will return a string that is valid C++ to
// define and initialize the custom uniforms defined in a SkSL scrap.
func cppCustomUniformValues(body ScrapBody) string {
	if body.Type != SKSL || body.SKSLMetaData == nil || len(body.SKSLMetaData.Uniforms) == 0 {
		return ""
	}
	uniforms, err := extractBodyUniforms(body)
	if err != nil {
		return ""
	}
	requiredFloatCount := 0
	for _, u := range uniforms {
		num, err := u.numFloats()
		if err != nil {
			return "    // Cannot determine input size."
		}
		requiredFloatCount += num
	}
	if requiredFloatCount != len(body.SKSLMetaData.Uniforms) {
		return fmt.Sprintf("    // User inputs size mismatch: %d != %d.",
			requiredFloatCount, len(body.SKSLMetaData.Uniforms))
	}
	i := 0
	var buffer bytes.Buffer
	buffer.WriteString("    // Inputs supplied by user:\n")
	for _, u := range uniforms {
		s, err := u.getCppDefinitionString(body.SKSLMetaData.Uniforms[i:])
		if err != nil {
			return fmt.Sprintf("    // Cannot get C++ definition for %s (%s).", u.Name, u.Type)
		}
		buffer.WriteString(fmt.Sprintf("    %s\n", s))
		num, _ := u.numFloats()
		i += num
	}
	return buffer.String()
}

// funcMap are the template helper functions available in each template.
var funcMap = template.FuncMap{
	"bodyAsQuotedStringSlice": bodyAsQuotedStringSlice,
	"getSkSLImageURL":         getSkSLImageURL,
	"getSkSLCustomUniforms":   getSkSLCustomUniforms,
	"cppCustomUniformValues":  cppCustomUniformValues,
	"indentMultilineString":   indentMultilineString,
	"loadImagesJS":            loadImagesJS,
	"createRuntimeEffectsJS":  createRuntimeEffectsJS,
	"createFragmentShadersJS": createFragmentShadersJS,
	"putShaderOnPaintJS":      putShaderOnPaintJS,
	"deleteShadersJS":         deleteShadersJS,
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
    const char * svg ={{ range bodyAsQuotedStringSlice .Scrap.Body }}
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

{{ indentMultilineString .Scrap.Body 8 }}
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
{{ cppCustomUniformValues .Scrap }}
    sk_sp<SkShader> myShader = builder.makeShader();

    // Fill the surface with |myShader|:
    SkPaint p;
    p.setShader(myShader);
    canvas->drawPaint(p);
}`
