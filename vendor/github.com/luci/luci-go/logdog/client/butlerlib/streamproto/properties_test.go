// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamproto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

// Test the TeeType struct.
func TestProperties(t *testing.T) {
	Convey(`A testing instance`, t, func() {
		ctx, _ := testclock.UseTime(context.Background(), time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC))
		Convey(`A Properties instance with no ContentType`, func() {
			p := Properties{}

			Convey(`Returns the configured ContentType if one is set.`, func() {
				p.ContentType = "foo/bar"
				p.StreamType = logpb.StreamType_TEXT
				So(p.ContentType, ShouldEqual, "foo/bar")
			})

			Convey(`Will fail to validate if a Prefix is set.`, func() {
				p.Prefix = "some/prefix"
				So(p.Validate(), ShouldNotBeNil)
			})

			Convey(`Will fail to validate if its LogStreamDescriptor is invalid.`, func() {
				So(p.Validate(), ShouldNotBeNil)
			})

			Convey(`Will validate if valid.`, func() {
				p.Name = "foo/bar"
				p.ContentType = "some/mimetype"
				p.Timestamp = google.NewTimestamp(clock.Now(ctx))
				So(p.Validate(), ShouldBeNil)
			})
		})
	})
}

func TestFlags(t *testing.T) {
	Convey(`A Flags instance`, t, func() {
		f := Flags{
			Name:        "my/stream",
			ContentType: "foo/bar",
		}

		Convey(`Converts to Properties.`, func() {
			p := f.Properties()
			So(p, ShouldResemble, &Properties{
				LogStreamDescriptor: logpb.LogStreamDescriptor{
					Name:        "my/stream",
					ContentType: "foo/bar",
					StreamType:  logpb.StreamType_TEXT,
				},
				Tee: TeeNone,
			})
		})
	})
}

func TestFlagsJSON(t *testing.T) {
	Convey(`A Flags instance`, t, func() {
		f := Flags{}
		Convey(`Will decode a TEXT stream.`, func() {
			t := `{"name": "my/stream", "contentType": "foo/bar", "type": "text"}`
			So(json.Unmarshal([]byte(t), &f), ShouldBeNil)

			So(f.Properties(), ShouldResemble, &Properties{
				LogStreamDescriptor: logpb.LogStreamDescriptor{
					Name:        "my/stream",
					ContentType: "foo/bar",
					StreamType:  logpb.StreamType_TEXT,
				},
				Tee: TeeNone,
			})
		})

		Convey(`Will fail to decode an invalid stream type.`, func() {
			t := `{"name": "my/stream", "type": "XXX_whatisthis?"}`
			So(json.Unmarshal([]byte(t), &f), ShouldNotBeNil)
		})

		Convey(`Will decode a BINARY stream.`, func() {
			t := `{"name": "my/stream", "type": "binary"}`
			So(json.Unmarshal([]byte(t), &f), ShouldBeNil)

			So(f.Properties(), ShouldResemble, &Properties{
				LogStreamDescriptor: logpb.LogStreamDescriptor{
					Name:        "my/stream",
					StreamType:  logpb.StreamType_BINARY,
					ContentType: string(types.ContentTypeBinary),
				},
				Tee: TeeNone,
			})
		})

		Convey(`Will decode a DATAGRAM stream.`, func() {
			t := `{"name": "my/stream", "type": "datagram"}`
			So(json.Unmarshal([]byte(t), &f), ShouldBeNil)

			So(f.Properties(), ShouldResemble, &Properties{
				LogStreamDescriptor: logpb.LogStreamDescriptor{
					Name:        "my/stream",
					StreamType:  logpb.StreamType_DATAGRAM,
					ContentType: string(types.ContentTypeLogdogDatagram),
				},
				Tee: TeeNone,
			})
		})
	})
}
