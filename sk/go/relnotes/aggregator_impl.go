// Copyright 2023 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package relnotes

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
)

type AggregatorImpl struct {
}

const whitespaceChars = "\n\r\t "

var milestoneRegex = regexp.MustCompile(`^Milestone (?P<num>\d+)$`)

// isNotesFile determines if a filename represents a release notes file.
func isNotesFile(filename string) bool {
	b := path.Base(filename)
	return b != "README.md" && b != ".md" && filepath.Ext(b) == ".md"
}

// getMilestone will return the milestone number in a single line of
// text. If none is found -1 will be returned.
func getMilestone(line string) int {
	if match := milestoneRegex.FindStringSubmatch(line); len(match) > 0 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			return val
		}
	}
	return -1
}

// NewAggregator creates a new relnotes AggregatorImpl object that can be used
// to aggregate release notes.
func NewAggregator() *AggregatorImpl {
	return &AggregatorImpl{}
}

// provide implementation of ListNoteFiles.
func (a *AggregatorImpl) ListNoteFiles(ctx context.Context, fs vfs.FS, notesDir string) ([]string, error) {
	infos, err := vfs.ReadDir(ctx, fs, notesDir)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failure listing notes directory %q", notesDir)
	}
	baseNames := make([]string, 0, len(infos))
	for _, info := range infos {
		if info.IsDir() || !isNotesFile(info.Name()) {
			continue
		}
		baseNames = append(baseNames, info.Name())
	}
	sort.Strings(baseNames)
	return baseNames, nil
}

// Read all notes and return a slice containing the raw file contents. This
// slice is ordered by the filenames from which they are read.
func (a *AggregatorImpl) readAllNotes(ctx context.Context, fs vfs.FS, notesDir string) ([][]byte, error) {
	baseNames, err := a.ListNoteFiles(ctx, fs, notesDir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	fileContents := make([][]byte, 0, len(baseNames))
	for _, fname := range baseNames {
		// Hard code slash path separator instead of path.Join() to simplify tests.
		notePath := fmt.Sprintf("%s/%s", notesDir, fname)
		data, err := vfs.ReadFile(ctx, fs, notePath)
		if err != nil {
			return nil, skerr.Wrapf(err, "Error reading %q", notePath)
		}
		fileContents = append(fileContents, data)
	}
	return fileContents, nil
}

// writeNote will write a single raw release note, as read from
// its file, using the writer identified by |w|, as an unordered
// list entry in Markdown format.
func writeNote(noteData []byte, w io.Writer) error {
	if len(noteData) == 0 {
		return skerr.Fmt("No note data")
	}
	if _, err := fmt.Fprint(w, "  * "); err != nil {
		return skerr.Wrap(err)
	}
	firstLine := true
	reader := bytes.NewReader(noteData)
	scanner := bufio.NewScanner(reader)
	numPendingEmptyLines := 0
	for scanner.Scan() {
		text := strings.TrimRight(scanner.Text(), whitespaceChars)
		if firstLine {
			if len(text) == 0 {
				continue
			}
			firstLine = false
			text = fmt.Sprintf("%s\n", text)
		} else {
			if len(text) > 0 {
				text = fmt.Sprintf("    %s\n", text)
				for numPendingEmptyLines > 0 {
					if _, err := fmt.Fprintln(w, ""); err != nil {
						return skerr.Wrap(err)
					}
					numPendingEmptyLines--
				}
			} else {
				numPendingEmptyLines++
			}
		}
		if _, err := fmt.Fprint(w, text); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

func (a *AggregatorImpl) writeAllNotes(ctx context.Context, fs vfs.FS, w io.Writer, relnotesDir string) error {
	allNoteData, err := a.readAllNotes(ctx, fs, relnotesDir)
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, noteData := range allNoteData {
		if err = writeNote(noteData, w); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// writeNewMilestoneSection will read all release notes contained in the
// |relnotesDir| directory, aggregate them under a new milestone heading, and
// write that new heading using the |w| writer.
func (a *AggregatorImpl) writeNewMilestoneSection(ctx context.Context, fs vfs.FS, w io.Writer, milestone int, relnotesDir string) error {
	if _, err := fmt.Fprintf(w, "Milestone %d\n", milestone); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := fmt.Fprintln(w, "-------------"); err != nil {
		return skerr.Wrap(err)
	}
	if err := a.writeAllNotes(ctx, fs, w, relnotesDir); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Aggregate does the following:
//
//  1. Read all release notes contained within the |relnotesDir| directory.
//  2. Create a new m+1 milestone heading with the new release notes
//     included in an unordered list.
//  3. Return a new byte array which are the existing release notes, read from
//     |aggregateFilePath|, but with the new milestone section inserted at the
//     top of the stream.
//
// This function does not modify any files on disk. The caller is responsible
// for writing these modified release notes to disk if desired.
func (a *AggregatorImpl) Aggregate(ctx context.Context, fs vfs.FS, currentMilestone int, aggregateFilePath, relnotesDir string) ([]byte, error) {
	b, err := vfs.ReadFile(ctx, fs, aggregateFilePath)
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to open current release notes")
	}
	r := bytes.NewReader(b)
	var newContents bytes.Buffer
	scanner := bufio.NewScanner(r)
	gotFirstMilestoneHeading := false
	newMilestone := currentMilestone + 1
	insertNotesAfterNextHeadingUnderlines := false
	for scanner.Scan() {
		t := scanner.Text()
		if !gotFirstMilestoneHeading {
			if m := getMilestone(t); m != -1 {
				gotFirstMilestoneHeading = true
				if m == newMilestone {
					// The release notes file already has a section for the new milestone.
					// This should only be the case for the first milestone release after
					// switching to the new release notes process with individual files.
					// Insert all new notes just after the milestone heading, but don't
					// create a new one.
					insertNotesAfterNextHeadingUnderlines = true
				} else if m == currentMilestone {
					if err = a.writeNewMilestoneSection(ctx, fs, &newContents,
						newMilestone, relnotesDir); err != nil {
						return nil, skerr.Wrap(err)
					}
					fmt.Fprintf(&newContents, "\n* * *\n\n")
				} else {
					return nil, skerr.Fmt("Cannot jump from milestone %d to %d", m, newMilestone)
				}
			}
		}
		if _, err = fmt.Fprintln(&newContents, t); err != nil {
			return nil, skerr.Wrap(err)
		}
		if insertNotesAfterNextHeadingUnderlines && t == "-------------" {
			insertNotesAfterNextHeadingUnderlines = false
			if err = a.writeAllNotes(ctx, fs, &newContents, relnotesDir); err != nil {
				return nil, skerr.Wrap(err)
			}
		}
	}
	if !gotFirstMilestoneHeading {
		return nil, skerr.Fmt("%s does not contain an existing milestone section",
			aggregateFilePath)
	}
	if err := scanner.Err(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return newContents.Bytes(), nil
}

// Make sure AggregatorImpl fulfills the Aggregator interface.
var _ Aggregator = (*AggregatorImpl)(nil)
