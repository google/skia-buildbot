package commit_msg

import "text/template"

const (
	// tmplNameAfdo is the name of the commit message template used for AFDO.
	tmplNameAfdo = "afdo"
)

var (
	// TmplGClient is the commit message template used for AFDO. It can be
	// referenced in config files using tmplNameAfdo.
	tmplAfdo = template.Must(parseCommitMsgTemplate(tmplCommitMsg, "afdo",
		`{{- define "boilerplate" -}}
This CL may cause a small binary size increase, roughly proportional
to how long it's been since our last AFDO profile roll. For larger
increases (around or exceeding 100KB), please file a bug against
gbiv@chromium.org. Additional context: https://crbug.com/805539

Please note that, despite rolling to chrome/android, this profile is
used for both Linux and Android.

{{ template "defaultBoilerplate" . }}
{{- end -}}`))
)
