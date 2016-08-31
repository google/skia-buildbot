// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package chunkstream

import (
	"errors"
)

// Buffer is a collection of ordered Chunks that can cheaply read and shifted
// as if it were a continuous byte stream.
//
// A Buffer is not goroutine-safe.
//
// The primary means of interacting with a Buffer is to construct a View and
// then use it to access the Buffer's contents. Views can be used concurrently,
// and View operations are goroutine-safe.
type Buffer struct {
	// First is a pointer to the first Chunk node in the buffer.
	first *chunkNode
	// Last is a pointer to the last Chunk node in the buffer.
	last *chunkNode

	// size is the total number of bytes in the Buffer.
	size int64

	// fidx is the current byte offset in first.
	fidx int
}

// Append adds additional Chunks to the buffer.
//
// After completion, the Chunk is now owned by the Buffer and should not be used
// anymore externally.
func (b *Buffer) Append(c ...Chunk) {
	for _, chunk := range c {
		b.appendChunk(chunk)
	}
}

func (b *Buffer) appendChunk(c Chunk) {
	// Ignore/discard zero-length data.
	if len(c.Bytes()) == 0 {
		c.Release()
		return
	}

	cn := newChunkNode(c)
	cn.next = nil
	if b.last == nil {
		// First node.
		b.first = cn
	} else {
		b.last.next = cn
	}
	b.last = cn
	b.size += int64(cn.length())
}

// Bytes constructs a byte slice containing the contents of the Buffer.
//
// This is a potentially expensive operation, and should generally be used only
// for debugging and tests, as it defeats most of the purpose of this package.
func (b *Buffer) Bytes() []byte {
	if b.Len() == 0 {
		return nil
	}

	m := make([]byte, 0, b.Len())
	idx := b.fidx
	for cur := b.first; cur != nil; cur = cur.next {
		m = append(m, cur.Bytes()[idx:]...)
		idx = 0
	}
	return m
}

// Len returns the total amount of data in the buffer.
func (b *Buffer) Len() int64 {
	return b.size
}

// FirstChunk returns the first Chunk in the Buffer, or nil if the Buffer has
// no Chunks.
func (b *Buffer) FirstChunk() Chunk {
	if b.first == nil {
		return nil
	}
	return b.first.Chunk
}

// View returns a View instance bound to this Buffer and spanning all data
// currently in the Buffer.
//
// The View is no longer valid after Consume is called on the Buffer.
func (b *Buffer) View() *View {
	return b.ViewLimit(b.size)
}

// ViewLimit constructs a View instance, but artificially constrains it to
// read at most the specified number of bytes.
//
// This is useful when reading a subset of the data into a Buffer, as ReadFrom
// does not allow a size to be specified.
func (b *Buffer) ViewLimit(limit int64) *View {
	if limit > b.size {
		limit = b.size
	}

	return &View{
		cur:  b.first,
		cidx: b.fidx,
		size: limit,

		b: b,
	}
}

// Consume removes the specified number of bytes from the beginning of the
// Buffer. If Consume skips past all of the data in a Chunk is no longer needed,
// it is Release()d.
func (b *Buffer) Consume(c int64) {
	if c == 0 {
		return
	}

	if c > b.size {
		panic(errors.New("consuming more data than available"))
	}
	b.size -= c

	for c > 0 {
		// Do we consume the entire chunk?
		if int64(b.first.length()-b.fidx) > c {
			// No. Advance our chunk index and terminate.
			b.fidx += int(c)
			break
		}

		n := b.first
		c -= int64(n.length() - b.fidx)
		b.first = n.next
		b.fidx = 0
		if b.first == nil {
			b.last = nil
		}

		// Release our node. We must not reference it after this.
		n.release()
	}
}
