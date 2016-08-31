// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package flagpb

import (
	"io/ioutil"
	"testing"

	"github.com/golang/protobuf/proto"
	"google.golang.org/genproto/protobuf"

	"github.com/luci/luci-go/common/proto/google/descutil"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	Convey("Unmarshal", t, func() {
		descFileBytes, err := ioutil.ReadFile("unmarshal_test.desc")
		So(err, ShouldBeNil)

		var desc descriptor.FileDescriptorSet
		err = proto.Unmarshal(descFileBytes, &desc)
		So(err, ShouldBeNil)

		resolver := NewResolver(&desc)

		resolveMsg := func(name string) *descriptor.DescriptorProto {
			_, obj, _ := descutil.Resolve(&desc, "flagpb."+name)
			So(obj, ShouldNotBeNil)
			return obj.(*descriptor.DescriptorProto)
		}

		unmarshalOK := func(typeName string, args ...string) map[string]interface{} {
			msg, err := UnmarshalUntyped(args, resolveMsg(typeName), resolver)
			So(err, ShouldBeNil)
			return msg
		}

		unmarshalErr := func(typeName string, args ...string) error {
			msg, err := UnmarshalUntyped(args, resolveMsg(typeName), resolver)
			So(msg, ShouldBeNil)
			return err
		}

		Convey("empty", func() {
			So(unmarshalOK("M1"), ShouldResemble, msg())
		})
		Convey("non-flag", func() {
			So(unmarshalErr("M1", "abc"), ShouldErrLike, `abc: a flag was expected`)
		})
		Convey("string", func() {
			Convey("next arg", func() {
				So(unmarshalOK("M1", "-s", "x"), ShouldResemble, msg("s", "x"))
			})
			Convey("equals sign", func() {
				So(unmarshalOK("M1", "-s=x"), ShouldResemble, msg("s", "x"))
			})
		})
		Convey("int32", func() {
			Convey("next arg", func() {
				So(unmarshalOK("M1", "-i", "1"), ShouldResemble, msg("i", int32(1)))
			})
			Convey("equals sign", func() {
				So(unmarshalOK("M1", "-i=1"), ShouldResemble, msg(
					"i", int32(1),
				))
			})
			Convey("error", func() {
				So(unmarshalErr("M1", "-i", "abc"), ShouldErrLike, "invalid syntax")
				So(unmarshalErr("M1", "-i=abc"), ShouldErrLike, "invalid syntax")
			})
		})
		Convey("enum", func() {
			Convey("by name", func() {
				So(unmarshalOK("M2", "-e", "V0"), ShouldResemble, msg("e", int32(0)))
			})
			Convey("error", func() {
				So(unmarshalErr("M2", "-e", "abc"), ShouldErrLike, `invalid value "abc" for enum E`)
			})
			Convey("by value", func() {
				So(unmarshalOK("M2", "-e", "0"), ShouldResemble, msg("e", int32(0)))
			})
		})
		Convey("bool", func() {
			Convey("without value", func() {
				So(unmarshalOK("M1", "-b"), ShouldResemble, msg("b", true))

				So(unmarshalOK("M1", "-b", "-s", "x"), ShouldResemble, msg(
					"b", true,
					"s", "x",
				))
			})
			Convey("without value, repeated", func() {
				So(unmarshalOK("M1", "-rb=false", "-rb"), ShouldResemble, msg("rb", repeated(false, true)))
			})
			Convey("with value", func() {
				So(unmarshalOK("M1", "-b=true"), ShouldResemble, msg("b", true))
				So(unmarshalOK("M1", "-b=false"), ShouldResemble, msg("b", false))
			})
		})
		Convey("bytes", func() {
			Convey("next arg", func() {
				So(unmarshalOK("M1", "-bb", "6869"), ShouldResemble, msg("bb", []byte("hi")))
			})
			Convey("equals sign", func() {
				So(unmarshalOK("M1", "-bb=6869"), ShouldResemble, msg("bb", []byte("hi")))
			})
			Convey("error", func() {
				So(unmarshalErr("M1", "-bb", "xx"), ShouldErrLike, "invalid byte: U+0078 'x'")
			})
		})

		Convey("many dashes", func() {
			Convey("2", func() {
				So(unmarshalOK("M1", "--s", "x"), ShouldResemble, msg("s", "x"))
			})
			Convey("3", func() {
				So(unmarshalErr("M1", "---s", "x"), ShouldErrLike, "---s: bad flag syntax")
			})
		})

		Convey("field not found", func() {
			So(unmarshalErr("M2", "-abc", "abc"), ShouldErrLike, `-abc: field abc not found in message M2`)
		})
		Convey("value not specified", func() {
			So(unmarshalErr("M1", "-s"), ShouldErrLike, `value was expected`)
		})

		Convey("message", func() {
			Convey("level 1", func() {
				So(unmarshalOK("M2", "-m1.s", "x"), ShouldResemble, msg(
					"m1", msg("s", "x"),
				))
				So(unmarshalOK("M2", "-m1.s", "x", "-m1.b"), ShouldResemble, msg(
					"m1", msg(
						"s", "x",
						"b", true,
					),
				))
			})
			Convey("level 2", func() {
				So(unmarshalOK("M3", "-m2.m1.s", "x"), ShouldResemble, msg(
					"m2", msg(
						"m1", msg("s", "x"),
					),
				))
			})
			Convey("not found", func() {
				So(unmarshalErr("M2", "-abc.s", "x"), ShouldErrLike, `field "abc" not found in message M2`)
			})
			Convey("non-msg subfield", func() {
				So(unmarshalErr("M1", "-s.dummy", "x"), ShouldErrLike, "field s is not a message")
			})
			Convey("message value", func() {
				const err = "M2.m1 is a message field. Specify its field values, not the message itself"
				So(unmarshalErr("M2", "-m1", "x"), ShouldErrLike, err)
				So(unmarshalErr("M2", "-m1=x"), ShouldErrLike, err)
			})
		})

		Convey("string and int32", func() {
			So(unmarshalOK("M1", "-s", "x", "-i", "1"), ShouldResemble, msg(
				"s", "x",
				"i", int32(1),
			))
		})

		Convey("repeated", func() {
			Convey("int32", func() {
				So(unmarshalOK("M1", "-ri", "1", "-ri", "2"), ShouldResemble, msg(
					"ri", repeated(int32(1), int32(2)),
				))
			})
			Convey("submessage string", func() {
				Convey("works", func() {
					So(unmarshalOK("M3", "-m1.s", "x", "-m1", "-m1.s", "y"), ShouldResemble, msg(
						"m1", repeated(
							msg("s", "x"),
							msg("s", "y"),
						),
					))
				})
				Convey("reports meaningful error", func() {
					err := unmarshalErr("M3", "-m1.s", "x", "-m1.s", "y")
					So(err, ShouldErrLike, `-m1.s: value is already set`)
					So(err, ShouldErrLike, `insert -m1`)
				})
			})
		})

		Convey("map", func() {
			Convey("map<string, string>", func() {
				So(unmarshalOK("MapContainer", "-ss.x", "a", "-ss.y", "b"), ShouldResemble, msg(
					"ss", msg("x", "a", "y", "b"),
				))
			})
			Convey("map<int32, int32>", func() {
				So(unmarshalOK("MapContainer", "-ii.1", "10", "-ii.2", "20"), ShouldResemble, msg(
					"ii", msg("1", int32(10), "2", int32(20)),
				))
			})
			Convey("map<string, M1>", func() {
				So(unmarshalOK("MapContainer", "-sm1.x.s", "a", "-sm1.y.s", "b"), ShouldResemble, msg(
					"sm1", msg(
						"x", msg("s", "a"),
						"y", msg("s", "b"),
					),
				))
			})
		})
	})
}
