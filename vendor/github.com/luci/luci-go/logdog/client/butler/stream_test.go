// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package butler

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/iotools"
	"github.com/luci/luci-go/logdog/client/butler/bundler"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

type testBundlerStream struct {
	closed   bool
	appended []byte
	ts       []time.Time
	err      error

	data []*testBundlerData
}

func (bs *testBundlerStream) LeaseData() bundler.Data {
	d := &testBundlerData{
		value: make([]byte, 128),
	}
	bs.data = append(bs.data, d)
	return d
}

func (bs *testBundlerStream) Append(d bundler.Data) error {
	if bs.err != nil {
		return bs.err
	}

	tbd := d.(*testBundlerData)
	bs.appended = append(bs.appended, tbd.value...)
	bs.ts = append(bs.ts, d.Timestamp())
	tbd.Release()
	return nil
}

func (bs *testBundlerStream) Close() {
	if bs.closed {
		panic("double close")
	}
	bs.closed = true
}

func (bs *testBundlerStream) closedAndReleased() bool {
	for _, d := range bs.data {
		if !d.released {
			return false
		}
	}
	return bs.closed
}

type testBundlerData struct {
	value    []byte
	ts       time.Time
	bound    bool
	released bool
}

func (d *testBundlerData) Release() {
	if d.released {
		panic("double release")
	}
	d.released = true
}

func (d *testBundlerData) Bytes() []byte {
	return d.value
}

func (d *testBundlerData) Bind(c int, ts time.Time) bundler.Data {
	d.ts = ts
	d.value = d.value[:c]
	d.bound = true
	return d
}

func (d *testBundlerData) Timestamp() time.Time {
	if !d.bound {
		panic("not bound, no timestamp")
	}
	return d.ts
}

type testReadCloser struct {
	data   []byte
	err    error
	closed bool
}

func (rc *testReadCloser) Read(b []byte) (int, error) {
	if len(b) < len(rc.data) {
		panic("test data too large")
	}
	if rc.closed {
		return 0, io.EOF
	}
	return copy(b, rc.data), rc.err
}

func (rc *testReadCloser) Close() error {
	if rc.closed {
		panic("double close")
	}
	rc.closed = true
	return rc.err
}

func TestStream(t *testing.T) {
	Convey(`A testing stream`, t, func() {
		c, tc := testclock.UseTime(context.Background(), testclock.TestTimeUTC)
		bs := &testBundlerStream{}
		rc := &testReadCloser{}

		s := &stream{
			Context: c,
			r:       rc,
			c:       rc,
			bs:      bs,
		}

		Convey(`Will read chunks until EOF.`, func() {
			rc.data = []byte("foo")
			So(s.readChunk(), ShouldBeTrue)

			rc.data = []byte(nil)
			So(s.readChunk(), ShouldBeTrue)

			tc.Add(time.Second)
			rc.data = []byte("bar")
			So(s.readChunk(), ShouldBeTrue)

			s.closeStream()
			So(s.readChunk(), ShouldBeFalse)
			So(bs.appended, ShouldResemble, []byte("foobar"))
			So(bs.ts, ShouldResemble, []time.Time{testclock.TestTimeUTC, testclock.TestTimeUTC.Add(time.Second)})
			So(bs.closedAndReleased(), ShouldBeTrue)
		})

		Convey(`Will release Data on Append error.`, func() {
			bs.err = errors.New("test error")
			rc.data = []byte("bar")
			So(s.readChunk(), ShouldBeFalse)

			s.closeStream()
			So(bs.closedAndReleased(), ShouldBeTrue)
		})

		Convey(`Will read data and ignore timeout errors.`, func() {
			rc.data = []byte("foo")
			rc.err = iotools.ErrTimeout
			So(s.readChunk(), ShouldBeTrue)

			s.closeStream()
			So(bs.appended, ShouldResemble, []byte("foo"))
			So(bs.closedAndReleased(), ShouldBeTrue)
		})

		Convey(`Will halt if a stream error is encountered.`, func() {
			rc.data = []byte("foo")
			rc.err = errors.New("test error")
			So(s.readChunk(), ShouldBeFalse)

			s.closeStream()
			So(bs.appended, ShouldResemble, []byte("foo"))
			So(bs.closedAndReleased(), ShouldBeTrue)
		})

		Convey(`Will close Bundler Stream even if Closer returns an error.`, func() {
			rc.err = errors.New("test error")
			s.closeStream()
			So(bs.closedAndReleased(), ShouldBeTrue)
		})
	})
}
