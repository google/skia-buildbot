// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamclient

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/luci/luci-go/common/clock/clockflag"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/recordio"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	. "github.com/smartystreets/goconvey/convey"
)

type testStreamWriteCloser struct {
	bytes.Buffer
	addr string

	err    error
	closed bool
}

func (ts *testStreamWriteCloser) Write(d []byte) (int, error) {
	if ts.err != nil {
		return 0, ts.err
	}
	return ts.Buffer.Write(d)
}

func (ts *testStreamWriteCloser) Close() error {
	ts.closed = true
	return nil
}

func TestClient(t *testing.T) {
	Convey(`A client registry with a test protocol`, t, func() {
		tswcErr := error(nil)
		tswc := (*testStreamWriteCloser)(nil)

		reg := Registry{}
		reg.Register("test", func(addr string) (Client, error) {
			return &clientImpl{
				factory: func() (io.WriteCloser, error) {
					tswc = &testStreamWriteCloser{
						addr: addr,
						err:  tswcErr,
					}
					return tswc, nil
				},
			}, nil
		})

		flags := streamproto.Flags{
			Name:      "test",
			Timestamp: clockflag.Time(testclock.TestTimeUTC),
		}

		Convey(`Will panic if the same protocol is registered twice.`, func() {
			So(func() { reg.Register("test", nil) }, ShouldPanic)
			So(func() { reg.Register("test2", nil) }, ShouldNotPanic)
		})

		Convey(`Will fail to instantiate a Client with an invalid protocol.`, func() {
			_, err := reg.NewClient("fake:foo")
			So(err, ShouldNotBeNil)
		})

		Convey(`Can instantiate a new client.`, func() {
			client, err := reg.NewClient("test:foo")
			So(err, ShouldBeNil)
			So(client, ShouldHaveSameTypeAs, &clientImpl{})

			Convey(`That can instantiate new Streams.`, func() {
				stream, err := client.NewStream(flags)
				So(err, ShouldBeNil)
				So(stream, ShouldHaveSameTypeAs, &streamImpl{})

				si := stream.(*streamImpl)
				So(si.WriteCloser, ShouldHaveSameTypeAs, &testStreamWriteCloser{})

				tswc := si.WriteCloser.(*testStreamWriteCloser)
				So(tswc.addr, ShouldEqual, "foo")

				Convey(`The stream should have the stream header written to it.`, func() {
					So(tswc.Next(len(streamproto.ProtocolFrameHeaderMagic)), ShouldResemble,
						streamproto.ProtocolFrameHeaderMagic)

					r := recordio.NewReader(tswc, -1)
					f, err := r.ReadFrameAll()
					So(err, ShouldBeNil)
					So(string(f), ShouldResemble, `{"name":"test","timestamp":"0001-02-03T04:05:06.000000007Z"}`)
				})
			})

			Convey(`If the stream fails to write the handshake, it will be closed.`, func() {
				tswcErr = errors.New("test error")
				_, err := client.NewStream(flags)
				So(err, ShouldNotBeNil)

				So(tswc, ShouldNotBeNil)
				So(tswc.closed, ShouldBeTrue)
			})
		})
	})
}
