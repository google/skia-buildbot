package coverageingest

import "html/template"

// coverageSummaryTemplateData is the data needed to expand the html template
// to render the summary file for many files.
type coverageSummaryTemplateData struct {
	Commit  string
	JobName string
	Files   fileSummarySlice
}

// fileSummaryTemplateData is the data entry for the coverage summary of a
// single source file.
type fileSummaryTemplateData struct {
	FileName     string
	PercentLines string
	CoveredLines int
	TotalLines   int
}

type fileSummarySlice []fileSummaryTemplateData

func (s fileSummarySlice) Len() int           { return len(s) }
func (s fileSummarySlice) Less(i, j int) bool { return s[i].FileName < s[j].FileName }
func (s fileSummarySlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

var HTML_TEMPLATE_SUMMARY = template.Must(template.New("summary").Parse(`<!DOCTYPE html>
<html>
	<head>
		<meta name="viewport" content="width=device-width,initial-scale=1">
		<meta charset="UTF-8">
		<link rel="stylesheet" type="text/css" href="/res/css/coverage-style.css">
	</head>
	<body>
		<h1 class="coverage-header">Coverage Summary</h1>
		<div class="coverage-subheader">{{.JobName}} @ <a href="https://skia.googlesource.com/skia/+/{{.Commit}}" target="_blank">{{.Commit}}</a></div>
		<table class="summary-table">
		<thead>
			<tr>
				<th>Filename</th>
				<th>Line Coverage</th>
			</tr>
		</thead>
		<tbody>
{{range .Files}}
			<tr>
				<td><a href="coverage/{{.FileName}}.html">{{.FileName}}</a></td>
				<td>{{.PercentLines}}% ({{.CoveredLines}}/{{.TotalLines}})</td>
			</tr>
{{end}}
		</tbody>
		</table>
	</body>
</html>`))
