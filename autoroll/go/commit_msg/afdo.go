package commit_msg

const (
	// TmplNameAfdo is the name of the commit message template used for AFDO.
	TmplNameAfdo = "afdo"

	// TmplGClient is the commit message template used for AFDO. It can be
	// referenced in config files using TmplNameAfdo.
	TmplAfdo = `Roll AFDO from {{.RollingFrom.String}} to {{.RollingTo.String}}

This CL may cause a small binary size increase, roughly proportional
to how long it's been since our last AFDO profile roll. For larger
increases (around or exceeding 100KB), please file a bug against
gbiv@chromium.org. Additional context: https://crbug.com/805539

Please note that, despite rolling to chrome/android, this profile is
used for both Linux and Android.

` + commitMsgInfoText + `

{{if .CqExtraTrybots}}Cq-Include-Trybots: {{stringsJoin .CqExtraTrybots ";"}}
{{end}}{{if .IncludeTbrLine}}Tbr: {{stringsJoin .Reviewers ","}}
{{end}}{{if .IncludeTests}}{{range .Tests}}Test: {{.}}
{{end}}{{end}}`
)
