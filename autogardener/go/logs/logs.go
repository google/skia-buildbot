package logs

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// LineRange represents a range of lines within a log.
type LineRange struct {
	Start int // inclusive
	End   int // exclusive
}

// LineRanges implements sort.Interface.
type LineRanges []*LineRange

func (lr LineRanges) Len() int           { return len(lr) }
func (lr LineRanges) Less(i, j int) bool { return lr[i].Start < lr[j].Start }
func (lr LineRanges) Swap(i, j int)      { lr[i], lr[j] = lr[j], lr[i] }

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
func ExtractSnippets(lines []string, contextLines, includeLastN, maxSnippetLines, maxSnippets int) []*LineRange {
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

	// First, find all of the interesting lines. Deduplicate exactly-matching
	// lines, keeping only the first occurrence.
	interestingLines := map[string]int{}
	for index, line := range lines[:totalLines-includeLastN] {
		if IsLogLineInteresting(line) {
			if _, ok := interestingLines[line]; !ok {
				interestingLines[line] = index
			}
		}
	}

	// Take the specified number of contextLines around each of the interesting
	// lines to create LineRanges.
	snippets := make([]*LineRange, 0, len(interestingLines)+1)
	for _, index := range interestingLines {
		start := index - contextLines
		if start < 0 {
			start = 0
		}
		end := index + contextLines + 1
		if end >= totalLines {
			end = totalLines
		}
		snippets = append(snippets, &LineRange{
			Start: start,
			End:   end,
		})
	}

	// Include the last N lines.
	if includeLastN > 0 {
		snippets = append(snippets, &LineRange{
			Start: totalLines - includeLastN,
			End:   totalLines,
		})
	}

	// Merge overlapping snippets.
	sort.Sort(LineRanges(snippets))
	merged := make([]*LineRange, 0, len(snippets))
	for _, s := range snippets {
		if len(merged) > 0 && merged[len(merged)-1].End >= s.Start {
			merged[len(merged)-1].End = s.End
		} else {
			merged = append(merged, s)
		}
	}

	// Truncate excessively-long line ranges. Bias toward the end of the log,
	// since that's where real errors are most likely to occur.
	if maxSnippets > 0 && len(merged) > maxSnippets {
		merged = merged[len(merged)-maxSnippets:]
	}
	for _, snippet := range merged {
		if snippet.End-snippet.Start > maxSnippetLines {
			snippet.Start = snippet.End - maxSnippetLines
		}
	}

	return merged
}

var interestingLogLineRegex = regexp.MustCompile(`(?i)\b(error|fail|fatal)`)

func IsLogLineInteresting(line string) bool {
	return interestingLogLineRegex.MatchString(line)
}
