package scrap

import (
	"fmt"
	"text/template"
)

func templateName(t Type, lang Lang) string {
	return fmt.Sprintf("%s-%s", t, lang)
}

func loadTemplates() (*template.Template, error) {
	ret := template.New("")
	for _, t := range allTypes {
		for _, lang := range allLangs {
			name := templateName(t, lang)
			ret.New(name).Parse(templates[name])
		}
	}

	return ret, nil
}

var templates = map[string]string{
	"svg-cpp":       "",
	"svg-js":        "",
	"sksl-cpp":      "",
	"sksl-js":       "",
	"particles-cpp": "",
	"particles-js":  "",
}
