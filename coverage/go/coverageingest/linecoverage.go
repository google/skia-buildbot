package coverageingest

import (
	"bufio"
	"bytes"
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
}

var executedLine = regexp.MustCompile(`^\s*(?P<line_num>\d+)\|\s+(?P<count>[^\|\s]+?)\|(?P<code>.*)$`)

// parseLinesCovered looks through the contents of a coverage output by
// a Source-based LLVM5 coverage run and returns a coverageData
// struct that represents the results.
func parseLinesCovered(contents string) *coverageData {
	retval := coverageData{
		executableLines: map[int]bool{},
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(contents))
	for scanner.Scan() {
		line := scanner.Text()
		if match := executedLine.FindStringSubmatch(line); match != nil {
			lineNum := util.SafeAtoi(match[indexForSubexpName("line_num", executedLine)])

			counts := match[indexForSubexpName("count", executedLine)]
			retval.executableLines[lineNum] = counts != "0"
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
// as executed.
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

	return &coverageData{executableLines: resultSet}
}

// Total returns the total number of lines that are potentially
// executable in this file.
func (c *coverageData) Total() int {
	return len(c.executableLines)
}

// Missed returns how many lines were not executed in this file.
func (c *coverageData) Missed() int {
	missed := 0
	for _, v := range c.executableLines {
		if !v {
			missed++
		}
	}
	return missed
}
