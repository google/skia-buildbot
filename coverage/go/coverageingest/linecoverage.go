package coverageingest

import (
	"bufio"
	"bytes"
	"html/template"
	"regexp"

	"go.skia.org/infra/go/util"
)

// coverageData represents which lines in a source code file were covered.
// There are two categories of lines - sourceLine and executableLines
// Every line (even the empty ones) in a source file are a sourceLine, but
// only those that contain executable code (and not, for example, comments)
// are considered executableLines. coverageData treats coverage as a binary
// status - it was run at least once or not.
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

// CoverageFileData represents the data to create a coverage summary for
// a single source file.
type CoverageFileData struct {
	FileName string
	Commit   string
	JobName  string
}

// coverageTemplateData is the data needed to expand the html template
// to render the coverage summary for a single source file.
type coverageTemplateData struct {
	CoverageFileData
	Lines []lineTemplateData
}

// lineTemplateData represents the template data needed for a single source line.
type lineTemplateData struct {
	Number  int
	Covered string
	Source  string
}

// ToHTMLPage returns an HTML page representing this coverageData.
// It uses the built-in html templating, so it should be immune to
// potential XSS attacks (e.g. td.JobName = "<script></script>" )
func (c *coverageData) ToHTMLPage(td CoverageFileData) (string, error) {
	ctd := coverageTemplateData{
		CoverageFileData: td,
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
		ctd.Lines = append(ctd.Lines, lineTemplateData{
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
			<tr class="covered-{{.Covered}}">
				<td>{{.Number}}</td>
				<td class="covered-{{.Covered}}">{{.Covered}}</td>
				<td><pre>{{.Source}}</pre></td>
			</tr>
{{end}}
		</tbody>
		</table>
	</body>
</html>`))
