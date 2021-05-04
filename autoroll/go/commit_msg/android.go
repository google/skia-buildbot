package commit_msg

import "text/template"

const (
	// TmplNameAndroid is the name of the commit message template used by
	// rollers which roll into Android.
	TmplNameAndroid = "android"
)

var (
	// TmplAndroid is the commit message template used by rollers which roll
	// into Android. It can be referenced in config files using tmplNameAndroid.
	tmplAndroid = template.Must(parseCommitMsgTemplate(tmplCommitMsg, TmplNameAndroid,
		`{{- define "footer" -}}
{{ if .IncludeTbrLine -}}
Tbr: {{ stringsJoin .Reviewers "," }}
{{ end -}}
Test: Presubmit checks will test this change.
Exempt-From-Owner-Approval: The autoroll bot does not require owner approval.
{{ if .BugProject -}}
{{ range .Bugs }}Bug: {{ . }}
{{ end }}
{{- end -}}
{{- if .IncludeTests -}}
{{ range .Tests }}Test: {{ . }}
{{ end -}}
{{- end -}}
{{ if .ExtraFooters -}}
{{ range .ExtraFooters }}{{.}}
{{ end -}}
{{- end -}}
{{- end -}}`))
)
