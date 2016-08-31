// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iotools

import (
	"net"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

// TestDeadlineReader tests the DeadlineReader struct.
func TestDeadlineReader(t *testing.T) {
	Convey(`A DeadlineReader with a nil connection`, t, func() {
		dr := &DeadlineReader{}

		Convey(`Should panic when Read is called.`, func() {
			buf := make([]byte, 1)
			So(func() { dr.Read(buf) }, ShouldPanic)
		})

		Convey(`Should panic when Close is called.`, func() {
			So(func() { dr.Close() }, ShouldPanic)
		})
	})

	// Open a local listening connection.
	Convey(`With a local listening connection`, t, func() {
		proto := "tcp"
		ln, err := net.Listen(proto, "127.0.0.1:0")
		if err != nil {
			proto = "tcp6"
			ln, err = net.Listen(proto, "[::0]:0")
		}
		So(err, ShouldBeNil)
		defer ln.Close()

		// Accept connections and don't write any data.
		dataC := make(chan []byte)
		go func() {
			c, err := ln.Accept()
			if err != nil {
				panic("Error while accepting a client connection.")
			}
			defer c.Close()

			data := <-dataC
			if data != nil {
				c.Write(data)
			}
		}()

		// Dial into the local listener.
		c, err := net.Dial(proto, ln.Addr().String())
		So(err, ShouldBeNil)

		// Create a deadline reader.
		dr := &DeadlineReader{Conn: c}
		defer dr.Close()

		// Wrap it in a deadline reader.
		Convey(`Given a deadline reader with no deadline, should block on read.`, func() {
			dr.Deadline = 0

			// Have the server send data (goroutine).
			go func() {
				dataC <- []byte{0xAA}
			}()

			// Connect and read bytes.
			buf := make([]byte, 1)
			amount, err := dr.Read(buf)
			So(err, ShouldBeNil)
			So(amount, ShouldEqual, 1)
		})

		Convey(`Given a deadline reader with a deadline, should timeout.`, func() {
			dr.Deadline = 1 * time.Millisecond

			// Connect and read bytes.
			buf := make([]byte, 1)
			_, err := dr.Read(buf)
			So(err, ShouldEqual, ErrTimeout)
		})
	})
}
