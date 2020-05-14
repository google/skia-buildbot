package parent

import (
	"bytes"
	"strings"
	"text/template"

	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
)

const (
	TMPL_COMMIT_MSG_DEFAULT = `Roll {{.ChildPath}} {{.RollingFrom.String}}..{{.RollingTo.String}} ({{len .Revisions}} commits)

{{.ChildRepo}}/+log/{{.RollingFrom.String}}..{{.RollingTo.String}}

{{if .IncludeLog}}git log {{.RollingFrom}}..{{.RollingTo}} --date=short --first-parent --format='%ad %ae %s'
{{range .Revisions}}{{.Timestamp.Format "2006-01-02"}} {{.Author}} {{.Description}}
{{end}}{{end}}{{if len .TransitiveDeps}}
Also rolling transitive DEPS:
{{range .TransitiveDeps}}  {{.Dep}} {{substr .RollingFrom 0 12}}..{{substr .RollingTo 0 12}}
{{end}}{{end}}
Created with:
  gclient setdep -r {{.ChildPath}}@{{.RollingTo}}

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
{{.ServerURL}}
Please CC {{stringsJoin .Reviewers ","}} on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/master/autoroll/README.md

{{if .CqExtraTrybots}}Cq-Include-Trybots: {{.CqExtraTrybots}}
{{end}}Bug: {{if .Bugs}}{{stringsJoin .Bugs ","}}{{else}}None{{end}}
Tbr: {{stringsJoin .Reviewers ","}}`
)

// CommitMsgVars contains variables used to fill in a commit message template.
type CommitMsgVars struct {
	Bugs           []string
	ChildPath      string
	ChildRepo      string
	CqExtraTrybots string
	IncludeLog     bool
	Reviewers      []string
	Revisions      []*revision.Revision
	RollingFrom    *revision.Revision
	RollingTo      *revision.Revision
	ServerURL      string
	Tests          []string
	TransitiveDeps []*version_file_common.TransitiveDepUpdate
}

// ParseCommitMsgTemplate parses the given commit message template string and
// returns a Template instance.
func ParseCommitMsgTemplate(tmpl string) (*template.Template, error) {
	return template.New("commitMsg").Funcs(template.FuncMap{
		"stringsJoin": strings.Join,
		"substr": func(s string, a, b int) string {
			if a > len(s) {
				return ""
			}
			if b > len(s) {
				b = len(s)
			}
			return s[a:b]
		},
	}).Parse(tmpl)
}

// ValidateCommitMsgTemplate returns an error if the given commit message
// template cannot be parsed and executed with a typical set of inputs.
func ValidateCommitMsgTemplate(tmpl string) error {
	t, err := ParseCommitMsgTemplate(tmpl)
	if err != nil {
		return skerr.Wrapf(err, "Failed to parse template")
	}
	a := &revision.Revision{
		Id:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Display: "aaaaaaaaaaaa",
		Author:  "a@google.com",
	}
	b := &revision.Revision{
		Id:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Display: "bbbbbbbbbbbb",
		Author:  "b@google.com",
	}
	c := &revision.Revision{
		Id:      "cccccccccccccccccccccccccccccccccccccccc",
		Display: "cccccccccccc",
		Author:  "c@google.com",
	}
	vars := &CommitMsgVars{
		Bugs:           []string{"skia:1234"},
		ChildPath:      "path/to/child",
		ChildRepo:      "https://child-repo.git",
		CqExtraTrybots: "extra-bot",
		IncludeLog:     true,
		Reviewers:      []string{"me@google.com"},
		Revisions:      []*revision.Revision{b, c},
		RollingFrom:    a,
		RollingTo:      c,
		ServerURL:      "https://fake.server.url",
		Tests:          []string{"some-test"},
		TransitiveDeps: []*version_file_common.TransitiveDepUpdate{
			{
				Dep:         "path/to/other",
				RollingFrom: "dddddddddddddddddddddddddddddddddddddddd",
				RollingTo:   "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
			},
		},
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return err
	}
	return nil
}
