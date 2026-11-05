// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package codereview

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.skia.org/infra/go/gerrit"
)

// sourceFile holds the contents of a file at the patchset revision along with
// precomputed line slices, byte offsets, and which 1-based lines were modified.
type sourceFile struct {
	path         string
	content      string
	lines        []string
	lineOffsets  []int
	changedLines map[int]bool
}

// newSourceFile constructs a sourceFile and precomputes the starting byte
// offset of each line in content.
func newSourceFile(path, content string, changedLines map[int]bool) *sourceFile {
	// Normalize CRLF to LF so byte offsets and substring searches use single-byte '\n'.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	offsets := make([]int, len(lines))
	pos := 0
	for i, l := range lines {
		offsets[i] = pos
		pos += len(l) + 1 // +1 accounts for the '\n' delimiter.
	}
	return &sourceFile{
		path:         path,
		content:      content,
		lines:        lines,
		lineOffsets:  offsets,
		changedLines: changedLines,
	}
}

// renderAnnotated formats the file with 1-based line numbers and a leading '+'
// marker on lines added or modified by the CL, for inclusion in the LLM prompt.
func (f *sourceFile) renderAnnotated() string {
	var sb strings.Builder
	for i, line := range f.lines {
		marker := ' '
		if f.changedLines[i+1] {
			marker = '+'
		}
		fmt.Fprintf(&sb, "%c %4d: %s\n", marker, i+1, line)
	}
	return sb.String()
}

// byteOffsetToLineAndColumn converts a 0-based byte offset in f.content into
// a 1-based line number and 0-based UTF-8 character (rune) column, as expected
// by Gerrit's CommentRange.
func (f *sourceFile) byteOffsetToLineAndColumn(b int) (int, int) {
	if b < 0 {
		b = 0
	}
	if b > len(f.content) {
		b = len(f.content)
	}
	// Binary search for the first line that starts after byte offset b; the
	// preceding line is the one containing b.
	lineIdx := sort.Search(len(f.lineOffsets), func(i int) bool {
		return f.lineOffsets[i] > b
	}) - 1
	if lineIdx < 0 {
		lineIdx = 0
	}
	// Gerrit column offsets are measured in Unicode code points, not bytes.
	col := utf8.RuneCountInString(f.content[f.lineOffsets[lineIdx]:b])
	return lineIdx + 1, col
}

// byteRangeToCommentRange converts a half-open byte span [startOffset, endOffset)
// in f.content into a Gerrit CommentRange.
func (f *sourceFile) byteRangeToCommentRange(startOffset, endOffset int) gerrit.CommentRange {
	startLine, startCol := f.byteOffsetToLineAndColumn(startOffset)
	endLine, endCol := f.byteOffsetToLineAndColumn(endOffset)
	return gerrit.CommentRange{
		StartLine:      startLine,
		StartCharacter: startCol,
		EndLine:        endLine,
		EndCharacter:   endCol,
	}
}

// lineRangeToCommentRange builds a whole-line Gerrit CommentRange spanning
// 1-based lines [startLine, endLine] inclusive, clamping out-of-bounds line
// numbers to the file's line count.
func (f *sourceFile) lineRangeToCommentRange(startLine, endLine int) gerrit.CommentRange {
	if startLine < 1 {
		startLine = 1
	}
	if startLine > len(f.lines) {
		startLine = len(f.lines)
	}
	if endLine < startLine {
		endLine = startLine
	}
	if endLine > len(f.lines) {
		endLine = len(f.lines)
	}
	return gerrit.CommentRange{
		StartLine:      startLine,
		StartCharacter: 0,
		EndLine:        endLine,
		EndCharacter:   utf8.RuneCountInString(f.lines[endLine-1]),
	}
}

// touchesChangedLine reports whether the given CommentRange overlaps or is
// immediately adjacent (±1 line) to any line modified in the CL.
func (f *sourceFile) touchesChangedLine(r gerrit.CommentRange) bool {
	for l := r.StartLine - 1; l <= r.EndLine+1; l++ {
		if f.changedLines[l] {
			return true
		}
	}
	return false
}

// findOccurrence searches f.content for oldText and returns the half-open byte
// range [start, end) of the match. If oldText appears multiple times, the
// occurrence closest to the 1-based hintStartLine is selected.
func (f *sourceFile) findOccurrence(oldText string, hintStartLine int) (int, int, bool) {
	if oldText == "" {
		return 0, 0, false
	}
	bestIdx := -1
	bestDist := 0
	searchFrom := 0
	for {
		idx := strings.Index(f.content[searchFrom:], oldText)
		if idx < 0 {
			break
		}
		absIdx := searchFrom + idx
		line, _ := f.byteOffsetToLineAndColumn(absIdx)
		dist := absInt(line - hintStartLine)
		if bestIdx < 0 || (hintStartLine > 0 && dist < bestDist) {
			bestIdx = absIdx
			bestDist = dist
		}
		searchFrom = absIdx + 1
	}
	if bestIdx < 0 {
		return 0, 0, false
	}
	return bestIdx, bestIdx + len(oldText), true
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// parseChangedLinesFromPatch parses a unified git diff and returns:
//  1. A map from file path to the set of 1-based line numbers added in the new
//     version of that file.
//  2. The ordered slice of modified/added file paths (excluding deleted files).
func parseChangedLinesFromPatch(patch string) (map[string]map[int]bool, []string) {
	changedByFile := make(map[string]map[int]bool)
	var fileOrder []string
	seenFile := make(map[string]bool)

	var currentFile string
	newLine := 0
	inHunk := false

	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			currentFile = ""
			inHunk = false
		case strings.HasPrefix(line, "+++ "):
			// Target file header, e.g. "+++ b/src/foo.cpp" or "+++ /dev/null" (if deleted).
			inHunk = false
			target := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			if target == "/dev/null" {
				currentFile = ""
				continue
			}
			currentFile = strings.TrimPrefix(target, "b/")
			if !seenFile[currentFile] {
				seenFile[currentFile] = true
				fileOrder = append(fileOrder, currentFile)
				changedByFile[currentFile] = make(map[int]bool)
			}
		case strings.HasPrefix(line, "@@ ") && currentFile != "":
			// Unified diff hunk header, e.g. "@@ -10,5 +12,7 @@" or "@@ -1 +1 @@".
			// Extract the starting line number on the new ("+") side.
			plusIdx := strings.Index(line, "+")
			if plusIdx < 0 {
				inHunk = false
				continue
			}
			rest := line[plusIdx+1:]
			endIdx := strings.IndexAny(rest, ", ")
			if endIdx > 0 {
				rest = rest[:endIdx]
			}
			start, err := strconv.Atoi(rest)
			if err != nil {
				inHunk = false
				continue
			}
			newLine = start
			inHunk = true
		case inHunk && currentFile != "":
			// Track line numbers on the new side of the diff: '+' lines are
			// newly added/modified, ' ' lines are unchanged context, and '-'
			// lines only exist on the old side so they don't advance newLine.
			if strings.HasPrefix(line, "+") {
				changedByFile[currentFile][newLine] = true
				newLine++
			} else if strings.HasPrefix(line, " ") {
				newLine++
			} else if strings.HasPrefix(line, "-") || strings.HasPrefix(line, `\`) {
				// Deleted line or "\ No newline at end of file".
			} else {
				inHunk = false
			}
		}
	}
	return changedByFile, fileOrder
}
