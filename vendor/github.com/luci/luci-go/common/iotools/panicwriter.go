// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iotools

import (
	"errors"
	"io"
)

// ErrPanicWriter is panic'd from the Writer provided to the callback in
// WriteTracker in the event of an io error.
var ErrPanicWriter = errors.New("ErrPanicWriter")

type panicWriter struct {
	CountingWriter
	err error
}

func (w *panicWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		panic(ErrPanicWriter)
	}
	n, err := w.CountingWriter.Write(p)
	if err != nil {
		w.err = err
		panic(ErrPanicWriter)
	}
	return n, nil
}

// WriteTracker helps to write complex writer routines correctly.
//
// This wraps a Writer with an implementation where any Write method will
// panic with the ErrPanicWriter error, catch that panic, and return the
// original io error as well as the number of written bytes.
//
// This means that the callback can use its Writer without tracking the number
// of bytes written, nor any io errors (i.e. it can ignore the return values
// from write operations entirely).
//
// If no io errors are encountered, this will return the callback's error and
// the number of written bytes.
func WriteTracker(w io.Writer, cb func(io.Writer) error) (n int, err error) {
	pw := &panicWriter{CountingWriter{Writer: w}, nil}
	defer func() {
		n = int(pw.Count)
		if r := recover(); r == ErrPanicWriter {
			err = pw.err
		} else if r != nil {
			panic(r)
		}
	}()
	err = cb(pw)
	n = int(pw.Count)
	return
}
