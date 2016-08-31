// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package butlerproto

import (
	"bytes"
	"io"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/data/recordio"
	"github.com/luci/luci-go/common/testing/assertions"
	"github.com/luci/luci-go/logdog/api/logpb"
	. "github.com/smartystreets/goconvey/convey"
)

func read(ir io.Reader) (*Reader, error) {
	r := Reader{}
	if err := r.Read(ir); err != nil {
		return nil, err
	}
	return &r, nil
}

func TestReader(t *testing.T) {
	Convey(`A Reader instance`, t, func() {
		r := Reader{}
		buf := bytes.Buffer{}
		fw := recordio.NewWriter(&buf)

		writeFrame := func(data []byte) error {
			if _, err := fw.Write(data); err != nil {
				return err
			}
			if err := fw.Flush(); err != nil {
				return err
			}
			return nil
		}

		push := func(m proto.Message) {
			data, err := proto.Marshal(m)
			if err != nil {
				panic(err)
			}
			if err := writeFrame(data); err != nil {
				panic(err)
			}
		}

		// Test case templates.
		md := logpb.ButlerMetadata{
			Type:         logpb.ButlerMetadata_ButlerLogBundle,
			ProtoVersion: logpb.Version,
		}
		bundle := logpb.ButlerLogBundle{}

		Convey(`Can read a ButlerLogBundle entry.`, func() {
			push(&md)
			push(&bundle)

			So(r.Read(&buf), ShouldBeNil)
			So(r.Bundle, ShouldNotBeNil)
		})

		Convey(`Will fail to Read an unknown type.`, func() {
			// Assert that we are testing an unknown type.
			unknownType := logpb.ButlerMetadata_ContentType(-1)
			So(logpb.ButlerMetadata_ContentType_name[int32(unknownType)], ShouldEqual, "")

			md.Type = unknownType
			push(&md)

			err := r.Read(&buf)
			So(err, ShouldNotBeNil)
			So(err, assertions.ShouldErrLike, "unknown data type")
		})

		Convey(`Will not decode contents if an unknown protocol version is identified.`, func() {
			// Assert that we are testing an unknown type.
			md.ProtoVersion = "DEFINITELY NOT VALID"
			push(&md)
			push(&bundle)

			So(r.Read(&buf), ShouldBeNil)
			So(r.Bundle, ShouldBeNil)
		})

		Convey(`Will fail to read junk metadata.`, func() {
			So(writeFrame([]byte{0xd0, 0x6f, 0x00, 0xd5}), ShouldBeNil)

			err := r.Read(&buf)
			So(err, ShouldNotBeNil)
			So(err, assertions.ShouldErrLike, "failed to unmarshal Metadata frame")
		})

		Convey(`With a proper Metadata frame`, func() {
			push(&md)

			Convey(`Will fail if the bundle data is junk.`, func() {
				So(writeFrame([]byte{0xd0, 0x6f, 0x00, 0xd5}), ShouldBeNil)

				err := r.Read(&buf)
				So(err, ShouldNotBeNil)
				So(err, assertions.ShouldErrLike, "failed to unmarshal Bundle frame")
			})
		})

		Convey(`With a proper compressed Metadata frame`, func() {
			md.Compression = logpb.ButlerMetadata_ZLIB
			push(&md)

			Convey(`Will fail if the data frame is missing.`, func() {
				err := r.Read(&buf)
				So(err, ShouldNotBeNil)
				So(err, assertions.ShouldErrLike, "failed to read Bundle data")
			})

			Convey(`Will fail if there is junk compressed data.`, func() {
				So(writeFrame(bytes.Repeat([]byte{0x55, 0xAA}, 16)), ShouldBeNil)

				err := r.Read(&buf)
				So(err, ShouldNotBeNil)
				So(err, assertions.ShouldErrLike, "failed to initialize zlib reader")
			})
		})

		Convey(`Will refuse to read a frame larger than our maximum size.`, func() {
			r.maxSize = 16
			So(writeFrame(bytes.Repeat([]byte{0x55}, 17)), ShouldBeNil)

			err := r.Read(&buf)
			So(err, ShouldEqual, recordio.ErrFrameTooLarge)
		})

		Convey(`Will refuse to read a compressed protobuf larger than our maximum size.`, func() {
			// We are crafting this data such that its compressed (frame) size is
			// below our threshold (16), but its compressed size exceeds it. Repeated
			// bytes compress very well :)
			//
			// Because the frame it smaller than our threshold, our FrameReader will
			// not outright reject the frame. However, the data is still larger than
			// we're allowed, and we must reject it.
			r.maxSize = 16
			w := Writer{
				Compress:          true,
				CompressThreshold: 0,
			}
			So(w.writeData(recordio.NewWriter(&buf), logpb.ButlerMetadata_ButlerLogBundle,
				bytes.Repeat([]byte{0x55}, 64)), ShouldBeNil)

			err := r.Read(&buf)
			So(err, ShouldNotBeNil)
			So(err, assertions.ShouldErrLike, "limit exceeded")
		})
	})
}

func TestWriter(t *testing.T) {
	Convey(`A Writer instance outputting to a Buffer`, t, func() {
		buf := bytes.Buffer{}
		w := Writer{}
		bundle := logpb.ButlerLogBundle{
			Entries: []*logpb.ButlerLogBundle_Entry{
				{},
			},
		}

		Convey(`When configured to compress with a threshold of 64`, func() {
			w.Compress = true
			w.CompressThreshold = 64

			Convey(`Will not compress if below the compression threshold.`, func() {
				So(w.Write(&buf, &bundle), ShouldBeNil)

				r, err := read(&buf)
				So(err, ShouldBeNil)
				So(r.Metadata.Compression, ShouldEqual, logpb.ButlerMetadata_NONE)
				So(r.Metadata.ProtoVersion, ShouldEqual, logpb.Version)
			})

			Convey(`Will not write data larger than the maximum bundle size.`, func() {
				w.maxSize = 16
				bundle.Secret = bytes.Repeat([]byte{'A'}, 17)
				err := w.Write(&buf, &bundle)
				So(err, ShouldNotBeNil)
				So(err, assertions.ShouldErrLike, "exceeds soft cap")
			})

			Convey(`Will compress data >= the threshold.`, func() {
				bundle.Secret = bytes.Repeat([]byte{'A'}, 64)
				So(w.Write(&buf, &bundle), ShouldBeNil)

				r, err := read(&buf)
				So(err, ShouldBeNil)
				So(r.Metadata.Compression, ShouldEqual, logpb.ButlerMetadata_ZLIB)
				So(r.Metadata.ProtoVersion, ShouldEqual, logpb.Version)

				Convey(`And can be reused.`, func() {
					So(w.Write(&buf, &bundle), ShouldBeNil)

					r, err := read(&buf)
					So(err, ShouldBeNil)
					So(r.Metadata.Compression, ShouldEqual, logpb.ButlerMetadata_ZLIB)
					So(r.Metadata.ProtoVersion, ShouldEqual, logpb.Version)
				})
			})
		})
	})
}
