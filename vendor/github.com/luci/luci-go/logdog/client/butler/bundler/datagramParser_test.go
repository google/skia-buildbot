// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"bytes"
	"testing"
	"time"

	"github.com/luci/luci-go/common/data/chunkstream"
	"github.com/luci/luci-go/common/data/recordio"
	"github.com/luci/luci-go/logdog/api/logpb"
	. "github.com/smartystreets/goconvey/convey"
)

func dgram(d ...byte) []byte {
	buf := bytes.Buffer{}
	recordio.WriteFrame(&buf, d)
	return buf.Bytes()
}

func spread(ts time.Time, inc time.Duration, size int, d []byte) ([]chunkstream.Chunk, time.Duration) {
	if len(d) == 0 {
		return nil, 0
	}
	if size == 0 {
		size = len(d)
	}

	result := make([]chunkstream.Chunk, 0, (len(d)/size)+1)
	offset := time.Duration(0)
	for len(d) > 0 {
		if size > len(d) {
			size = len(d)
		}
		result = append(result, data(ts.Add(offset), d[:size]...))
		d = d[size:]
		offset += inc
	}
	return result, offset
}

func TestDatagramParser(t *testing.T) {
	Convey(`A datagramParser`, t, func() {
		s := &parserTestStream{
			now:         time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC),
			prefixIndex: 1337,
		}
		p := &datagramParser{
			baseParser: s.base(),
			maxSize:    64,
		}
		c := &constraints{
			limit: 32,
		}

		Convey(`Loaded with {<2>, 0xca, 0xfe}, {<0>, }, {<5>, 0xd0, 0x65, 0x10, 0xbb, 0x12}`, func() {
			now := s.now
			for _, b := range [][]byte{
				{0xca, 0xfe},
				{},
				{0xd0, 0x65, 0x10, 0xbb, 0x12},
			} {
				chunks, offset := spread(now, time.Second, 2, dgram(b...))
				p.Append(chunks...)
				now = now.Add(offset)
			}

			Convey(`Yields the 3 datagrams as individual LogEntry.`, func() {
				le, err := p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, shouldMatchLogEntry, s.le(0, logpb.Datagram{
					Data: []byte{0xca, 0xfe},
				}))

				le, err = p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, shouldMatchLogEntry, s.add(2*time.Second).le(1, logpb.Datagram{}))

				le, err = p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, shouldMatchLogEntry, s.add(time.Second).le(2, logpb.Datagram{
					Data: []byte{0xd0, 0x65, 0x10, 0xbb, 0x12},
				}))
			})

			Convey(`With a limit of 2`, func() {
				c.limit = 2

				Convey(`When not truncating, only yields the first two datagrams.`, func() {
					le, err := p.nextEntry(c)
					So(err, ShouldBeNil)
					So(le, shouldMatchLogEntry, s.le(0, logpb.Datagram{
						Data: []byte{0xca, 0xfe},
					}))

					le, err = p.nextEntry(c)
					So(err, ShouldBeNil)
					So(le, shouldMatchLogEntry, s.add(2*time.Second).le(1, logpb.Datagram{}))

					le, err = p.nextEntry(c)
					So(err, ShouldBeNil)
					So(le, ShouldBeNil)
				})

				Convey(`When truncating, yields all three, the last as partial.`, func() {
					c.limit = 2
					c.allowSplit = true

					le, err := p.nextEntry(c)
					So(err, ShouldBeNil)
					So(le, shouldMatchLogEntry, s.le(0, logpb.Datagram{
						Data: []byte{0xca, 0xfe},
					}))

					le, err = p.nextEntry(c)
					So(err, ShouldBeNil)
					So(le, shouldMatchLogEntry, s.add(2*time.Second).le(1, logpb.Datagram{}))

					le, err = p.nextEntry(c)
					So(err, ShouldBeNil)
					So(le, shouldMatchLogEntry, s.add(time.Second).le(2, logpb.Datagram{
						Data: []byte{0xd0, 0x65},
						Partial: &logpb.Datagram_Partial{
							Size:  5,
							Index: 0,
						},
					}))

					le, err = p.nextEntry(c)
					So(err, ShouldBeNil)
					So(le, shouldMatchLogEntry, s.add(time.Second).le(2, logpb.Datagram{
						Data: []byte{0x10, 0xbb},
						Partial: &logpb.Datagram_Partial{
							Size:  5,
							Index: 1,
						},
					}))

					le, err = p.nextEntry(c)
					So(err, ShouldBeNil)
					So(le, shouldMatchLogEntry, s.add(time.Second).le(2, logpb.Datagram{
						Data: []byte{0x12},
						Partial: &logpb.Datagram_Partial{
							Size:  5,
							Index: 2,
							Last:  true,
						},
					}))

					le, err = p.nextEntry(c)
					So(err, ShouldBeNil)
					So(le, ShouldBeNil)
				})
			})
		})

		Convey(`Datagram [<1>, 0xFF]`, func() {
			Convey(`Returns nothing when the data is only partially available.`, func() {
				p.Append(data(s.now, 0x01))
				c.limit = 1
				c.allowSplit = true

				le, err := p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, ShouldBeNil)
			})
		})

		Convey(`A datagram with a incomplete size header {0x80, 0x80}`, func() {
			Convey(`Returns no log entries.`, func() {
				p.Append(data(s.now, 0x80, 0x80))

				le, err := p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, ShouldBeNil)
			})
		})

		Convey(`A datagram that is larger than the maximum size`, func() {
			Convey(`Returns no log entries.`, func() {
				p.Append(data(s.now, 0x80, 0x80, 0x01))

				_, err := p.nextEntry(c)
				So(err, ShouldEqual, recordio.ErrFrameTooLarge)
			})
		})
	})
}
