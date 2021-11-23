// Package parsers defines parsers for the paths of #include statements from C++ header and
// source files.
//
// It makes use of regular expressions. These should be powerful enough for the job, but not
// too complicated, because the C++ code this extension is meant to work on follows some style
// guidelines which mean we do not necessarily have to use LLVM or similar to properly run the
// preprocessor directives before figuring out the includes.
package parsers

import (
	"regexp"
	"sort"

	"go.skia.org/infra/bazel/gazelle/cpp/common"
	"go.skia.org/infra/bazel/gazelle/frontend/parsers"
)

var repoIncludeRegex = regexp.MustCompile(`^\s*#\s*include\s+"(?P<file>.+)"`)
var systemIncludeRegex = regexp.MustCompile(`^\s*#\s*include\s+<(?P<file>.+)>`)

func ParseCIncludes(source string) ([]string, []string) {
	// Remove comments from the source file.
	lines := parsers.SplitLinesAndRemoveComments(source)

	// Extract all imports.
	repoIncludeSet := map[string]bool{}
	systemIncludeSet := map[string]bool{}
	for _, line := range lines {
		match := repoIncludeRegex.FindStringSubmatch(line)
		if len(match) != 0 {
			importPath := match[1]
			// Ignore includes of files which are not C++ files. These will likely need to
			// be listed as textual_hdrs to make Bazel happy.
			// https://docs.bazel.build/versions/main/be/c-cpp.html#cc_library.textual_hdrs
			if !common.IsCppHeader(importPath) && !common.IsCppSource(importPath) {
				continue
			}
			repoIncludeSet[importPath] = true
		}
		match = systemIncludeRegex.FindStringSubmatch(line)
		if len(match) != 0 {
			importPath := match[1]
			systemIncludeSet[importPath] = true
		}
	}

	return setToSlice(repoIncludeSet), setToSlice(systemIncludeSet)
}

func setToSlice(set map[string]bool) []string {
	var slice []string
	for path := range set {
		slice = append(slice, path)
	}
	sort.Strings(slice)
	return slice
}
