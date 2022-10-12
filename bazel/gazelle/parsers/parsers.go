package parsers

import (
	"fmt"
	"regexp"
	"strings"
)

// SplitLinesAndRemoveComments takes a multiline string, splits it into lines, and returns two
// equal-length arrays of strings: one with the original lines, and another one with the original
// lines minus any "// line comments" and "/* block comments */".
//
// This should work for any language that uses C++-style comments, including TypeScript and Sass.
func SplitLinesAndRemoveComments(source string) ([]string, []string) {
	verbatim := strings.Split(source, "\n")
	noComments := stripBlockComments(verbatim)
	noComments = stripCommentedOutLines(noComments)
	return verbatim, noComments
}

var (
	// singleLineBlockCommentRegexp matches a single-line /* block comment */, and captures any
	// uncommented code before and after the block comment.
	//
	// Known limitation: This regexp ignores string literals.
	singleLineBlockCommentRegexp = regexp.MustCompile(`(?P<uncommented_before>.*)/\*.*\*/(?P<uncommented_after>.*)`)

	// blockCommentStartRegexp matches the "/*" at the beginning of a /* block comment */, and
	// captures any uncommented code that precedes it.
	//
	// Known limitation: This regexp ignores the beginning of a block comment if it is preceded by a
	// single or double quote, as the block comment itself might be part of a string literal.
	blockCommentStartRegexp = regexp.MustCompile(fmt.Sprintf(`(?P<uncommented>[^'"%s]*)/\*`, "`"))

	// blockCommentEndRegexp matches the "*/" at the end of a /* block comment */, and captures any
	// uncommented code that succeeds it.
	blockCommentEndRegexp = regexp.MustCompile(`\*/(?P<uncommented>.*)`)
)

// stripBlockComments strips /* block comments */ from the given lines of code. Any commented-out
// lines are replaced by blank lines. Thus, the returned array has the same length as the input
// array.
func stripBlockComments(lines []string) []string {
	var outputLines []string
	blockComment := false // Keeps track of whether we're currently inside a /* block comment */.

	for _, line := range lines {
		if !blockComment {
			// We are not currently inside a /* block comment */. Does this line have one or more
			// single-line block comments?
			match := singleLineBlockCommentRegexp.FindStringSubmatch(line)
			for len(match) > 0 {
				// Remove the single-line block-comment and proceed as if it was never there.
				line = match[1] + match[2]
				match = singleLineBlockCommentRegexp.FindStringSubmatch(line)
			}

			// Does a multi-line block-comment start on the current line?
			match = blockCommentStartRegexp.FindStringSubmatch(line)
			if len(match) > 0 {
				// Block comment found. Keep the portion of the line that precedes the
				// "/*" characters.
				blockComment = true
				outputLines = append(outputLines, match[1])
			} else {
				// No block comment found. We can keep the current line as-is.
				outputLines = append(outputLines, line)
			}
		} else {
			// We are currently inside a /* block comment */. Does it end on the current line?
			match := blockCommentEndRegexp.FindStringSubmatch(line)
			if len(match) > 0 {
				// Found the end of the block comment. Keep the portion of the line that succeeds
				// the "*/" characters.
				blockComment = false
				outputLines = append(outputLines, match[1])
			} else {
				// We are still inside a block comment. The current line is replaced with a blank
				// line.
				outputLines = append(outputLines, "")
			}
		}
	}

	return outputLines
}

// commentedOutLineRegexp matches lines that are commented out via a single-line comment.
var commentedOutLineRegexp = regexp.MustCompile(`^\s*//`)

// stripCommentedOutLines replaces any lines beginning with a "//" comment with an empty line. The
// returned array has the same length as the input array.
func stripCommentedOutLines(lines []string) []string {
	var outputLines []string
	for _, line := range lines {
		if commentedOutLineRegexp.MatchString(line) {
			outputLines = append(outputLines, "")
		} else {
			outputLines = append(outputLines, line)
		}
	}
	return outputLines
}
