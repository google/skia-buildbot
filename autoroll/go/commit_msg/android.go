package commit_msg

import "text/template"

const (
	// TmplNameAndroid is the name of the commit message template used by
	// rollers which roll into Android.
	TmplNameAndroid = "android"

	// TmplNameAndroidNoCR is the name of the commit message template used by
	// rollers which roll into Android where the service account does not
	// have CR+2 access.
	TmplNameAndroidNoCR = "android_nocr"
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

	// TmplAndroidNoCR is the commit message template used by rollers which roll
	// into Android where the service account does not have CR+2 access.
	// It can be referenced in config files using tmplNameAndroidNoCR.
	tmplAndroidNoCR = template.Must(parseCommitMsgTemplate(tmplCommitMsg, TmplNameAndroidNoCR,
		`{{- define "boilerplate" }}
Please enable autosubmit on changes if possible when approving them.

{{ template "defaultBoilerplate" . }}{{ end -}}

{{- define "footer" -}}
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
