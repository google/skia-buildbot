package parsers

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

////////////////////////////////
// TypeScript imports parser. //
////////////////////////////////

var tsImportRegexps = []*regexp.Regexp{
	// Matches the following styles of imports:
	//     import * from 'foo';
	//     import * as bar from 'foo';
	//     import { bar, baz } from 'foo';
	//     import { bar, baz as qux } from 'foo';
	regexp.MustCompile(`^\s*import\s*{?(\*|[[:alnum:]]|_|\$|,|\s)*}?\s*from\s*'(?P<path>.*)'`), // Single quotes.
	regexp.MustCompile(`^\s*import\s*{?(\*|[[:alnum:]]|_|\$|,|\s)*}?\s*from\s*"(?P<path>.*)"`), // Double quotes.

	// Matches multiline imports, e.g.:
	//     import {
	//       bar,
	//       baz as qux,
	//     } from 'foo';
	regexp.MustCompile(`^\s*}?\s*from\s*'(?P<path>.*)'`), // Single quotes.
	regexp.MustCompile(`^\s*}?\s*from\s*"(?P<path>.*)"`), // Double quotes.

	// Matches imports for side-effects only, e.g.:
	//     import 'foo';
	regexp.MustCompile(`^\s*import\s*'(?P<path>.*)'`), // Single quotes.
	regexp.MustCompile(`^\s*import\s*"(?P<path>.*)"`), // Double quotes.
}

// parseTSImports takes the contents of a TypeScript source file and extracts the verbatim paths of
// any imported modules.
func parseTSImports(source string) []string {
	// Remove comments from the source file.
	lines := splitLinesAndRemoveComments(source)

	// Extract all imports.
	importsSet := map[string]bool{}
	for _, line := range lines {
		for _, re := range tsImportRegexps {
			match := re.FindStringSubmatch(line)
			if len(match) != 0 {
				importPath := match[len(match)-1] // The path is the last capture group on all regexps.
				importsSet[importPath] = true
			}
		}
	}

	// Sort imports lexicographically.
	var imports []string
	for path := range importsSet {
		imports = append(imports, path)
	}
	sort.Strings(imports)

	return imports
}

//////////////////////////
// Sass imports parser. //
//////////////////////////

// sassImportRegexps match the following kinds of Sass imports:
//
//     @import 'foo';
//     @use 'foo';
//     @use 'foo' as bar;
//     @use 'foo' with (
//       $bar: 1px
//     );
//     @forward 'foo';
//     @forward 'foo' as foo-*;
//     @forward 'foo' hide $bar, $baz;
//     @forward 'foo' with (
//       $bar: $1px
//     );
//
// See https://sass-lang.com/documentation/at-rules.
var sassImportRegexps = []*regexp.Regexp{
	regexp.MustCompile(`^\s*@(import|use|forward)\s*'(?P<path>[\w~_/\.\-]+)'`), // Single quotes.
	regexp.MustCompile(`^\s*@(import|use|forward)\s*"(?P<path>[\w~_/\.\-]+)"`), // Double quotes.
}

// parseSassImports takes the contents of a Sass source file and extracts the verbatim paths of any
// imported modules.
func parseSassImports(source string) []string {
	// Remove comments from the source file.
	lines := splitLinesAndRemoveComments(source)

	// Extract all imports.
	importsSet := map[string]bool{}
	for _, line := range lines {
		for _, re := range sassImportRegexps {
			match := re.FindStringSubmatch(line)
			if len(match) != 0 {
				importPath := match[len(match)-1] // The path is the last capture group on all regexps.
				// Filter out plain CSS imports. See
				// https://sass-lang.com/documentation/at-rules/import#plain-css-imports.
				if strings.HasSuffix(importPath, ".css") {
					continue
				}
				importsSet[importPath] = true
			}
		}
	}

	// Sort imports lexicographically.
	var imports []string
	for path := range importsSet {
		imports = append(imports, path)
	}
	sort.Strings(imports)

	return imports
}

///////////////////////////////////////////
// Functions for filtering out comments. //
///////////////////////////////////////////

// splitLinesAndRemoveComments deletes "// line comments" and "/* block comments */" from the given
// source, and splits the results into lines.
//
// This works for both TypeScript and Sass because both languages use the same syntax for comments.
func splitLinesAndRemoveComments(source string) []string {
	lines := strings.Split(source, "\n")
	lines = stripBlockComments(lines)
	lines = stripCommentedOutLines(lines)
	return lines
}

var (
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
			// We are not currently inside a /* block comment */. Does one start on the current line?
			match := blockCommentStartRegexp.FindStringSubmatch(line)
			if len(match) > 0 {
				// Block comment found. Keep the portion of the line that precedes the "/*" characters.
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
				// Found the end of the block comment. Keep the portion of the line that succeeds the "*/"
				// characters.
				blockComment = false
				outputLines = append(outputLines, match[1])
			} else {
				// We are still inside a block comment. The entire line can be discarded, so we do nothing.
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
