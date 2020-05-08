package commit_msg

const (
	// TmplNameDefault is the name of the default commit message template which
	// is suitable for most rollers.
	TmplNameDefault = "default"

	// TmplDefault is the default commit message template which is suitable for
	// most rollers. It can be referenced in config files using TmplNameDefault.
	TmplDefault = `Roll {{.ChildName}} from {{.RollingFrom}} to {{.RollingTo}}{{if .IncludeRevisionCount}} ({{len .Revisions}} revision{{if gt (len .Revisions) 1}}s{{end}}){{end}}

{{if .ChildLogURL}}{{.ChildLogURL}}

{{end}}{{if .IncludeLog}}{{range .Revisions}}{{.Timestamp.Format "2006-01-02"}} {{.Author}} {{.Description}}
{{end}}
{{end}}{{if len .TransitiveDeps}}Also rolling transitive DEPS:
{{range .TransitiveDeps}}  {{.Dep}}: {{substr .RollingFrom 0 12}}..{{substr .RollingTo 0 12}}
{{end}}
{{end}}` + commitMsgInfoText + `

{{if .CqExtraTrybots}}Cq-Include-Trybots: {{stringsJoin .CqExtraTrybots ";"}}
{{end}}Bug: {{if .Bugs}}{{stringsJoin .Bugs ","}}{{else}}None{{end}}
{{if .IncludeTbrLine}}Tbr: {{stringsJoin .Reviewers ","}}
{{end}}{{if .IncludeTests}}{{range .Tests}}Test: {{.}}
{{end}}{{end}}`
)
