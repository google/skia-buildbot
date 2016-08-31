// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package recordio

// FrameHeaderSize calculates the size of the RecordIO frame header for a given
// amount of data.
//
// A RecordIO frame header is a Uvarint containing the length. Uvarint values
// contain 7 bits of size data and 1 continuation bit (see encoding/binary).
func FrameHeaderSize(s int64) int {
	size := 1
	for s >>= 7; s > 0; s >>= 7 {
		size++
	}
	return size
}
