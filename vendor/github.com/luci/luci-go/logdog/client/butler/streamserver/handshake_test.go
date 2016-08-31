// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamserver

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	"github.com/luci/luci-go/logdog/common/types"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func writePanic(w io.Writer, d []byte) {
	amt, err := w.Write(d)
	if err != nil {
		panic(err)
	}
	if amt != len(d) {
		panic("failed to write full buffer")
	}
}

type handshakeBuilder struct {
	magic []byte // The frame header. If empty, don't include a frame header.
	size  uint64 // The size. If zero, calculate the size.
}

func (b *handshakeBuilder) writeTo(w io.Writer, handshake string, data []byte) {
	// Frame header
	if len(b.magic) > 0 {
		writePanic(w, b.magic)
	}

	// Size
	size := b.size
	if size == 0 {
		size = uint64(len(handshake))
	}
	sizeBuf := make([]byte, binary.MaxVarintLen64)
	count := binary.PutUvarint(sizeBuf, uint64(size))
	writePanic(w, sizeBuf[:count])

	if handshake != "" {
		writePanic(w, []byte(handshake))
	}
	writePanic(w, data)
}

// Generate a reader from the configuration.
func (b *handshakeBuilder) reader(handshake string, data []byte) io.Reader {
	r := bytes.Buffer{}
	b.writeTo(&r, handshake, data)
	return &r
}

// Tests for handshakeProtocol
func testHandshakeProtocol(t *testing.T, verbose bool) {
	Convey(fmt.Sprintf(`A handshakeProtocol instance (verbose=%v)`, verbose), t, func() {
		hb := handshakeBuilder{
			magic: streamproto.ProtocolFrameHeaderMagic,
		}

		ctx, tc := testclock.UseTime(context.Background(), testclock.TestTimeUTC)
		p := &handshakeProtocol{}
		p.forceVerbose = verbose

		Convey(`Will fail if no handshake data is provided.`, func() {
			hb.magic = []byte{}

			_, err := p.Handshake(ctx, hb.reader("{}", nil))
			So(err, ShouldNotBeNil)
		})

		Convey(`Will fail with an invalid handshake protocol.`, func() {
			hb.magic = []byte{0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA}

			_, err := p.Handshake(ctx, hb.reader("{}", nil))
			So(err, ShouldNotBeNil)
		})

		Convey(`Loading a handshake frame starting with an invalid size varint value must fail.`, func() {
			// This exceeds the maximum 64-bit varint size (10 bytes) and never
			// terminates (no MSB).
			_, err := p.Handshake(ctx, bytes.NewReader([]byte{
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
			}))
			So(err, ShouldNotBeNil)
		})

		Convey(`Loading a handshake frame larger than the maximum header size must fail.`, func() {
			hb.size = maxHeaderSize + 1
			_, err := p.Handshake(ctx, hb.reader("", nil))
			So(err, ShouldNotBeNil)
		})

		Convey(`Loading an JSON object with just a name`, func() {
			props, err := p.Handshake(ctx, hb.reader(`{"name": "test"}`, nil))
			So(err, ShouldBeNil)

			Convey(`Should produce a valid stream configuration.`, func() {
				So(props, ShouldResemble, &streamproto.Properties{
					LogStreamDescriptor: logpb.LogStreamDescriptor{
						Name:        "test",
						Timestamp:   google.NewTimestamp(tc.Now()),
						StreamType:  logpb.StreamType_TEXT,
						ContentType: string(types.ContentTypeText),
					},
				})
			})
		})

		Convey(`Loading a fully-specified configuration`, func() {
			data := `{
				"name": "test", "tee": "stdout", "timestamp": "2015-05-07T01:29:51+00:00",
				"contentType": "text/plain",
				"tags": {"foo": "bar", "baz": "qux"}
			}`
			props, err := p.Handshake(ctx, hb.reader(data, nil))
			So(err, ShouldBeNil)

			Convey(`Should produce a specific configuration.`, func() {
				So(props, ShouldResemble, &streamproto.Properties{
					LogStreamDescriptor: logpb.LogStreamDescriptor{
						Name:        "test",
						ContentType: "text/plain",
						Timestamp:   google.NewTimestamp(time.Date(2015, 05, 07, 1, 29, 51, 0, time.UTC)),
						Tags: map[string]string{
							"baz": "qux",
							"foo": "bar",
						},
					},
					Tee: streamproto.TeeStdout,
				})
			})
		})

		Convey(`Loading a (valid) JSON array should fail to load.`, func() {
			data := `["This is an array!"]`
			_, err := p.Handshake(ctx, hb.reader(data, nil))
			So(err, ShouldNotBeNil)
		})

		Convey(`Loading a JSON descriptor with just a name should succeed.`, func() {
			data := `{"name": "test"}`
			props, err := p.Handshake(ctx, hb.reader(data, nil))
			So(err, ShouldBeNil)
			So(props, ShouldNotBeNil)
		})

		Convey(`Loading an empty JSON object with a larger-than-necessary header size should fail.`, func() {
			data := `{}`
			hb.size = uint64(len(data) + 10)
			_, err := p.Handshake(ctx, hb.reader(data, nil))
			So(err, ShouldNotBeNil)
		})

		Convey(`Loading an JSON with an erroneous config should fail.`, func() {
			data := `{"timestamp": "text-for-some-reason"}`
			_, err := p.Handshake(ctx, hb.reader(data, nil))
			So(err, ShouldNotBeNil)
		})

		Convey(`Loading an invalid JSON descriptor should fail.`, func() {
			data := `invalid`
			_, err := p.Handshake(ctx, hb.reader(data, nil))
			So(err, ShouldNotBeNil)
		})

		for idx, v := range []string{"none", "stdout", "stderr", "clearly invalid"} {
			Convey(fmt.Sprintf(`A protocol with a tee type of: %s`, v), func() {
				ctx := context.Background()
				data := fmt.Sprintf(`{"name": "test", "tee": "%s"}`, v)
				_, err := p.Handshake(ctx, hb.reader(data, nil))
				if idx <= 2 {
					Convey(`Should successfully parse.`, func() {
						So(err, ShouldBeNil)
					})
				} else {
					Convey(`Should fail to parse.`, func() {
						So(err, ShouldNotBeNil)
					})
				}
			})
		}
	})
}

func TestHandshakeProtocol(t *testing.T) {
	testHandshakeProtocol(t, false)
}

// As an optimization, we buffer data differently for verbose output. This
// creates a separate code path that we have to take if logging verbose function
// is set.
func TestHandshakeProtocolVerbose(t *testing.T) {
	testHandshakeProtocol(t, true)
}
