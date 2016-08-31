// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package chunkstream

import (
	"bytes"
	"errors"
	"io"
)

// View is static read-only snapshot of the contents of the Buffer, presented
// as a contiguous stream of bytes.
//
// View implements the io.Reader and io.ByteReader interfaces. It also offers a
// series of utility functions optimized for the chunks.
type View struct {
	// cur is the first node of the view.
	cur *chunkNode
	// cidx is the byte offset within cur of the current byte.
	cidx int
	// size is the size of thew view. Accesses beyond this size will fail.
	size int64
	// consumed is a count of the number of bytes in the view that have been
	// consumed via Skip().
	consumed int64

	// b is the Buffer from which this View's snapshot was taken.
	b *Buffer
}

var _ interface {
	io.Reader
	io.ByteReader
} = (*View)(nil)

func (r *View) Read(b []byte) (int, error) {
	total := int64(0)
	err := error(nil)
	for len(b) > 0 {
		chunk := r.chunkBytes()
		if len(chunk) == 0 {
			err = io.EOF
			break
		}

		amount := copy(b, chunk)
		total += int64(amount)
		b = b[amount:]
		r.Skip(int64(amount))
	}
	if r.Remaining() == 0 {
		err = io.EOF
	}
	return int(total), err
}

// ReadByte implements io.ByteReader, reading a single byte from the buffer.
func (r *View) ReadByte() (byte, error) {
	chunk := r.chunkBytes()
	if len(chunk) == 0 {
		return 0, io.EOF
	}
	r.Skip(1)
	return chunk[0], nil
}

// Remaining returns the number of bytes remaining in the Reader view.
func (r *View) Remaining() int64 {
	return r.size
}

// Consumed returns the number of bytes that have been skipped via Skip or
// higher-level calls.
func (r *View) Consumed() int64 {
	return r.consumed
}

// Skip advances the View forwards a fixed number of bytes.
func (r *View) Skip(count int64) {
	for count > 0 {
		if r.cur == nil {
			panic(errors.New("cannot skip past end buffer"))
		}

		amount := r.chunkRemaining()
		if count < int64(amount) {
			amount = int(count)
			r.cidx += amount
		} else {
			// Finished consuming this chunk, move on to the next.
			r.cur = r.cur.next
			r.cidx = 0
		}

		count -= int64(amount)
		r.consumed += int64(amount)
		r.size -= int64(amount)
	}
}

// Index scans the View for the specified needle bytes. If they are
// found, their index in the View is returned. Otherwise, Index returns
// -1.
//
// The View is not modified during the search.
func (r *View) Index(needle []byte) int64 {
	if r.Remaining() == 0 {
		return -1
	}
	if len(needle) == 0 {
		return 0
	}

	rc := r.Clone()
	if !rc.indexDestructive(needle) {
		return -1
	}
	return rc.consumed - r.consumed
}

// indexDestructive implements Index by actively mutating the View.
//
// It returns true if the needle was found, and false if not. The view will be
// mutated regardless.
func (r *View) indexDestructive(needle []byte) bool {
	tbuf := make([]byte, 2*len(needle))
	idx := int64(0)
	for {
		data := r.chunkBytes()
		if len(data) == 0 {
			return false
		}

		// Scan the current chunk for needle. Note that if the current chunk is too
		// small to hold needle, this is a no-op.
		if idx = int64(bytes.Index(data, needle)); idx >= 0 {
			r.Skip(idx)
			return true
		}
		if len(data) > len(needle) {
			// The needle is definitely not in this space.
			r.Skip(int64(len(data) - len(needle)))
		}

		// needle isn't in the current chunk; however, it may begin at the end of
		// the current chunk and complete in future chunks.
		//
		// We will scan a space twice the size of the needle, as otherwise, this
		// would end up scanning for one possibility, incrementing by one, and
		// repeating via 'for' loop iterations.
		//
		// Afterwards, we advance only the size of the needle, as we don't want to
		// preclude the needle starting after our last scan range.
		//
		// For example, to find needle "NDL":
		//
		// AAAAND|L|AAAA
		//     |------|^- [NDLAAA], 0
		//
		// AAAAN|D|NDL|AAAA
		//    |------| [ANDNDL], 3
		//
		// AAAA|A|A|NDL
		//   |-------| [AAAAND], -1, consume 3 => A|NDL|
		//
		//
		// Note that we perform the read with a cloned View so we don't
		// actually consume this data.
		pr := r.Clone()
		amt, _ := pr.Read(tbuf)
		if amt < len(needle) {
			// All remaining buffers cannot hold the needle.
			return false
		}

		if idx = int64(bytes.Index(tbuf[:amt], needle)); idx >= 0 {
			r.Skip(idx)
			return true
		}
		r.Skip(int64(len(needle)))
	}
}

// Clone returns a copy of the View view.
//
// The clone is bound to the same underlying Buffer as the source.
func (r *View) Clone() *View {
	return r.CloneLimit(r.size)
}

// CloneLimit returns a copy of the View view, optionally truncating it.
//
// The clone is bound to the same underlying Buffer as the source.
func (r *View) CloneLimit(limit int64) *View {
	c := *r
	if c.size > limit {
		c.size = limit
	}
	return &c
}

func (r *View) chunkRemaining() int {
	if r.cur == nil {
		return 0
	}
	result := r.cur.length() - r.cidx
	if int64(result) > r.size {
		result = int(r.size)
	}
	return result
}

func (r *View) chunkBytes() []byte {
	if r.cur == nil {
		return nil
	}
	data := r.cur.Bytes()[r.cidx:]
	remaining := r.Remaining()
	if int64(len(data)) > remaining {
		data = data[:remaining]
	}
	return data
}
