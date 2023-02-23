// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"

	"go.skia.org/infra/go/skerr"
)

const (
	whitespaceChars = "\n\r\t "
	// stock shader inputs defined by shaders.skia.org for SkSL scraps.
	skslDefaultInputs = `// Inputs supplied by shaders.skia.org:
uniform float3 iResolution;      // Viewport resolution (pixels)
uniform float  iTime;            // Shader playback time (s)
uniform float4 iMouse;           // Mouse drag pos=.xy Click pos=.zw (pixels)
uniform float3 iImageResolution; // iImage1 resolution (pixels)
uniform shader iImage1;          // An input image.`
)

var uniformDefinitionRegex = regexp.MustCompile(`^\s*uniform\s+(?P<type>float[234x]*)\s+(?P<name>i[^\s;]+)\s*;$`)

type templateMap map[Lang]map[Type]*template.Template

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

// writeShaderInputDefinitions will write the variable definitions, in SkSL,
// using the supplied writer, for any shader inputs. These are not the same
// as standard variable inputs as those are already part of the shader code
// writen by the user.
func writeShaderInputDefinitions(w io.StringWriter, node scrapNode, indent int) {
	if node.Scrap.SKSLMetaData == nil {
		return
	}
	for _, child := range node.Scrap.SKSLMetaData.Children {
		mustWriteStringf(w, "%suniform shader %s;\n", strings.Repeat(" ", indent), child.UniformName)
	}
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

// funcMap are the template helper functions available in each template.
var funcMap = template.FuncMap{
	"bodyAsQuotedStringSlice": bodyAsQuotedStringSlice,
	"getSkSLImageURL":         getSkSLImageURL,
	"getSkSLCustomUniforms":   getSkSLCustomUniforms,
	"createShadersCPP":        createShadersCPP,
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
	"svg-cpp":  svgCpp,
	"svg-js":   "",
	"sksl-cpp": skslCpp,
	"sksl-js":  skslJavaScript,
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
