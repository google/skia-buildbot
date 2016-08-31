// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package chunkstream

import (
	"sync"
)

var chunkNodePool = sync.Pool{
	New: func() interface{} {
		return &chunkNode{}
	},
}

func newChunkNode(c Chunk) *chunkNode {
	n := chunkNodePool.Get().(*chunkNode)
	n.Chunk = c
	return n
}

// chunkNode wraps a Chunk interface in a linked-list node.
type chunkNode struct {
	Chunk

	next *chunkNode
}

func (n *chunkNode) release() {
	n.Chunk.Release()
	n.Chunk = nil
	n.next = nil
	chunkNodePool.Put(n)
}

func (n *chunkNode) length() int {
	return len(n.Bytes())
}
