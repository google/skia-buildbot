package logs

import (
	"fmt"
	"strconv"
	"strings"
)

// LineRange represents a range of lines within a log.
type LineRange struct {
	Start int // inclusive
	End   int // exclusive
}

// RenderLine renders a single line of logs with line numbers for easier
// reference.
func RenderLine(line string, lineNum, totalLines int) string {
	padLength := len(strconv.Itoa(totalLines))
	padFmt := fmt.Sprintf("%%%dd", padLength)
	lineNumStr := fmt.Sprintf(padFmt, lineNum)
	return fmt.Sprintf("%s | %s", lineNumStr, line)
}

// RenderLineRange renders a range of log lines.
func RenderLineRange(lines []string, lineRange *LineRange) string {
	// Note: we number the lines starting at 1.
	rv := []string{
		fmt.Sprintf("==== Lines %d-%d of %d ====", lineRange.Start+1, lineRange.End, len(lines)),
	}
	for i := lineRange.Start; i < lineRange.End; i++ {
		rv = append(rv, RenderLine(lines[i], i+1, len(lines)))
	}
	return strings.Join(rv, "\n")
}

// RenderLineRanges renders multiple ranges of log lines.
func RenderLineRanges(lines []string, ranges []*LineRange) string {
	var rv []string
	for _, r := range ranges {
		rv = append(rv, RenderLineRange(lines, r))
	}
	return strings.Join(rv, "\n") + "\n"
}

// ExtractSnippets extracts interesting parts of the given log lines based on
// some heuristics.
func ExtractSnippets(lines []string, contextLines, includeLastN int) []*LineRange {
	totalLines := len(lines)
	if totalLines == 0 {
		return nil
	}
	if totalLines <= includeLastN {
		return []*LineRange{
			{
				Start: 0,
				End:   totalLines,
			},
		}
	}

	var snippets []*LineRange
	for i, line := range lines[:totalLines-includeLastN] {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "fatal") {
			start := i - contextLines
			if start < 0 {
				start = 0
			}
			end := i + contextLines + 1
			if end >= totalLines {
				end = totalLines
			}
			snippets = append(snippets, &LineRange{
				Start: start,
				End:   end,
			})
		}
	}

	// Include the last N lines.
	if includeLastN > 0 {
		snippets = append(snippets, &LineRange{
			Start: totalLines - includeLastN,
			End:   totalLines,
		})
	}

	// Merge overlapping snippets.
	merged := make([]*LineRange, 0, len(snippets))
	for _, s := range snippets {
		if len(merged) > 0 && merged[len(merged)-1].End >= s.Start {
			merged[len(merged)-1].End = s.End
		} else {
			merged = append(merged, s)
		}
	}

	return merged
}
