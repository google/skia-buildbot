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

// bodyAsStringSlice is a template helper function that breaks up a ScrapBody
// into a form suitable as a multi-line C++ const char * string.
//
// That is, it takes "<svg> \n</svg>"
//
// And turns it into:
//
//  []string{
//    `"<svg> \n"`,
//    `"</svg>";`,
//  }
func bodyAsStringSlice(body string) []string {
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

// funcMap are the template helper functions available in each template.
var funcMap = template.FuncMap{
	"bodyAsStringSlice": bodyAsStringSlice,
}

func loadTemplates() (templateMap, error) {
	ret := templateMap{}
	for _, lang := range allLangs {
		ret[lang] = map[Type]*template.Template{}
		for _, t := range allTypes {
			tmpl, err := template.New("").Funcs(funcMap).Parse(templates[templateName(t, lang)])
			if err != nil {
				return nil, skerr.Wrapf(err, "Failed to parse template %v %v", lang, t)
			}
			ret[lang][t] = tmpl
		}
	}

	return ret, nil
}

// TODO(jcgregorio) Fill in the rest of the templates.
var templates = map[string]string{
	"svg-cpp":       svgCpp,
	"svg-js":        "",
	"sksl-cpp":      "",
	"sksl-js":       "",
	"particles-cpp": "",
	"particles-js":  "",
}

const svgCpp = `void draw(SkCanvas* canvas) {
    const char * svg ={{ range bodyAsStringSlice .Body }}
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
