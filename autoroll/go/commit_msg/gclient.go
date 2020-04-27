package commit_msg

const (
	// TmplNameGClient is the name of the commit message template used by
	// rollers which use gclient.
	TmplNameGClient = "gclient"

	// TmplGClient is the commit message template used by rollers which use
	// gclient. It can be referenced in config files using TmplNameGClient.
	TmplGClient = `Roll {{.ChildName}} {{.RollingFrom}}..{{.RollingTo}} ({{len .Revisions}} commits)

{{if .ChildLogURL}}{{.ChildLogURL}}

{{end}}{{if .IncludeLog}}{{range .Revisions}}{{.Timestamp.Format "2006-01-02"}} {{.Author}} {{.Description}}
{{end}}
{{end}}{{if len .TransitiveDeps}}Also rolling transitive DEPS:
{{range .TransitiveDeps}}  {{.Dep}}: {{substr .RollingFrom 0 12}}..{{substr .RollingTo 0 12}}
{{end}}
{{end}}` + commitMsgInfoText + `

{{if .CqExtraTrybots}}Cq-Include-Trybots: {{.CqExtraTrybots}}
{{end}}Bug: {{if .Bugs}}{{stringsJoin .Bugs ","}}{{else}}None{{end}}
{{if .IncludeTbrLine}}Tbr: {{stringsJoin .Reviewers ","}}
{{end}}{{if .IncludeTests}}{{range .Tests}}Test: {{.}}
{{end}}{{end}}`
)
