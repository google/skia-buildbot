// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logdog

// GetMessageProject implements ProjectBoundMessage.
func (ar *RegisterStreamRequest) GetMessageProject() string { return ar.Project }

// GetMessageProject implements ProjectBoundMessage.
func (ar *LoadStreamRequest) GetMessageProject() string { return ar.Project }

// GetMessageProject implements ProjectBoundMessage.
func (ar *TerminateStreamRequest) GetMessageProject() string { return ar.Project }

// GetMessageProject implements ProjectBoundMessage.
func (ar *ArchiveStreamRequest) GetMessageProject() string { return ar.Project }

// Complete returns true if the archive request expresses that the archived
// log stream was complete.
//
// A log stream is complete if every entry between zero and its terminal index
// is included.
func (ar *ArchiveStreamRequest) Complete() bool {
	tidx := ar.TerminalIndex
	if tidx < 0 {
		tidx = -1
	}
	return (ar.LogEntryCount == (tidx + 1))
}
