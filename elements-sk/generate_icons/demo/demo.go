package demo

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	"go.skia.org/infra/go/skerr"
)

const iconsTSPath = "elements-sk/modules/icons-demo-sk/icons.ts"

// Generate generates file //elements-sk/modules/icons-demo-sk/icons.ts, which is used by the
// icons-demo-sk custom element to showcase all generated icons.
func Generate(workspaceDir string, iconNamesByCategory map[string][]string) error {
	buf := &bytes.Buffer{}
	if err := iconsTSTemplate.Execute(buf, iconsTSTemplateData{IconNamesByCategory: iconNamesByCategory}); err != nil {
		return skerr.Wrap(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, iconsTSPath), buf.Bytes(), 0644); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

type iconsTSTemplateData struct {
	IconNamesByCategory map[string][]string
}

var iconsTSTemplate = template.Must(template.New("icons-demo-sk-ts").Parse(`/* This is a generated file! */
{{ range $category, $iconNames :=  .IconNamesByCategory }}
// Icon category: {{$category}}.
{{ range $iconNames }}import '../icons/{{.}}-icon-sk';
{{ end }}{{ end }}
// Icon names do not contain the "-icon-sk" prefix.
export const icons = new Map<string, string[]>();
{{ range $category, $iconNames :=  .IconNamesByCategory }}icons.set('{{$category}}', [
{{ range $iconNames }}  '{{.}}',
{{ end }}]);
{{ end }}`))
