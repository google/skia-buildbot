// Package parsers defines parsers for the paths of "import" statements found in TypeScript and Sass
// files.
//
// The ad-hoc parsers in this package utilize regular expressions to extract out import paths,
// filter out comments, etc. While these parsers do not capture every aspect of the TypeScript and
// Sass grammars, they are sufficient for the purpose of parsing the import statements in the
// TypeScript and Sass files found in our codebase.
//
// The following alternatives were ruled out because of their high implementation and maintenance
// cost:
//
//  - Using third-party parsers written in Go (none exist at this time).
//  - Generate real parsers using e.g. Goyacc (https://pkg.go.dev/golang.org/x/tools/cmd/goyacc).
//  - Use the TypeScript compiler API to inspect the AST of a TypeScript file (requires calling
//    Node.js code from Gazelle).
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

// tsImportRegexps contains all the regular expressions necessary to extract imports from a
// TypeScript source file.
var tsImportRegexps = []*regexp.Regexp{
	// Matches the following styles of imports:
	//     import * from 'foo';
	//     export * from 'foo';
	//     import * as bar from 'foo';
	//     import { bar, baz } from 'foo';
	//     import { bar, baz as qux } from 'foo';
	regexp.MustCompile(`^\s*(import|export)\s*(\*|[[:alnum:]]|_|\$|,|\{|\}|\s)*\s*from\s*'(?P<path>.*)'`), // Single quotes.
	regexp.MustCompile(`^\s*(import|export)\s*(\*|[[:alnum:]]|_|\$|,|\{|\}|\s)*\s*from\s*"(?P<path>.*)"`), // Double quotes.

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

// ignoredTsImportsRegexp matches import paths that should be ignored, namely CSS and Sass imports.
// Importing CSS and Sass files from TypeScript files is a Webpack idiom that both the TypeScript
// compiler and our front-end BUILD rules ignore, in favor of other mechanisms such as the
// sass_deps and sk_element_deps in various rules, and "ghost" Sass imports.
//
// See the sk_element macro definition for more, or go/skia-infra-bazel-frontend for the design.
var ignoredTsImportsRegexp = regexp.MustCompile(`\.s?css$`)

// ParseTSImports takes the contents of a TypeScript source file and extracts the verbatim paths of
// any imported modules.
func ParseTSImports(source string) []string {
	// Remove comments from the source file.
	lines := SplitLinesAndRemoveComments(source)

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

	// Filter out ignored imports, and sort imports lexicographically.
	var imports []string
	for path := range importsSet {
		if !ignoredTsImportsRegexp.MatchString(path) {
			imports = append(imports, path)
		}
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

// ParseSassImports takes the contents of a Sass source file and extracts the verbatim paths of any
// imported modules.
func ParseSassImports(source string) []string {
	// Remove comments from the source file.
	lines := SplitLinesAndRemoveComments(source)

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
			// We are not currently inside a /* block comment */. Does this line have a single-line block
			// comment?
			match := singleLineBlockCommentRegexp.FindStringSubmatch(line)
			if len(match) > 0 {
				// Remove the single-line block-comment and proceed as if it was never there.
				line = match[1] + match[2]
			}

			// Does a multi-line block-comment start on the current line?
			match = blockCommentStartRegexp.FindStringSubmatch(line)
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
