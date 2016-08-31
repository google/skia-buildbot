// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iotools

import (
	"errors"
	"io"
	"net"
	"time"
)

// ErrTimeout is an error returned if the DeadlineReader times out.
var ErrTimeout = errors.New("iotools: timeout")

// ReadTimeoutSetter is an interface for an object that can have its read
// timeout set.
type ReadTimeoutSetter interface {
	// SetReadTimeout sets the read deadline for subsqeuent reads on this
	// Reader.
	SetReadTimeout(time.Duration) error
}

// DeadlineReader is a wrapper around a net.Conn that applies an idle timeout
// deadline to the Conn's Read() method.
type DeadlineReader struct {
	net.Conn

	// Deadline is the read deadline to apply prior to each 'Read()'. It can also
	// be set via SetReadTimeout.
	Deadline time.Duration
}

var _ interface {
	io.ReadCloser
	ReadTimeoutSetter
} = (*DeadlineReader)(nil)

// Read implements io.Reader.
func (r *DeadlineReader) Read(b []byte) (int, error) {
	// If we have a deadline, apply it before the 'Read()'
	if r.Deadline > 0 {
		deadline := time.Now().Add(r.Deadline)
		if err := r.Conn.SetDeadline(deadline); err != nil {
			return 0, err
		}
	}

	amount, err := r.Conn.Read(b)
	if e, ok := err.(net.Error); ok && e.Timeout() {
		// Don't report back read timeout errors.
		err = ErrTimeout
	}
	return amount, err
}

// SetReadTimeout implements ReadDeadlineSetter.
func (r *DeadlineReader) SetReadTimeout(d time.Duration) error {
	r.Deadline = d
	return nil
}
