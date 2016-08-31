// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package chunkstream

// Chunk wraps a fixed-size byte buffer. It is the primary interface used by
// the chunk library.
//
// A Chunk reference should be released once the user is finished with it. After
// being released, it may no longer be accessed.
type Chunk interface {
	// Bytes returns the underlying byte slice contained by this Chunk.
	Bytes() []byte

	// Release releases the Chunk. After being released, a Chunk's methods may no
	// longer be used.
	Release()
}
