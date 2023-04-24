// Copyright 2023 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package relnotes

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vfs"
	vfs_mocks "go.skia.org/infra/go/vfs/mocks"
)

func TestIsNotesFile_InvalidNoteFileNames_ReturnsFalse(t *testing.T) {
	test := func(name, basename string) {
		t.Run(name, func(t *testing.T) {
			assert.False(t, isNotesFile(basename))
		})
	}
	test("README", "README.md")
	test("OnlySuffix", ".md")
	test("dot", ".")
	test("EmptyString", "")
	test("READMEinDir", "path/to/README.md")
}

func TestIsNotesFile_ValidNoteFileNames_ReturnsTrue(t *testing.T) {
	test := func(name, basename string) {
		t.Run(name, func(t *testing.T) {
			assert.True(t, isNotesFile(basename))
		})
	}
	test("BugNumber", "bug_12345.md")
	test("WithSpaces", "file base name.md")
	test("NoteInDir", "path/to/bug_12345.md")
}

func TestListNoteFiles_WithMixturesOfValidAndInvalidNames_FiltersInvalid(t *testing.T) {
	relnotesDirContents := []os.FileInfo{
		vfs.FileInfo{
			Name:    "README.md",
			Size:    128,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "valid_note_file.md",
			Size:    128,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "dir_with_markdown_extension.md",
			Size:    0,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   true,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "text_file.txt",
			Size:    0,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   true,
			Sys:     nil,
		}.Get(),
	}

	dir := vfs_mocks.NewFile(t)
	fs := vfs_mocks.NewFS(t)
	fs.On("Open", testutils.AnyContext, "relnotes").Return(dir, nil)
	dir.On("ReadDir", testutils.AnyContext, -1).Return(relnotesDirContents, nil)
	dir.On("Close", testutils.AnyContext).Return(nil)
	aggregator := NewAggregator()
	names, err := aggregator.ListNoteFiles(context.Background(), fs, "relnotes")
	assert.NoError(t, err)
	require.Equal(t, []string{"valid_note_file.md"}, names)
}

func TestListNoteFiles_WithUnsortedDirListing_FilesAreSorted(t *testing.T) {
	relnotesDirContents := []os.FileInfo{
		vfs.FileInfo{
			Name:    "third.md",
			Size:    128,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "second.md",
			Size:    128,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "zebra.md",
			Size:    0,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "Third.md",
			Size:    0,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "1_note.md",
			Size:    0,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
	}

	dir := vfs_mocks.NewFile(t)
	fs := vfs_mocks.NewFS(t)
	fs.On("Open", testutils.AnyContext, "relnotes").Return(dir, nil)
	dir.On("ReadDir", testutils.AnyContext, -1).Return(relnotesDirContents, nil)
	dir.On("Close", testutils.AnyContext).Return(nil)

	aggregator := NewAggregator()
	fnames, err := aggregator.ListNoteFiles(context.Background(), fs, "relnotes")
	require.NoError(t, err)
	require.Equal(t, []string{"1_note.md", "Third.md", "second.md", "third.md", "zebra.md"}, fnames)
}

func TestGetMilestone_WithMilestones_ReturnsCorrectValue(t *testing.T) {
	test := func(name string, expected int, basename string) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, expected, getMilestone(basename))
		})
	}
	test("SingleDigit", 7, "Milestone 7")
	test("TwoDigits", 42, "Milestone 42")
	test("ThreeDigits", 512, "Milestone 512")
	test("FourDigits", 1267, "Milestone 1267")
}

func TestGetMilestone_InvalidMilestones_NoMatch(t *testing.T) {
	test := func(name, basename string) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, -1, getMilestone(basename))
		})
	}
	test("WithDots", "Milestone 112.5")
	test("HeaderWithPeriod", "Milestone 112.")
	test("NoNumber", "Milestone ")
	test("NoMilestoneWord", "Just some text ")
	test("NotAtStart", "Fixed in Milestone 42.")
	test("TrailingSpace", "Milestone 42 ")
}

func TestWriteNote_WithNoteData_WritesListItem(t *testing.T) {
	test := func(name, expected, note string) {
		t.Run(name, func(t *testing.T) {
			var outputText bytes.Buffer
			err := writeNote([]byte(note), &outputText)
			assert.NoError(t, err)
			require.Equal(t, expected, outputText.String())
		})
	}
	test("SingleLine", "  * Just one line\n", "Just one line")
	test("SingleLineWithNewline", "  * Just one line\n", "Just one line\n")
	test("TwoLines", "  * First line.\n    Second line.\n", "First line.\nSecond line.")
}

func TestWriteNote_WithLeadingTrailingEmptyLines_EmptyLinesIgnored(t *testing.T) {
	test := func(name, expected, note string) {
		t.Run(name, func(t *testing.T) {
			var outputText bytes.Buffer
			err := writeNote([]byte(note), &outputText)
			assert.NoError(t, err)
			require.Equal(t, expected, outputText.String())
		})
	}

	test("LeadingEmptyLines", "  * text\n", "   \n\ntext")
	test("InteriorEmptyLinesNotChanged", "  * line1\n\n\n    line5\n", "line1\n\n\nline5")
	test("TrailingEmptyLines", "  * text\n", "text\n\n   \n")
}

func TestWriteNote_InvalidNoteData_ReturnsError(t *testing.T) {
	test := func(name, note string) {
		t.Run(name, func(t *testing.T) {
			noteData := []byte(note)
			var outputText bytes.Buffer
			err := writeNote(noteData, &outputText)
			assert.Error(t, err)
		})
	}
	test("EmptyNoteFile", "")
}

func TestWriteAllNotes_WithValidNotes_OutputMilestoneSection(t *testing.T) {
	firstNote := []byte("First note")
	secondNote := []byte("Second note")
	relnotesDirContents := []os.FileInfo{
		vfs.FileInfo{
			Name:    "README.md",
			Size:    128,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "first.md",
			Size:    int64(len(firstNote)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "second.md",
			Size:    int64(len(secondNote)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
	}
	fs := vfs_mocks.NewFS(t)
	dir := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes").Return(dir, nil)
	dir.On("ReadDir", testutils.AnyContext, -1).Return(relnotesDirContents, nil)
	dir.On("Close", testutils.AnyContext).Return(nil)

	f1 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes/first.md").Once().Return(f1, nil)
	f1.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, firstNote)
	}).Return(len(firstNote), io.EOF)
	f1.On("Close", testutils.AnyContext).Return(nil)

	f2 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes/second.md").Once().Return(f2, nil)
	f2.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, secondNote)
	}).Return(len(secondNote), io.EOF)
	f2.On("Close", testutils.AnyContext).Return(nil)

	aggregator := NewAggregator()
	var outputText bytes.Buffer
	err := aggregator.writeNewMilestoneSection(context.Background(), fs,
		&outputText, 42, "relnotes")
	assert.NoError(t, err)

	const expected = `Milestone 42
-------------
  * First note
  * Second note
`
	assert.Equal(t, expected, outputText.String())
}

func TestWriteAllNotes_WithNoNotes_WriteEmptyMilestoneSection(t *testing.T) {
	dir := vfs_mocks.NewFile(t)
	fs := vfs_mocks.NewFS(t)
	fs.On("Open", testutils.AnyContext, "relnotes").Return(dir, nil)
	dir.On("ReadDir", testutils.AnyContext, -1).Return([]os.FileInfo{}, nil)
	dir.On("Close", testutils.AnyContext).Return(nil)

	aggregator := NewAggregator()
	var outputText bytes.Buffer
	err := aggregator.writeNewMilestoneSection(context.Background(), fs, &outputText, 42, "relnotes")
	assert.NoError(t, err)

	const expected = `Milestone 42
-------------
`
	assert.Equal(t, expected, outputText.String())
}

func TestAggregate_WithExistingTopLevelNotesAndValidNotes_WritesNewMilestone(t *testing.T) {
	firstNote := []byte("First note")
	secondNote := []byte("Second note")
	relnotesDirContents := []os.FileInfo{
		vfs.FileInfo{
			Name:    "README.md",
			Size:    128,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "first.md",
			Size:    int64(len(firstNote)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "second.md",
			Size:    int64(len(secondNote)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
	}
	fs := vfs_mocks.NewFS(t)
	dir := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes").Return(dir, nil)
	dir.On("ReadDir", testutils.AnyContext, -1).Return(relnotesDirContents, nil)
	dir.On("Close", testutils.AnyContext).Return(nil)

	f1 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes/first.md").Once().Return(f1, nil)
	f1.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, firstNote)
	}).Return(len(firstNote), io.EOF)
	f1.On("Close", testutils.AnyContext).Return(nil)

	f2 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes/second.md").Once().Return(f2, nil)
	f2.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, secondNote)
	}).Return(len(secondNote), io.EOF)
	f2.On("Close", testutils.AnyContext).Return(nil)

	const currentReleaseNotes = `Skia Graphics Release Notes

This file includes a list of high level updates for each milestone release.

Milestone 113
-------------
  * First item
  * Second item
  * Third item
`
	f3 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "RELEASE_NOTES.md").Once().Return(f3, nil)
	f3.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, currentReleaseNotes)
	}).Return(len(currentReleaseNotes), io.EOF)
	f3.On("Close", testutils.AnyContext).Return(nil)

	aggregator := NewAggregator()
	newNotes, err := aggregator.Aggregate(context.Background(), fs, 113, "RELEASE_NOTES.md", "relnotes")
	assert.NoError(t, err)

	const expectedReleaseNotes = `Skia Graphics Release Notes

This file includes a list of high level updates for each milestone release.

Milestone 114
-------------
  * First note
  * Second note

* * *

Milestone 113
-------------
  * First item
  * Second item
  * Third item
`

	require.Equal(t, expectedReleaseNotes, string(newNotes))
}

func TestAggregate_NewMilestoneMatchingExistingHeading_ModifiesExistingHeading(t *testing.T) {
	firstNote := []byte("Note from file 1")
	secondNote := []byte("Note from file 2")
	relnotesDirContents := []os.FileInfo{
		vfs.FileInfo{
			Name:    "README.md",
			Size:    128,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "first.md",
			Size:    int64(len(firstNote)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "second.md",
			Size:    int64(len(secondNote)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
	}
	fs := vfs_mocks.NewFS(t)
	dir := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes").Return(dir, nil)
	dir.On("ReadDir", testutils.AnyContext, -1).Return(relnotesDirContents, nil)
	dir.On("Close", testutils.AnyContext).Return(nil)

	f1 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes/first.md").Once().Return(f1, nil)
	f1.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, firstNote)
	}).Return(len(firstNote), io.EOF)
	f1.On("Close", testutils.AnyContext).Return(nil)

	f2 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "relnotes/second.md").Once().Return(f2, nil)
	f2.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, secondNote)
	}).Return(len(secondNote), io.EOF)
	f2.On("Close", testutils.AnyContext).Return(nil)

	const currentReleaseNotes = `Skia Graphics Release Notes

This file includes a list of high level updates for each milestone release.

Milestone 113
-------------
  * First item
  * Second item
  * Third item

* * *

Milestone 112
-------------
  * One
  * Two
  * Three
`
	f3 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "RELEASE_NOTES.md").Once().Return(f3, nil)
	f3.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, currentReleaseNotes)
	}).Return(len(currentReleaseNotes), io.EOF)
	f3.On("Close", testutils.AnyContext).Return(nil)

	aggregator := NewAggregator()
	newNotes, err := aggregator.Aggregate(context.Background(), fs, 112, "RELEASE_NOTES.md", "relnotes")
	assert.NoError(t, err)

	const expectedReleaseNotes = `Skia Graphics Release Notes

This file includes a list of high level updates for each milestone release.

Milestone 113
-------------
  * Note from file 1
  * Note from file 2
  * First item
  * Second item
  * Third item

* * *

Milestone 112
-------------
  * One
  * Two
  * Three
`
	require.Equal(t, expectedReleaseNotes, string(newNotes))
}

func TestAggregate_NewMilestoneGreaterByTwo_ReturnsError(t *testing.T) {
	const currentReleaseNotes = `Skia Graphics Release Notes

This file includes a list of high level updates for each milestone release.

Milestone 110
-------------
  * First item
  * Second item
  * Third item

* * *

Milestone 109
-------------
  * One
  * Two
  * Three
`
	fs := vfs_mocks.NewFS(t)
	f1 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "RELEASE_NOTES.md").Once().Return(f1, nil)
	f1.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, currentReleaseNotes)
	}).Return(len(currentReleaseNotes), io.EOF)
	f1.On("Close", testutils.AnyContext).Return(nil)

	aggregator := NewAggregator()
	// Aggregate can handle existing milestones 111 or 112, but no others and
	// should fail
	_, err := aggregator.Aggregate(context.Background(), fs, 112, "RELEASE_NOTES.md", "relnotes")
	assert.Error(t, err)
}

func TestAggregate_FailListNotes_ReturnsError(t *testing.T) {
	const relNotes = `Skia Graphics Release Notes

This file includes a list of high level updates for each milestone release.

Milestone 114
-------------
  * First note
  * Second note
`
	dir := vfs_mocks.NewFile(t)
	fs := vfs_mocks.NewFS(t)
	fs.On("Open", testutils.AnyContext, "relnotes").Return(dir, nil)
	dir.On("ReadDir", testutils.AnyContext, -1).Return(nil, skerr.Fmt("ReadDir failure"))
	dir.On("Close", testutils.AnyContext).Return(nil)

	f1 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "RELEASE_NOTES.md").Once().Return(f1, nil)
	f1.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, relNotes)
	}).Return(len(relNotes), io.EOF)
	f1.On("Close", testutils.AnyContext).Return(nil)

	aggregator := NewAggregator()
	notes, err := aggregator.Aggregate(context.Background(), fs, 114, "RELEASE_NOTES.md", "relnotes")
	require.Error(t, err)
	require.Nil(t, notes)
}

func TestAggregate_NoMilestone_ReturnsError(t *testing.T) {
	const relNotes = "File with no existing milestone section."
	fs := vfs_mocks.NewFS(t)
	f1 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "RELEASE_NOTES.md").Once().Return(f1, nil)
	f1.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, relNotes)
	}).Return(len(relNotes), io.EOF)
	f1.On("Close", testutils.AnyContext).Return(nil)

	aggregator := NewAggregator()
	notes, err := aggregator.Aggregate(context.Background(), fs, 1, "RELEASE_NOTES.md", "relnotes")
	require.Error(t, err)
	require.Nil(t, notes)
}

func TestAggregate_ReadFileFails_ReturnsError(t *testing.T) {
	fs := vfs_mocks.NewFS(t)
	f1 := vfs_mocks.NewFile(t)
	fs.On("Open", testutils.AnyContext, "RELEASE_NOTES.md").Once().Return(f1, nil)
	f1.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Return(0, skerr.Fmt("Read failure"))
	f1.On("Close", testutils.AnyContext).Return(nil)

	aggregator := NewAggregator()
	notes, err := aggregator.Aggregate(context.Background(), fs, 1, "RELEASE_NOTES.md", "relnotes")
	require.Error(t, err)
	require.Nil(t, notes)
}
