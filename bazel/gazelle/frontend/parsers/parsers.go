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
	"regexp"
	"sort"
	"strings"

	"go.skia.org/infra/bazel/gazelle/parsers"
)

////////////////////////////////
// TypeScript imports parser. //
////////////////////////////////

// tsImportRegexps contains all the regular expressions necessary to extract imports from a
// TypeScript source file.
var tsImportRegexps = []*regexp.Regexp{
	// Matches the following styles of imports:
	//
	//     import * from 'foo';
	//     export * from 'foo';
	//     import * as bar from 'foo';
	//     import { bar, baz } from 'foo';
	//     import { bar, baz as qux } from 'foo';
	//
	// All of the above imports can be ignored with a "gazelle:ignore" comment, e.g.:
	//
	//     import * from 'foo'; // gazelle:ignore
	regexp.MustCompile(`^\s*(import|export)\s*(\*|[[:alnum:]]|_|\$|,|\{|\}|\s)*\s*from\s*'(?P<path>.*)'`), // Single quotes.
	regexp.MustCompile(`^\s*(import|export)\s*(\*|[[:alnum:]]|_|\$|,|\{|\}|\s)*\s*from\s*"(?P<path>.*)"`), // Double quotes.

	// Matches multiline imports, e.g.:
	//
	//     import {
	//       bar,
	//       baz as qux,
	//     } from 'foo';
	//
	// Multiline imports can also be ignored with a "gazelle:ignore" comment, e.g.:
	//
	//     import {
	//       bar,
	//       baz as qux,
	//     } from 'foo'; // gazelle:ignore
	regexp.MustCompile(`^\s*}?\s*from\s*'(?P<path>.*)'`), // Single quotes.
	regexp.MustCompile(`^\s*}?\s*from\s*"(?P<path>.*)"`), // Double quotes.

	// Matches imports for side-effects only, e.g.:
	//
	//     import 'foo';
	//
	// These can also be ignored via a "gazelle:ignore" comment, e.g.:
	//
	//     import 'foo'; // gazelle:ignore
	regexp.MustCompile(`^\s*import\s*'(?P<path>.*)'`), // Single quotes.
	regexp.MustCompile(`^\s*import\s*"(?P<path>.*)"`), // Double quotes.
}

// tsGazelleIgnoreCommentRegexps matches "gazelle:ignore" comments.
var tsGazelleIgnoreCommentRegexps = []*regexp.Regexp{
	regexp.MustCompile(`.*//.*gazelle:ignore.*`),       // Line comment.
	regexp.MustCompile(`.*/\*.*gazelle:ignore.*\*/.*`), // Single-line block comment.
}

// ParseTSImports takes the contents of a TypeScript source file and extracts the verbatim paths of
// any imported modules. Import statements ending with a "gazelle:ignore" comment are ignored.
func ParseTSImports(source string) []string {
	// Remove comments from the source file.
	verbatimLines, linesWithCommentsRemoved := parsers.SplitLinesAndRemoveComments(source)

	// Extract all imports.
	importsSet := map[string]bool{}
	for i, line := range linesWithCommentsRemoved {
		for _, importRegexp := range tsImportRegexps {
			match := importRegexp.FindStringSubmatch(line)
			if len(match) != 0 {
				importPath := match[len(match)-1] // The path is the last capture group on all TS import regexps.

				// Is this import statement ignored via a "gazelle:ignored" comment?
				isIgnored := false
				for _, ignoreRegexp := range tsGazelleIgnoreCommentRegexps {
					// We need to test against the verbatim line, as it preserves all comments.
					match := ignoreRegexp.FindStringSubmatch(verbatimLines[i])
					if len(match) != 0 {
						isIgnored = true
					}
				}

				if !isIgnored {
					importsSet[importPath] = true
				}
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
	regexp.MustCompile(`^\s*@(?P<rule>import|use|forward)\s*'(?P<path>[\w~_/\.\-]+)'`), // Single quotes.
	regexp.MustCompile(`^\s*@(?P<rule>import|use|forward)\s*"(?P<path>[\w~_/\.\-]+)"`), // Double quotes.
}

// ParseSassImports takes the contents of a Sass source file and extracts the verbatim paths of any
// imported modules.
func ParseSassImports(source string) []string {
	// Remove comments from the source file.
	_, linesWithCommentsRemoved := parsers.SplitLinesAndRemoveComments(source)

	// Extract all imports.
	importsSet := map[string]bool{}
	for _, line := range linesWithCommentsRemoved {
		for _, re := range sassImportRegexps {
			match := re.FindStringSubmatch(line)
			if len(match) != 0 {
				rule := match[1] // Either "import", "use", or "forward".
				importPath := match[2]
				// Filter out plain CSS imports. See
				// https://sass-lang.com/documentation/at-rules/import#plain-css-imports.
				if rule == "import" && strings.HasSuffix(importPath, ".css") {
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
