// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iotools

import (
	"io"
)

// CountingWriter is an io.Writer that counts the number of bytes that are
// written.
type CountingWriter struct {
	io.Writer // The underlying io.Writer.

	// Count is the number of bytes that have been written.
	Count int64

	singleByteBuf [1]byte
}

var _ io.Writer = (*CountingWriter)(nil)

// Write implements io.Writer.
func (c *CountingWriter) Write(buf []byte) (int, error) {
	amount, err := c.Writer.Write(buf)
	c.Count += int64(amount)
	return amount, err
}

// WriteByte implements io.ByteWriter.
func (c *CountingWriter) WriteByte(b byte) error {
	// If our underlying Writer is a ByteWriter, use its WriteByte directly.
	if bw, ok := c.Writer.(io.ByteWriter); ok {
		if err := bw.WriteByte(b); err != nil {
			return err
		}
		c.Count++
		return nil
	}

	c.singleByteBuf[0] = b
	_, err := c.Write(c.singleByteBuf[:])
	return err
}
