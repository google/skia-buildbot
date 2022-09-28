// Package bloaty provides functions to parse the output of Bloaty[1] and turn it into a data table
// that can be displayed on a web page with google.visualization.TreeMap[2].
//
// Most code in this package is a 1:1 Golang port of [3]. The following TODOs are copied from [3]:
//
// TODO(skbug.com/12151): Deal with symbols vs. fullsymbols, even both?
// TODO(skbug.com/12151): Support aggregation by scope, rather than file (split C++ identifiers on
//                        '::')
// TODO(skbug.com/12151): Deal with duplicate symbols better. These are actually good targets for
//                        optimization. They are sometimes static functions in headers (so they
//                        appear in multiple .o files). There are also symbols that appear multiple
//                        times due to inlining (eg, kNoCropRect).
// TODO(skbug.com/12151): Figure out why some symbols are misattributed. Eg, Swizzle::Convert and
//                        ::Make are tied to the header by nm, and then to one caller (at random) by
//                        Bloaty. They're not inlined, though. Unless LTO is doing something wacky
//                        here? Scope-aggregation may be the answer? Ultimately, this seems like an
//                        issue with Bloaty and/or debug information itself.
//
// [1] https://github.com/google/bloaty
// [2] https://developers.google.com/chart/interactive/docs/gallery/treemap
// [3] https://skia.googlesource.com/skia/+/5a7d91c35beb48afce9362852f1f5e26f7550ba8/tools/bloaty_treemap.py

package bloaty

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
)

// OutputItem represents a single line in a Bloaty output file.
type OutputItem struct {
	// CompileUnit is the source file where a symbol is found, e.g. "src/core/SkBuffer.cpp".
	CompileUnit string

	// Symbol is the name of a symbol, e.g. "SkRBuffer::read()"".
	Symbol string

	// VirtualMemorySize is the number of bytes the binary takes when it is loaded into memory.
	VirtualMemorySize int

	// FileSize is the number of bytes the binary takes on disk.
	FileSize int
}

const (
	// expectedHeaderLine is the header line we expect to find in a Bloaty output file. We use
	// this to ensure that Bloaty was invoked with the expected command-line flags.
	expectedHeaderLine = "compileunits\tsymbols\tvmsize\tfilesize"

	androidBuildPrefix = "/buildbot/src/android/"
)

// ParseTSVOutput parses the TSV output of Bloaty.
//
// The parameter should contain the output of a Bloaty invocation such as the following:
//
//     $ bloaty <path/to/binary> -d compileunits,symbols -n 0 --tsv
func ParseTSVOutput(bloatyOutput string) ([]OutputItem, error) {
	if strings.TrimSpace(bloatyOutput) == "" {
		return nil, skerr.Fmt("empty input")
	}

	var items []OutputItem

	for i, line := range strings.Split(strings.TrimSpace(bloatyOutput), "\n") {
		errOnLine := func(msg string, args ...interface{}) error {
			allArgs := append([]interface{}{i + 1}, args...)
			return skerr.Fmt("on line %d: "+msg, allArgs...)
		}

		wrapErrOnLine := func(err error, msg string, args ...interface{}) error {
			allArgs := append([]interface{}{i + 1}, args...)
			return skerr.Wrapf(err, "on line %d: "+msg, allArgs...)
		}

		if i == 0 {
			// We check that the input file is in the expected format by looking at the header line.
			//
			// In the future, we could use this to automatically detect the source columns.
			if line != expectedHeaderLine {
				return nil, errOnLine("unrecognized header format; must be: %q", expectedHeaderLine)
			}

			// Skip the header line.
			continue
		}

		cols := strings.Split(line, "\t")
		if len(cols) != 4 {
			return nil, errOnLine("expected 4 columns, got %d", len(cols))
		}

		item := OutputItem{
			CompileUnit: cols[0],
			Symbol:      cols[1],
		}

		if item.Symbol == item.CompileUnit {
			// Bloaty 1.0 would seem to group unknown symbols into a "symbol" with the same
			// name as the section/compile unit. For example,
			// [section .rodata]	[section .rodata]	10425940	10425940
			// Having a CompileUnit and Symbol with the same name causes problems for the treemap
			// as well as understanding what is going on, so we add a prefix to the symbol
			item.Symbol = "UNKNOWN " + item.Symbol
		} else if strings.HasPrefix(item.CompileUnit, "[section") {
			// Some symbols can be in multiple sections. For example, source code can live in
			// [section .text], unwind information in [section .eh_frame], and unwind metadata in
			// [section .eh_frame_hdr]. Thus, we add a suffix for any data in sections, so as not
			// to have duplicate symbols which makes the treemap sad.
			item.Symbol += " " + item.CompileUnit
		}
		// In arm outputs, we see sections show up, which causes cycles in the treemap.
		// Thus, we skip them. They seem pretty small too.
		if strings.HasPrefix(item.Symbol, "[section") {
			continue
		}

		var err error
		item.VirtualMemorySize, err = strconv.Atoi(cols[2])
		if err != nil {
			return nil, wrapErrOnLine(err, "could not convert vmsize column to integer")
		}

		item.FileSize, err = strconv.Atoi(cols[3])
		if err != nil {
			return nil, wrapErrOnLine(err, "could not convert filesize column to integer")
		}

		// Strip the leading "../../" from paths.
		for strings.HasPrefix(item.CompileUnit, "../") {
			item.CompileUnit = strings.TrimPrefix(item.CompileUnit, "../")
		}
		// Sometimes bloaty's file paths for third_party include the skia folder and sometimes
		// they do not. We would prefer all third_party stuff show up seperate from Skia.
		if strings.HasPrefix(item.CompileUnit, "skia/third_party/") {
			item.CompileUnit = strings.TrimPrefix(item.CompileUnit, "skia/")
		}

		// Files in third_party and from the NDK sometimes have absolute paths. Strip those.
		if path.IsAbs(item.CompileUnit) {
			item.CompileUnit = path.Clean(item.CompileUnit)
			if idx := strings.Index(item.CompileUnit, "third_party"); idx != -1 {
				item.CompileUnit = item.CompileUnit[idx:]
			} else if strings.HasPrefix(item.CompileUnit, androidBuildPrefix) {
				item.CompileUnit = strings.TrimPrefix(item.CompileUnit, androidBuildPrefix)
			} else {
				return nil, errOnLine("unexpected absolute path %q", line)
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// TreeMapDataTableRow represents a "row" in a two-dimensional JavaScript array suitable for passing
// to google.visualization.arrayToDataTable()[1].
//
// [1] https://developers.google.com/chart/interactive/docs/reference#arraytodatatable
type TreeMapDataTableRow struct {
	// Name is the unique, non-empty name of the node. Uniqueness is necessary, otherwise
	// google.visualization.TreeView will throw an exception.
	Name string `json:"name"`

	// Parent is the name of the parent node. The root node will have an empty parent. Client code
	// must turn the parent node into a null JavaScript value, otherwise google.visualization.TreeView
	// will throw an exception.
	Parent string `json:"parent"`

	// Size is the size in bytes of the node. Non-symbol nodes (e.g. paths) should be 0.
	Size int `json:"size"`
}

// GenTreeMapDataTableRows takes a parsed Bloaty output and returns a slice of "row" structs that
// can be converted into a two-dimensional JavaScript array suitable for passing to
// google.visualization.arrayToDataTable()[1]. The resulting DataTable can then be passed to a
// google.visualization.TreeMap[2].
//
// Callers should prepend a header row such as ['Name', 'Parent', 'Size'] to the two-dimensional
// JavaScript array, and must convert any parent fields with empty strings to null JavaScript values
// before passing the array to google.visualization.arrayToDataTable().
//
// [1] https://developers.google.com/chart/interactive/docs/reference#arraytodatatable
// [2] https://developers.google.com/chart/interactive/docs/gallery/treemap
func GenTreeMapDataTableRows(items []OutputItem, maxChildren int) []TreeMapDataTableRow {
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		// Sort in order of ascending compile unit, descending file size, ascending symbol
		if a.CompileUnit != b.CompileUnit {
			return a.CompileUnit < b.CompileUnit
		}
		if a.FileSize != b.FileSize {
			return a.FileSize > b.FileSize
		}
		return a.Symbol < b.Symbol
	})
	rows := []TreeMapDataTableRow{
		{
			Name:   "ROOT",
			Parent: "",
			Size:   0,
		},
	}

	parentDirMap := map[string]string{}

	// addCompileUnitOrParentDir outputs rows to the data table establishing the node hierarchy, and
	// ensures that each line is emitted exactly once.
	//
	// Example for a given path "foo/bar/baz.cpp":
	//
	//     ['foo/bar/baz.cpp', 'foo/bar', 0],
	//     ['foo/bar',         'foo',     0],
	//     ['foo',             'ROOT',    0],
	var addCompileUnitOrParentDir func(fileOrDir string)
	addCompileUnitOrParentDir = func(fileOrDir string) {
		_, ok := parentDirMap[fileOrDir]
		if !ok {
			parentDir, _ := path.Split(fileOrDir)
			parentDir = path.Clean(parentDir) // Remove trailing slashes ("/a/b/" becomes "/a/b").
			if parentDir == "." {
				parentDirMap[fileOrDir] = "ROOT"
			} else {
				addCompileUnitOrParentDir(parentDir)
				parentDirMap[fileOrDir] = parentDir
			}

			parent := parentDirMap[fileOrDir]
			rows = append(rows, TreeMapDataTableRow{
				Name:   fileOrDir,
				Parent: parent,
				Size:   0,
			})
		}
	}

	symbolFreqs := map[string]int{}
	parentsToChildCounts := map[string]int{}
	overflowedParentsToSize := map[string]int{}

	for _, item := range items {
		addCompileUnitOrParentDir(item.CompileUnit)

		// Make the symbol name unique by appending a number to repeated symbols (a repeated "foo"
		// symbol becomes "foo_1", "foo_2", etc.) This is important because google.visualization.TreeMap
		// requires node names to be unique.
		if freq, ok := symbolFreqs[item.Symbol]; !ok {
			symbolFreqs[item.Symbol] = 1
		} else {
			symbolFreqs[item.Symbol] = freq + 1
			item.Symbol = fmt.Sprintf("%s_%d", item.Symbol, freq)
		}

		if parentsToChildCounts[item.CompileUnit] >= maxChildren {
			overflowedParentsToSize[item.CompileUnit] += item.FileSize
		} else {
			rows = append(rows, TreeMapDataTableRow{
				Name:   item.Symbol,
				Parent: item.CompileUnit,
				Size:   item.FileSize,
			})
		}
		parentsToChildCounts[item.CompileUnit]++
	}

	for parent, size := range overflowedParentsToSize {
		rows = append(rows, TreeMapDataTableRow{
			Name:   fmt.Sprintf("remainder (%s)", parent),
			Parent: parent,
			Size:   size,
		})
	}

	return rows
}
