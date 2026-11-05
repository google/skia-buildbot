// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package codereview

import (
	"fmt"
	"os"
	"strings"

	"go.skia.org/infra/go/gerrit"
)

// reviewComment represents a validated code review comment with line/column
// range and optional fix replacements.
type reviewComment struct {
	Path         string                       `json:"path"`
	Range        gerrit.CommentRange          `json:"range"`
	Message      string                       `json:"message"`
	SuggestedFix []*gerrit.FixReplacementInfo `json:"suggested_fix"`
}

// resolveComments validates raw comments from the model against the CL's
// sourceFiles, filters out comments that do not touch any modified lines, and
// converts text replacements into Gerrit FixReplacementInfo ranges.
func resolveComments(raw []*modelComment, files map[string]*sourceFile) []*reviewComment {
	type byteSpan struct {
		start, end int
	}
	var out []*reviewComment
	for _, mc := range raw {
		f, ok := files[mc.Path]
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: skipping comment on unknown file %q\n", mc.Path)
			continue
		}
		msg := strings.TrimSpace(mc.Message)
		if msg == "" {
			continue
		}
		commentRange := f.lineRangeToCommentRange(mc.StartLine, mc.EndLine)
		touchesChanged := f.touchesChangedLine(commentRange)

		var fixReplacements []*gerrit.FixReplacementInfo
		spansByFile := make(map[string][]byteSpan)
		replacementsValid := len(mc.Replacements) > 0

		for _, rep := range mc.Replacements {
			repPath := rep.Path
			if repPath == "" {
				repPath = mc.Path
			}
			rf, ok := files[repPath]
			if !ok {
				fmt.Fprintf(os.Stderr, "Warning: replacement references unknown file %q; dropping suggested fix\n", repPath)
				replacementsValid = false
				break
			}
			oldText := strings.ReplaceAll(rep.OldText, "\r\n", "\n")
			newText := strings.ReplaceAll(rep.NewText, "\r\n", "\n")
			if oldText == "" || oldText == newText {
				replacementsValid = false
				break
			}
			hintLine := rep.StartLine
			if hintLine <= 0 && repPath == mc.Path {
				hintLine = mc.StartLine
			}
			origS, origE, found := rf.findOccurrence(oldText, hintLine)
			if !found {
				fmt.Fprintf(os.Stderr, "Warning: could not find old_text %q in %s near line %d; dropping suggested fix\n", oldText, repPath, hintLine)
				replacementsValid = false
				break
			}
			// Reject multi-line replacements where old_text begins at the start
			// of a line but stops mid-line, which happens when the model
			// accidentally truncates the last line of old_text.
			if strings.Contains(oldText, "\n") && (origS == 0 || rf.content[origS-1] == '\n') && origE < len(rf.content) && rf.content[origE] != '\n' {
				fmt.Fprintf(os.Stderr, "Warning: multi-line old_text %q in %s near line %d ends mid-line; dropping suggested fix\n", oldText, repPath, hintLine)
				replacementsValid = false
				break
			}

			// When deleting full line(s) (newText is empty and oldText starts
			// at a line boundary and is followed by '\n'), include the trailing
			// newline in the deleted span so an empty line is not left behind.
			if newText == "" && (origS == 0 || rf.content[origS-1] == '\n') && origE < len(rf.content) && rf.content[origE] == '\n' {
				origE++
			}

			// Gerrit rejects fix suggestions containing overlapping replacement
			// ranges within the same file.
			for _, existing := range spansByFile[repPath] {
				if max(origS, existing.start) < min(origE, existing.end) {
					fmt.Fprintf(os.Stderr, "Warning: overlapping replacements in %s near line %d; dropping suggested fix\n", repPath, hintLine)
					replacementsValid = false
					break
				}
			}
			if !replacementsValid {
				break
			}
			spansByFile[repPath] = append(spansByFile[repPath], byteSpan{start: origS, end: origE})

			repRange := rf.byteRangeToCommentRange(origS, origE)
			if rf.touchesChangedLine(repRange) {
				touchesChanged = true
			}
			fixReplacements = append(fixReplacements, &gerrit.FixReplacementInfo{
				Path:        repPath,
				Range:       repRange,
				Replacement: newText,
			})
		}

		if !touchesChanged {
			continue
		}
		// If any replacement in the fix failed validation, still post the
		// review comment itself without a suggested fix.
		if !replacementsValid {
			fixReplacements = []*gerrit.FixReplacementInfo{}
		}

		out = append(out, &reviewComment{
			Path:         mc.Path,
			Range:        commentRange,
			Message:      msg,
			SuggestedFix: fixReplacements,
		})
	}
	return out
}
