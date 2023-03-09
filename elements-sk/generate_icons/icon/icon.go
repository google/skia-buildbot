package icon

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"go.skia.org/infra/go/skerr"
)

const (
	indexTSPathTemplate  = "elements-sk/modules/icons/%s-icon-sk/index.ts"
	iconTSPathTemplate   = "elements-sk/modules/icons/%s-icon-sk/%s-icon-sk.ts"
	iconSCSSPathTemplate = "elements-sk/modules/icons/%s-icon-sk/%s-icon-sk.scss"
)

var svgTagRegexp = regexp.MustCompile(`<svg .+?>(?P<content>.+)</svg>`)

// Generate generates an //elements-sk/modules/icons/<name>-icon-sk custom element that displays
// the SVG contents of the file at iconSvgPath.
func Generate(workspaceDir, name, iconSvgPath string) error {
	// Read in icon file.
	data, err := os.ReadFile(iconSvgPath)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Extract out <svg> tag.
	match := svgTagRegexp.FindStringSubmatch(string(data))
	if match == nil {
		return skerr.Fmt("could not parse <svg> tag from file %s", iconSvgPath)
	}

	// Compute target paths.
	indexTSPath := filepath.Join(workspaceDir, fmt.Sprintf(indexTSPathTemplate, name))
	iconTSPath := filepath.Join(workspaceDir, fmt.Sprintf(iconTSPathTemplate, name, name))
	iconSCSSPath := filepath.Join(workspaceDir, fmt.Sprintf(iconSCSSPathTemplate, name, name))

	// Create custom element's directory.
	if err := os.MkdirAll(filepath.Dir(indexTSPath), os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}

	// Generate index.ts file.
	buf := &bytes.Buffer{}
	if err := indexTSTemplate.Execute(buf, indexTSTemplateData{Name: name}); err != nil {
		return skerr.Wrap(err)
	}
	if err := os.WriteFile(indexTSPath, buf.Bytes(), 0644); err != nil {
		return skerr.Wrap(err)
	}

	// Generate TypeScript file.
	buf = &bytes.Buffer{}
	if err := iconTSTemplate.Execute(buf, iconTSTemplateData{
		Name:    name,
		Content: match[1],
	}); err != nil {
		return skerr.Wrap(err)
	}
	if err := os.WriteFile(iconTSPath, buf.Bytes(), 0644); err != nil {
		return skerr.Wrap(err)
	}

	// Generate SCSS file.
	if err := os.WriteFile(iconSCSSPath, []byte(scssTemplate), 0644); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

type indexTSTemplateData struct{ Name string }

var indexTSTemplate = template.Must(template.New("index-ts").Parse(`// This is a generated file!

import './{{ .Name }}-icon-sk';
`))

type iconTSTemplateData struct {
	Name    string
	Content string
}

var iconTSTemplate = template.Must(template.New("icon-sk-ts").Parse(`// This is a generated file!

import { define } from '../../define';

const iconSkTemplate = document.createElement('template');
iconSkTemplate.innerHTML = '<svg class="icon-sk-svg" xmlns="http://www.w3.org/2000/svg" width=24 height=24 viewBox="0 0 24 24">{{ .Content }}</svg>';

define('{{.Name}}-icon-sk', class extends HTMLElement {
  connectedCallback() {
    const icon = iconSkTemplate.content.cloneNode(true);
    while (this.firstChild) {
      this.removeChild(this.firstChild);
    }
    this.appendChild(icon);
  }
});
`))

const scssTemplate = `/* This is a generated file! */

@import '../icon-sk';
`
