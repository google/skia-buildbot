package scrap

import (
	"fmt"
	"text/template"

	"go.skia.org/infra/go/skerr"
)

type templateMap map[Lang]map[Type]*template.Template

func templateName(t Type, lang Lang) string {
	return fmt.Sprintf("%s-%s", t, lang)
}

func loadTemplates() (templateMap, error) {
	ret := templateMap{}
	for _, lang := range allLangs {
		ret[lang] = map[Type]*template.Template{}
		for _, t := range allTypes {
			tmpl, err := template.New("").Parse(templates[templateName(t, lang)])
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

// TODO(jcgregorio) Doesn't actually work yet, svg isn't turned on in fiddle.
const svgCpp = `
void draw(SkCanvas* canvas) {
    sk_sp<SkData> data(SkData::MakeWithCString("{{ .Body }}"));

    SkMemoryStream stream(std::move(data));
    sk_sp<SkSVGDOM> svgDom = SkSVGDOM::MakeFromStream(stream);

    // Use the intrinsic SVG size if available, otherwise fall back to a default value.
    static const SkSize kDefaultContainerSize = SkSize::Make(128, 128);
    if (svgDom->containerSize().isEmpty()) {
        svgDom->setContainerSize(kDefaultContainerSize);
    }

  	svgDom->render(canvas);
}`
