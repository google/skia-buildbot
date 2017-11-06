package coverageingest

import (
	"bufio"
	"bytes"
	"regexp"

	"go.skia.org/infra/go/util"
)

type coverageData struct {
	executableLines map[int]bool // maps line number -> was run
}

var executedLine = regexp.MustCompile(`^\s*(?P<line_num>\d+)\|\s+(?P<count>[^\|\s]+?)\|(?P<code>.*)$`)

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

func (c *coverageData) Total() int {
	return len(c.executableLines)
}

func (c *coverageData) Missed() int {
	missed := 0
	for _, v := range c.executableLines {
		if !v {
			missed++
		}
	}
	return missed
}
