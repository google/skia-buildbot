// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package renderer exports the capability to render a LogDog log stream to an
// io.Writer.
//
//   - Text streams are rendered by emitting the logs and their newlines in
//     order.
//   - Binary streams are rendered by emitting the sequential binary data
//     verbatim.
package renderer

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/luci/luci-go/logdog/api/logpb"
)

// Source returns successive LogEntry records for the Renderer to render.
type Source interface {
	// NextLogEntry returns the next successive LogEntry record to render, or an
	// error if it could not be retrieved.
	NextLogEntry() (*logpb.LogEntry, error)
}

// Renderer is a stateful instance that provides an io.Reader interface to a
// log stream.
type Renderer struct {
	// Source is the log Source to use to retrieve the logs to render.
	Source Source
	// Reproduce, if true, attempts to reproduce the original stream data.
	//
	// For text streams, this means using the original streams' encoding and
	// newline delimiters. If this is false, UTF8 and "\n" will be used.
	Reproduce bool
	// DatagramWriter is a function to call to render a complete datagram stream.
	// If it returns false, or if nil, a hex dump renderer will be used to
	// render the datagram.
	DatagramWriter func(io.Writer, []byte) bool

	// Currently-buffered data.
	buf bytes.Buffer
	// bufPos is the current position in the buffer.
	bufPos int
	// err is the error returned by bufferNext. Once this is set, no more logs
	// will be read and any buffered logs will be drained.
	err error

	// dgBuf is a buffer used for partial datagrams.
	dgBuf bytes.Buffer
}

var _ io.Reader = (*Renderer)(nil)

func (r *Renderer) Read(b []byte) (int, error) {
	buffered := r.buffered()
	for r.err == nil && len(buffered) == 0 {
		r.err = r.bufferNext()
		r.bufPos = 0

		buffered = r.buffered()
	}

	count := copy(b, buffered)
	r.bufPos += count
	return count, r.err
}

func (r *Renderer) buffered() []byte {
	return r.buf.Bytes()[r.bufPos:]
}

func (r *Renderer) bufferNext() error {
	r.buf.Reset()

	// Fetch and buffer the next log entry.
	le, err := r.Source.NextLogEntry()
	if le != nil {
		switch {
		case le.GetText() != nil:
			for _, line := range le.GetText().Lines {
				r.buf.WriteString(line.Value)
				if !r.Reproduce {
					r.buf.WriteRune('\n')
				} else {
					r.buf.WriteString(line.Delimiter)
				}
			}

		case le.GetBinary() != nil:
			r.buf.Write(le.GetBinary().Data)

		case le.GetDatagram() != nil:
			dg := le.GetDatagram()
			if _, err := r.dgBuf.Write(dg.Data); err != nil {
				return err
			}

			if p := dg.GetPartial(); p == nil || p.Last {
				// Datagram is complete. render it.
				bytesStr := "bytes"
				if r.dgBuf.Len() == 1 {
					bytesStr = "byte"
				}

				fmt.Fprintf(&r.buf, "Datagram #%d (%d %s)\n", le.Sequence, r.dgBuf.Len(), bytesStr)
				if f := r.DatagramWriter; f == nil || !f(&r.buf, r.dgBuf.Bytes()) {
					// Writer failed, or no writer configured. Use a hex dump.
					if err := dumpHex(&r.buf, r.dgBuf.Bytes()); err != nil {
						return err
					}
				}
				r.buf.WriteRune('\n')

				r.dgBuf.Reset()
			}
		}
	}
	return err
}

func dumpHex(w io.Writer, data []byte) (err error) {
	// Hex dump.
	d := hex.Dumper(w)
	defer func() {
		if cerr := d.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	_, err = d.Write(data)
	return
}
