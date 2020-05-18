package commit_msg

import "text/template"

const (
	// TmplNameDefault is the name of the default commit message template which
	// is suitable for most rollers.
	TmplNameDefault = "default"
)

var (
	// tmplNameCommitMsg is the name of the overall commit message template.
	tmplNameCommitMsg = "commitMsg"

	// tmplCommitMsg utilizes the skeleton with the default block definitions.
	// It is the primary entry point which is executed for every commit message.
	// Custom commit messages may modify it by overriding any of the blocks
	// defined above, including the commitMsg itself for a completely custom
	// message.
	tmplCommitMsg = template.Must(parseCommitMsgTemplate(tmplFooterDefault, tmplNameCommitMsg, `
{{- define "subject" }}{{ template "defaultSubject" . }}{{ end -}}
{{- define "revisions" }}{{ template "defaultRevisions" . }}{{ end -}}
{{- define "boilerplate" }}{{ template "defaultBoilerplate" . }}{{ end -}}
{{- define "footer" }}{{ template "defaultFooter" . }}{{ end -}}
{{ template "skeleton" . }}`))

	tmplNameSkeleton = "skeleton"
	// tmplSkeleton defines the basic structure for all commit messages.
	tmplSkeleton = template.Must(parseCommitMsgTemplate(nil, tmplNameSkeleton,
		`{{ template "subject" . }}

{{ template "revisions" . }}

{{ template "boilerplate" . }}

{{ template "footer" . }}
`))

	tmplNameSubjectDefault = "defaultSubject"
	tmplSubjectDefault     = template.Must(parseCommitMsgTemplate(tmplSkeleton, tmplNameSubjectDefault,
		`Roll {{ .ChildName }} from {{ .RollingFrom }} to {{ .RollingTo }}{{ if .IncludeRevisionCount}} ({{ len .Revisions }} revision{{ if gt (len .Revisions) 1 }}s{{ end }}){{ end }}`))

	tmplNameRevisionsDefault = "defaultRevisions"
	tmplRevisionsDefault     = template.Must(parseCommitMsgTemplate(tmplSubjectDefault, tmplNameRevisionsDefault,
		`{{ if .ChildLogURL }}{{ .ChildLogURL }}

{{ end -}}
{{- if .IncludeLog -}}
{{ range .Revisions }}{{ .Timestamp.Format "2006-01-02" }} {{ .Author }} {{ .Description }}
{{ end }}
{{ end -}}
{{ if len .TransitiveDeps -}}
Also rolling transitive DEPS:
{{ range .TransitiveDeps }}  {{ .Dep }} from {{ substr .RollingFrom 0 12 }} to {{ substr .RollingTo 0 12 }}
{{ end }}
{{- end }}`))

	tmplNameBoilerplateDefault = "defaultBoilerplate"
	tmplBoilerplateDefault     = template.Must(parseCommitMsgTemplate(tmplRevisionsDefault, tmplNameBoilerplateDefault,
		`If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
{{.ServerURL}}
Please CC {{stringsJoin .Reviewers ","}} on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/master/autoroll/README.md`))

	tmplNameFooterDefault = "defaultFooter"
	tmplFooterDefault     = template.Must(parseCommitMsgTemplate(tmplBoilerplateDefault, tmplNameFooterDefault,
		`{{ if .CqExtraTrybots -}}
Cq-Include-Trybots: {{ stringsJoin .CqExtraTrybots ";" }}
{{ end -}}
{{ if .BugProject -}}
Bug: {{ if .Bugs }}{{ stringsJoin .Bugs "," }}{{ else }}None{{ end }}
{{ end -}}
{{ if .IncludeTbrLine -}}
Tbr: {{ stringsJoin .Reviewers "," }}
{{ end -}}
{{ if .IncludeTests -}}
{{ range .Tests }}Test: {{.}}
{{- end -}}
{{- end }}`))
)
