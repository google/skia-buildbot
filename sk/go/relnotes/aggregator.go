// Copyright 2023 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package relnotes

import (
	"context"

	"go.skia.org/infra/go/vfs"
)

// Aggregator defines an interface for processing all release notes
// for an upcoming release milestone into a single file.
type Aggregator interface {
	// ListNoteFiles will retrieve the base filenames of all files that are
	// considered to be release notes. This function matches all Markdown
	// files (with `*.md` extension), but ignores README.md (if present).
	ListNoteFiles(ctx context.Context, fs vfs.FS, notesDir string) ([]string, error)

	// Aggregate will process all release notes in the //relnotes directory.
	// It will read the current release notes file, identified by |relnotesPath|,
	// to determine the next milestone number. It then aggregates the individual
	// release notes, contained in the //relnotes folder, into a single
	// markdown list. This list will be inserted as a new milestone heading
	// section into a release notes Markdown stream returned as an array of bytes.
	// This function does not modify the existing release notes file.
	Aggregate(ctx context.Context, fs vfs.FS, currentMilestone int, aggregateFilePath, relnotesDir string) ([]byte, error)
}
