// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"bytes"
	"time"
	"unicode/utf8"

	"github.com/luci/luci-go/logdog/api/logpb"
)

const (
	posixNewline   = "\n"
	windowsNewline = "\r\n"
)

var (
	posixNewlineBytes = []byte(posixNewline)
)

// textParser is a parser implementation for the LogDog TEXT stream type.
type textParser struct {
	baseParser

	sequence int64
	buf      bytes.Buffer
}

var _ parser = (*textParser)(nil)

func (p *textParser) nextEntry(c *constraints) (*logpb.LogEntry, error) {
	limit := int64(c.limit)
	ts := time.Time{}
	txt := logpb.Text{}
	lineCount := 0
	for limit > 0 {
		br := p.ViewLimit(limit)
		if br.Remaining() == 0 {
			// Exceeded either limit or available buffer data.
			break
		}

		// Use the timestamp of the first data chunk.
		if len(txt.Lines) == 0 {
			ts, _ = p.firstChunkTime()
		} else if ct, _ := p.firstChunkTime(); !ct.Equal(ts) {
			// New timestamp, so need new LogEntry.
			break
		}

		// Find the index of our delimiter.
		//
		// We do this using a cross-platform approach that works on POSIX systems,
		// Mac (>=OSX), and Windows: we scan for "\n", then look backwards to see if
		// it was preceded by "\r" (for Windows-style newlines, "\r\n").
		idx := br.Index(posixNewlineBytes)

		newline := ""
		if idx >= 0 {
			br = br.CloneLimit(idx)
			newline = posixNewline
		} else if !c.allowSplit {
			// No delimiter within our limit, and we're not allowed to split, so we're
			// done.
			break
		}

		// Load the exportable data into our buffer.
		p.buf.Reset()
		p.buf.ReadFrom(br)

		// Does our exportable buffer end with "\r"? If so, treat it as a possible
		// Windows newline sequence.
		if p.buf.Len() > 0 && p.buf.Bytes()[p.buf.Len()-1] == byte('\r') {
			split := false
			if newline != "" {
				// "\n" => "\r\n"
				newline = windowsNewline
				split = true
			} else {
				// If we're closed and this is the last byte in the stream, it is a
				// dangling "\r" and we should include it. Otherwise, leave it for the
				// next round.
				split = !(c.closed && int64(p.buf.Len()) == p.Len())
			}

			if split {
				p.buf.Truncate(p.buf.Len() - 1)
			}
		}

		partial := (idx < 0)
		if !partial {
			lineCount++
		}

		// If we didn't have a delimiter, make sure we don't terminate in the middle
		// of a UTF8 character.
		if partial {
			count := 0
			lidx := -1
			b := p.buf.Bytes()
			for len(b) > 0 {
				r, sz := utf8.DecodeRune(b)
				count += sz
				if r != utf8.RuneError {
					lidx = count
				}
				b = b[sz:]
			}
			if lidx < 0 {
				break
			}
			p.buf.Truncate(lidx)
		}

		txt.Lines = append(txt.Lines, &logpb.Text_Line{
			Value:     p.buf.String(),
			Delimiter: newline,
		})
		p.Consume(int64(p.buf.Len() + len(newline)))
		limit -= int64(p.buf.Len() + len(newline))
	}

	if len(txt.Lines) == 0 {
		return nil, nil
	}
	le := p.baseLogEntry(ts)
	le.Sequence = uint64(p.sequence)
	le.Content = &logpb.LogEntry_Text{Text: &txt}

	p.sequence += int64(lineCount)
	return le, nil
}
