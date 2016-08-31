// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"testing"
	"time"

	"github.com/luci/luci-go/logdog/api/logpb"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBinaryParser(t *testing.T) {
	Convey(`A binaryParser with a threshold of 2.`, t, func() {
		s := &parserTestStream{
			now:         time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC),
			prefixIndex: 1337,
		}
		p := &binaryParser{
			baseParser: s.base(),
			threshold:  2,
		}
		c := &constraints{
			limit: 32,
		}

		Convey(`Loaded with data below the threshold`, func() {
			p.Append(data(s.now, 1))
			Convey(`Returns nil when reading data smaller than the thresold.`, func() {
				le, err := p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, ShouldBeNil)
			})

			Convey(`Returns a LogEntry when truncating.`, func() {
				c.allowSplit = true
				le, err := p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, shouldMatchLogEntry, s.le(0, logpb.Binary{
					Data: []byte{1},
				}))

				le, err = p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, ShouldBeNil)
			})
		})

		Convey(`Loaded with 10 bytes of data`, func() {
			p.Append(data(s.now, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9))

			Convey(`Should yield all data with a limit of 32.`, func() {
				le, err := p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, shouldMatchLogEntry, s.le(0, logpb.Binary{
					Data: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				}))

				le, err = p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, ShouldBeNil)
			})

			Convey(`Should yield [0..5], [6..9] with a limit of 6.`, func() {
				c.limit = 6
				le, err := p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, shouldMatchLogEntry, s.le(0, logpb.Binary{
					Data: []byte{0, 1, 2, 3, 4, 5},
				}))

				le, err = p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, shouldMatchLogEntry, s.le(6, logpb.Binary{
					Offset: 6,
					Data:   []byte{6, 7, 8, 9},
				}))

				le, err = p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, ShouldBeNil)
			})
		})

		Convey(`Loaded with 8 bytes of data from different times.`, func() {
			for i := 0; i < 8; i++ {
				p.Append(data(s.now.Add(time.Duration(i)*time.Second), byte(i)))
			}

			Convey(`Ignores the time boundary and returns all 8 bytes.`, func() {
				le, err := p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, shouldMatchLogEntry, s.le(0, logpb.Binary{
					Data: []byte{0, 1, 2, 3, 4, 5, 6, 7},
				}))

				le, err = p.nextEntry(c)
				So(err, ShouldBeNil)
				So(le, ShouldBeNil)
			})
		})
	})
}
