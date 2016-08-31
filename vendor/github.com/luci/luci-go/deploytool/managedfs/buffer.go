// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package managedfs

import (
	"sync"
)

const (
	copyBufferSize = 4096
)

var bufferPool sync.Pool

func withBuffer(f func([]byte)) {
	buf := bufferPool.Get().([]byte)
	if buf == nil {
		buf = make([]byte, copyBufferSize)
	}
	defer bufferPool.Put(buf)

	f(buf)
}
