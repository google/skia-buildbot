// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package chunkstream

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type testChunk struct {
	data     []byte
	released bool
}

var _ Chunk = (*testChunk)(nil)

func tc(d ...byte) *testChunk {
	return &testChunk{
		data: d,
	}
}

func (c *testChunk) String() string {
	pieces := make([]string, len(c.data))
	for i, d := range c.data {
		pieces[i] = fmt.Sprintf("0x%02x", d)
	}
	return fmt.Sprintf("{%s}", strings.Join(pieces, ", "))
}

func (c *testChunk) Bytes() []byte {
	return c.data
}

func (c *testChunk) Len() int {
	return len(c.data)
}

func (c *testChunk) Release() {
	if c.released {
		panic("double-free")
	}
	c.released = true
}

func TestChunkNode(t *testing.T) {
	Convey(`A chunkNode wrapping a testing Chunk implementation`, t, func() {
		c := tc(0, 1, 2)
		n := newChunkNode(c)

		Convey(`Should call Chunk methods.`, func() {
			So(n.Bytes(), ShouldResemble, []byte{0, 1, 2})
		})

		Convey(`When released, releases the wrapped Chunk.`, func() {
			n.release()
			So(c.released, ShouldBeTrue)

			Convey(`If released again, panics.`, func() {
				So(func() { n.release() }, ShouldPanic)
			})
		})
	})
}
