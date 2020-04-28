package commit_msg

const (
	// TmplNameAndroid is the name of the commit message template used by
	// rollers which roll into Android.
	TmplNameAndroid = "android"

	// TmplAndroid is the commit message template used by rollers which roll
	// into Android. It can be referenced in config files using TmplNameAndroid.
	TmplAndroid = `Roll {{.ChildName}} {{.RollingFrom}}..{{.RollingTo}} ({{len .Revisions}} commits)

{{if .ChildLogURL}}{{.ChildLogURL}}

{{end}}{{if .IncludeLog}}{{range .Revisions}}{{.Timestamp.Format "2006-01-02"}} {{.Author}} {{.Description}}
{{end}}
{{end}}` + commitMsgInfoText + `

Test: Presubmit checks will test this change.
Exempt-From-Owner-Approval: The autoroll bot does not require owner approval.
{{if .IncludeBugs}}{{range .Bugs}}Bug: {{.}}
{{end}}{{end}}{{if .IncludeTests}}{{range .Tests}}Test: {{.}}
{{end}}{{end}}`
)
