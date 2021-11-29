package parsers

import (
	"fmt"
	"regexp"
	"strings"
)

// SplitLinesAndRemoveComments deletes "// line comments" and "/* block comments */" from the given
// source, and splits the results into lines.
//
// This works for any language that uses that style of comments, including Typescript, Sass, C++.
func SplitLinesAndRemoveComments(source string) []string {
	lines := strings.Split(source, "\n")
	lines = stripBlockComments(lines)
	lines = stripCommentedOutLines(lines)
	return lines
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

// stripBlockComments strips /* block comments */ from the given lines of code.
func stripBlockComments(lines []string) []string {
	var outputLines []string
	blockComment := false // Keeps track of whether we're currently inside a /* block comment */.

	for _, line := range lines {
		if !blockComment {
			// We are not currently inside a /* block comment */. Does this line have a single-line
			// block comment?
			match := singleLineBlockCommentRegexp.FindStringSubmatch(line)
			if len(match) > 0 {
				// Remove the single-line block-comment and proceed as if it was never there.
				line = match[1] + match[2]
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
				// We are still inside a block comment. The entire line can be discarded, so we
				// do nothing.
			}
		}
	}

	return outputLines
}

// commentedOutLineRegexp matches lines that are commented out via a single-line comment.
var commentedOutLineRegexp = regexp.MustCompile(`^\s*//`)

// stripCommentedOutLines strips out any lines that begin with a "//" single-line comment.
func stripCommentedOutLines(lines []string) []string {
	var outputLines []string
	for _, line := range lines {
		if !commentedOutLineRegexp.MatchString(line) {
			outputLines = append(outputLines, line)
		}
	}
	return outputLines
}
