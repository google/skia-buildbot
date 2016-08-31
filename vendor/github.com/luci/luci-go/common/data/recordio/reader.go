// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package recordio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// ErrFrameTooLarge is an error that is returned if a frame that is larger than
// the maximum allowed size (not including the frame header) is read.
var ErrFrameTooLarge = fmt.Errorf("frame: frame size exceeds maximum")

// Reader reads individual frames from a frame-formatted input Reader.
type Reader interface {
	// ReadFrame reads the next frame, returning the frame's size and an io.Reader
	// for that frame's data. The io.Reader is restricted such that it cannot read
	// past the frame.
	//
	// The frame must be fully read before another Reader call can be made.
	// Failure to do so will cause the Reader to become unsynchronized.
	ReadFrame() (int64, io.Reader, error)

	// ReadFrame returns the contents of the next frame. If there are no more
	// frames available, ReadFrame will return io.EOF.
	ReadFrameAll() ([]byte, error)
}

// reader is an implementation of a Reader that uses an underlying
// io.Reader and io.ByteReader to read frames.
//
// The io.Reader and io.ByteReader must read from the same source.
type reader struct {
	io.Reader
	io.ByteReader

	maxSize int64
}

// NewReader creates a new Reader which reads frame data from the
// supplied Reader instance.
//
// If the Reader instance is also an io.ByteReader, its ReadByte method will
// be used directly.
func NewReader(r io.Reader, maxSize int64) Reader {
	br, ok := r.(io.ByteReader)
	if !ok {
		br = &simpleByteReader{Reader: r}
	}
	return &reader{
		Reader:     r,
		ByteReader: br,
		maxSize:    maxSize,
	}
}

func (r *reader) ReadFrame() (int64, io.Reader, error) {
	// Read the frame size.
	count, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, nil, err
	}

	if count > uint64(r.maxSize) {
		return 0, nil, ErrFrameTooLarge
	}

	lr := &io.LimitedReader{
		R: r.Reader,
		N: int64(count),
	}
	return int64(count), lr, nil
}

func (r *reader) ReadFrameAll() ([]byte, error) {
	count, fr, err := r.ReadFrame()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}

	data := make([]byte, count)
	if _, err := fr.Read(data); err != nil {
		return nil, err
	}
	return data, nil
}

// simpleByteReader implements the io.ByteReader interface for an io.Reader.
type simpleByteReader struct {
	io.Reader

	buf [1]byte
}

func (r *simpleByteReader) ReadByte() (byte, error) {
	_, err := r.Read(r.buf[:])
	return r.buf[0], err
}

// Split splits the supplied buffer into its component records.
//
// This method implements zero-copy segmentation, so the individual records are
// slices of the original data set.
func Split(data []byte) (records [][]byte, err error) {
	br := bytes.NewReader(data)

	for br.Len() > 0 {
		var size uint64
		size, err = binary.ReadUvarint(br)
		if err != nil {
			return
		}
		if size > uint64(br.Len()) {
			err = ErrFrameTooLarge
			return
		}

		// Pull out the record from the original byte stream without copying.
		// Casting size to an integer is safe at this point, since we have asserted
		// that it is less than the remaining length in the buffer, which is an int.
		offset := len(data) - br.Len()
		records = append(records, data[offset:offset+int(size)])

		if _, err := br.Seek(int64(size), 1); err != nil {
			// Our measurements should protect us from this being an invalid seek.
			panic(err)
		}
	}
	return records, nil
}
