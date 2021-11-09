package commit_msg

import "text/template"

const (
	// TmplNameCanary is the name of the commit message template used by
	// canary rolls.
	TmplNameCanary = "canary"
)

var (
	// TmplCanary is the commit message template used by canary rolls.
	// It can be referenced in config files using TmplNameCanary.
	tmplCanary = template.Must(parseCommitMsgTemplate(tmplCommitMsg, TmplNameCanary,
		`{{- define "subject" }}Canary roll {{ .ChildName }} to {{ .RollingTo }}{{ end -}}
{{- define "revisions" }}{{ if .ChildLogURL }}{{ .ChildLogURL }}{{ end -}}{{end -}}
{{- define "boilerplate" }}
DO_NOT_SUBMIT: This canary roll is only for testing

Documentation for Autoroller Canaries is here:
go/autoroller-canary-bots (Googlers only)

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug
{{ end -}}
{{- define "footer" -}}
Commit: false
{{ if .CqExtraTrybots -}}
Cq-Include-Trybots: {{ stringsJoin .CqExtraTrybots ";" }}
{{ end -}}
{{ if .CqDoNotCancelTrybots -}}
Cq-Do-Not-Cancel-Tryjobs: true
{{ end -}}
{{- end -}}`))
)
