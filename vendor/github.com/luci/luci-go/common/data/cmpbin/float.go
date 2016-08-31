// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cmpbin

import (
	"encoding/binary"
	"io"
	"math"
)

// WriteFloat64 writes a memcmp-sortable float to buf. It returns the number
// of bytes written (8, unless an error occurs), and any write error
// encountered.
func WriteFloat64(buf io.Writer, v float64) (n int, err error) {
	bits := math.Float64bits(v)
	bits = bits ^ (-(bits >> 63) | (1 << 63))
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, bits)
	return buf.Write(data)
}

// ReadFloat64 reads a memcmp-sortable float from buf (as written by
// WriteFloat64). It also returns the number of bytes read (8, unless an error
// occurs), and any read error encountered.
func ReadFloat64(buf io.Reader) (ret float64, n int, err error) {
	// byte-ordered floats http://stereopsis.com/radix.html
	data := make([]byte, 8)
	if n, err = buf.Read(data); err != nil {
		return
	}
	bits := binary.BigEndian.Uint64(data)
	ret = math.Float64frombits(bits ^ (((bits >> 63) - 1) | (1 << 63)))
	return
}
