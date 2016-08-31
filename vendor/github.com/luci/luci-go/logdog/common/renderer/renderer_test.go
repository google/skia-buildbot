// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package renderer

import (
	"bytes"
	"io"
	"testing"

	"github.com/luci/luci-go/logdog/api/logpb"

	. "github.com/smartystreets/goconvey/convey"
)

// testSource implements the Source interface using stubbed data.
type testSource struct {
	logs []*logpb.LogEntry
	err  error
}

func (ts *testSource) NextLogEntry() (*logpb.LogEntry, error) {
	if ts.err != nil {
		return nil, ts.err
	}
	if len(ts.logs) == 0 {
		return nil, io.EOF
	}

	var le *logpb.LogEntry
	le, ts.logs = ts.logs[0], ts.logs[1:]
	return le, nil
}

func (ts *testSource) loadLogEntry(le *logpb.LogEntry) {
	ts.logs = append(ts.logs, le)
}

func (ts *testSource) loadText(line, delim string) {
	ts.loadLogEntry(&logpb.LogEntry{
		Content: &logpb.LogEntry_Text{
			Text: &logpb.Text{
				Lines: []*logpb.Text_Line{
					{
						Value:     line,
						Delimiter: delim,
					},
				},
			},
		},
	})
}

func (ts *testSource) loadBinary(data []byte) {
	ts.loadLogEntry(&logpb.LogEntry{
		Content: &logpb.LogEntry_Binary{
			Binary: &logpb.Binary{
				Data: data,
			},
		},
	})
}

func (ts *testSource) loadDatagram(data []byte, term bool) {
	ts.loadLogEntry(&logpb.LogEntry{
		Content: &logpb.LogEntry_Datagram{
			Datagram: &logpb.Datagram{
				Partial: &logpb.Datagram_Partial{
					Last: term,
				},
				Data: data,
			},
		},
	})
}

func TestRenderer(t *testing.T) {
	t.Parallel()

	Convey(`A Renderer connected to a test Source`, t, func() {
		ts := testSource{}
		b := bytes.Buffer{}
		r := &Renderer{
			Source: &ts,
		}

		Convey(`With no log data, will render nothing and return.`, func() {
			c, err := b.ReadFrom(r)
			So(err, ShouldBeNil)
			So(c, ShouldEqual, 0)
		})

		Convey(`With TEXT log entries ["1", "2"] using "DELIM" as the delimiter`, func() {
			ts.loadText("1", "DELIM")
			ts.loadText("2", "DELIM")

			Convey(`When not configured to reproduce, renders "1\n2\n".`, func() {
				_, err := b.ReadFrom(r)
				So(err, ShouldBeNil)
				So(b.String(), ShouldEqual, "1\n2\n")
			})

			Convey(`When configured to reproduce, renders "1DELIM2DELIM".`, func() {
				r.Reproduce = true

				_, err := b.ReadFrom(r)
				So(err, ShouldBeNil)
				So(b.String(), ShouldEqual, "1DELIM2DELIM")
			})
		})

		Convey(`With BINARY log entries {{0x00}, {0x01, 0x02}, {}, {0x03}}`, func() {
			ts.loadBinary([]byte{0x00})
			ts.loadBinary([]byte{0x01, 0x02})
			ts.loadBinary([]byte{})
			ts.loadBinary([]byte{0x03})

			Convey(`Renders {0x00, 0x01, 0x02, 0x03}.`, func() {
				_, err := b.ReadFrom(r)
				So(err, ShouldBeNil)
				So(b.Bytes(), ShouldResemble, []byte{0x00, 0x01, 0x02, 0x03})
			})

			Convey(`Can read the stream byte-by-byte.`, func() {
				b := [1]byte{}

				c, err := r.Read(b[:])
				So(err, ShouldBeNil)
				So(c, ShouldEqual, 1)
				So(b[0], ShouldEqual, 0x00)

				c, err = r.Read(b[:])
				So(err, ShouldBeNil)
				So(c, ShouldEqual, 1)
				So(b[0], ShouldEqual, 0x01)

				c, err = r.Read(b[:])
				So(err, ShouldBeNil)
				So(c, ShouldEqual, 1)
				So(b[0], ShouldEqual, 0x02)

				c, err = r.Read(b[:])
				So(err, ShouldBeNil)
				So(c, ShouldEqual, 1)
				So(b[0], ShouldEqual, 0x03)

				c, err = r.Read(b[:])
				So(err, ShouldEqual, io.EOF)
				So(c, ShouldEqual, 0)
			})
		})

		Convey(`With partial DATAGRAM log entries {{0x00}, {0x01, 0x02}, {}, {0x03}}.`, func() {
			ts.loadDatagram([]byte{0x00}, false)
			ts.loadDatagram([]byte{0x01, 0x02}, false)
			ts.loadDatagram([]byte{}, false)
			ts.loadDatagram([]byte{0x03}, true)

			hexDump := "Datagram #0 (4 bytes)\n" +
				"00000000  00 01 02 03                                       |....|\n\n"

			Convey(`Renders a full hex dump.`, func() {
				_, err := b.ReadFrom(r)
				So(err, ShouldBeNil)
				So(b.String(), ShouldEqual, hexDump)
			})

			Convey(`When deferring to a datagram writer`, func() {

				Convey(`Uses the writer instead of a hex dump.`, func() {
					var bytes []byte
					r.DatagramWriter = func(w io.Writer, dg []byte) bool {
						bytes = make([]byte, len(dg))
						copy(bytes, dg)

						w.Write([]byte("rendered"))
						return true
					}

					_, err := b.ReadFrom(r)
					So(err, ShouldBeNil)
					So(b.String(), ShouldEqual, "Datagram #0 (4 bytes)\nrendered\n")
					So(bytes, ShouldResemble, []byte{0x00, 0x01, 0x02, 0x03})
				})

				Convey(`Renders a full hex dump when the writer returns false.`, func() {
					r.DatagramWriter = func(w io.Writer, dg []byte) bool { return false }

					_, err := b.ReadFrom(r)
					So(err, ShouldBeNil)
					So(b.String(), ShouldEqual, hexDump)
				})
			})
		})

		Convey(`With empty log entries, renders nothing.`, func() {
			ts.loadLogEntry(&logpb.LogEntry{})
			c, err := b.ReadFrom(r)
			So(err, ShouldBeNil)
			So(c, ShouldEqual, 0)
		})
	})
}
