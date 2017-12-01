package coverageingest

import (
	"bufio"
	"bytes"
	"html/template"
	"regexp"

	"go.skia.org/infra/go/util"
)

// coverageData represents a set of lines that can be executed and
// whether they were executed at least once, as reported by the file.
// coverageData should only be used to represent the coverage for
// a single file. If one wants to track coverage for a folder of files,
// they must use multiple coverageData, one per file.
// The coverageData structs for the same file on multiple runs can be
// joined together with the Union() function, allowing a complete coverage
// picture to be calculated.
type coverageData struct {
	executableLines map[int]bool // maps line number -> was run at least once
	totalLines      int
	sourceLines     map[int]string // maps line number -> string that has the code
}

var executedLine = regexp.MustCompile(`^\s*(?P<line_num>\d+)\|\s+(?P<count>[^\|\s]+?)?\|(?P<code>.*)\s*$`)

// parseLinesCovered looks through the contents of a coverage output by
// a Source-based LLVM5 coverage run and returns a coverageData
// struct that represents the results.
func parseLinesCovered(contents string) *coverageData {
	retval := coverageData{
		executableLines: map[int]bool{},
		sourceLines:     map[int]string{},
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(contents))
	for scanner.Scan() {
		line := scanner.Text()
		if match := executedLine.FindStringSubmatch(line); match != nil {
			ln := match[indexForSubexpName("line_num", executedLine)]
			lineNum := util.SafeAtoi(ln)

			count := match[indexForSubexpName("count", executedLine)]
			if count != "" {
				retval.executableLines[lineNum] = count != "0"
			}
			retval.sourceLines[lineNum] = match[indexForSubexpName("code", executedLine)]
		}
	}

	return &retval
}

// indexForSubexpName returns the index of a named regex subexpression. It's not
// complicated but reduces "magic numbers" and makes the logic of complicated
// regexes easier to follow.
func indexForSubexpName(name string, r *regexp.Regexp) int {
	return util.Index(name, r.SubexpNames())
}

// Union joins two coverageData structs together. The operation is
// an OR operation, such that iff either coverageData report a line
// as executed, the result of this function call will have that line
// as executed. It is assumed that both structs have the same source
// lines seen.
func (c *coverageData) Union(o *coverageData) *coverageData {
	if o == nil {
		return c
	}
	set := c.executableLines
	other := o.executableLines

	resultSet := make(map[int]bool, len(set))
	for key, val := range set {
		resultSet[key] = val
	}

	for key, val := range other {
		if a, ok := resultSet[key]; ok {
			resultSet[key] = val || a
		} else {
			resultSet[key] = val
		}
	}

	return &coverageData{executableLines: resultSet, sourceLines: o.sourceLines}
}

// TotalSource returns the total number of source lines in this file.
func (c *coverageData) TotalSource() int {
	return len(c.sourceLines)
}

// TotalExecutable returns the total number of lines that are potentially
// executable in this file.
func (c *coverageData) TotalExecutable() int {
	return len(c.executableLines)
}

// MissedExecutable returns how many lines were not executed in this file.
func (c *coverageData) MissedExecutable() int {
	missed := 0
	for _, v := range c.executableLines {
		if !v {
			missed++
		}
	}
	return missed
}

type TemplateData struct {
	FileName string
	Commit   string
	JobName  string
}

type lineData struct {
	Number  int
	Covered string
	Source  string
}

type coverageTemplateData struct {
	TemplateData
	Lines []lineData
}

// Missed returns how many lines were not executed in this file.
func (c *coverageData) ToHTMLPage(td TemplateData) (string, error) {
	ctd := coverageTemplateData{
		TemplateData: td,
	}
	for i := 1; i <= c.TotalSource(); i++ {
		cov := ""
		if covered, executable := c.executableLines[i]; executable {
			if covered {
				cov = "yes"
			} else {
				cov = "no"
			}
		}
		ctd.Lines = append(ctd.Lines, lineData{
			Number:  i,
			Covered: cov,
			Source:  c.sourceLines[i],
		})
	}
	b := bytes.Buffer{}
	err := HTML_TEMPLATE_FILE.Execute(&b, ctd)
	return b.String(), err
}

var HTML_TEMPLATE_FILE = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html>
	<head>
		<meta name="viewport" content="width=device-width,initial-scale=1">
		<meta charset="UTF-8">
		<link rel="stylesheet" type="text/css" href="/res/css/coverage-style.css">
	</head>
	<body>
		<h1 class="coverage-header">{{.JobName}}</h1>
		<div class="coverage-subheader">{{.FileName}} @ <a href="https://skia.googlesource.com/skia/+/{{.Commit}}" target="_blank">{{.Commit}}</a></div>
		<table class="coverage-table">
		<thead>
			<tr>
				<th>Line</th>
				<th>Covered?</th>
				<th>Source</th>
			</tr>
		</thead>
		<tbody>
{{range .Lines}}
			<tr>
				<td>{{.Number}}</td>
				<td class="covered-{{.Covered}}">{{.Covered}}</td>
				<td><pre>{{.Source}}</pre></td>
			</tr>
{{end}}
		</tbody>
		</table>
	</body>
</html>`))

type coverageSummaryTemplateData struct {
	Commit  string
	JobName string
	Files   summaryFileSlice
}

type summaryFile struct {
	FileName     string
	PercentLines string
	CoveredLines int
	TotalLines   int
}

type summaryFileSlice []summaryFile

func (s summaryFileSlice) Len() int           { return len(s) }
func (s summaryFileSlice) Less(i, j int) bool { return s[i].FileName < s[j].FileName }
func (s summaryFileSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

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
