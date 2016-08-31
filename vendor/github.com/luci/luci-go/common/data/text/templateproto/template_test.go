// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package templateproto

import (
	"testing"

	"github.com/golang/protobuf/proto"
	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func parse(template string) *File_Template {
	ret := &File_Template{}
	So(proto.UnmarshalText(template, ret), ShouldBeNil)
	return ret
}

func TestTemplateNormalize(t *testing.T) {
	t.Parallel()

	badCases := []struct {
		err, data string
	}{
		{"body is empty", ""},

		{"param \"\": invalid name", `body: "foofball" param: <key: "" value: <>>`},
		{"param \"$#foo\": malformed name", `body: "foofball" param: <key: "$#foo" value:<>>`},
		{"param \"${}\": malformed name", `body: "foofball" param: <key: "${}" value:<>>`},
		{"param \"blah${cool}nerd\": malformed name", `body: "foofball" param: <key: "blah${cool}nerd" value:<>>`},
		{"param \"${ok}\": not present in body", `body: "foofball" param: <key: "${ok}" value:<>>`},
		{"param \"${foof}\": schema: is nil", `body: "${foof}ball" param: <key: "${foof}" value:<>>`},
		{"param \"${foof}\": schema: has no type", `body: "${foof}ball" param: <key: "${foof}" value: <schema: <>>>`},

		{`param "${foof}": default value: type is "str", expected "int"`, `body: "${foof}ball" param: <key: "${foof}" value: <schema: <int:<>> default: <str: "">>>`},
		{"param \"${foof}\": default value: not nullable", `body: "${foof}ball" param: <key: "${foof}" value: <schema: <int:<>> default: <null: <>>>>`},
		{"param \"${foof}\": default value: invalid character 'q'", `body: "${foof}ball" param: <key: "${foof}" value: <schema: <object:<>> default: <object: "querp">>>`},

		{`param "${f}": schema: set requires entries`, `body: "${f}" param: <key: "${f}" value: <schema: <enum:<>>>>`},
		{`param "${f}": schema: blank token`, `body: "${f}" param: <key: "${f}" value: <schema: <enum:<entry: <>>>>>`},
		{`param "${f}": schema: duplicate token "tok"`, `body: "{\"k\": ${f}}" param: <key: "${f}" value: <schema: <enum:<entry: <token: "tok"> entry: <token: "tok">>>>>`},

		{"parsing rendered body: invalid character 'w'", `body: "wumpus"`},
	}

	Convey("File_Template.Normalize", t, func() {
		Convey("bad", func() {

			for _, tc := range badCases {
				Convey(tc.err, func() {
					t := parse(tc.data)
					So(t.Normalize(), ShouldErrLike, tc.err)
				})
			}

			Convey("length", func() {
				Convey("string", func() {
					t := parse(`
					body: "{\"key\": ${foof}}"
					param: <
						key: "${foof}"
						value: <
							schema: <str:<max_length: 1>>
						>
					>
					`)
					So(t.Normalize(), ShouldBeNil)
					_, err := t.RenderL(LiteralMap{"${foof}": "hi there"})
					So(err, ShouldErrLike, "param \"${foof}\": value is too large")
				})

				Convey("bytes", func() {
					t := parse(`
					body: "{\"key\": ${foof}}"
					param: <
						key: "${foof}"
						value: <
							schema: <bytes:<max_length: 1>>
						>
					>
					`)
					So(t.Normalize(), ShouldBeNil)
					_, err := t.RenderL(LiteralMap{"${foof}": []byte("hi there")})
					So(err, ShouldErrLike, "param \"${foof}\": value is too large")
				})

				Convey("object", func() {
					t := parse(`
					body: "{\"key\": ${foof}}"
					param: <
						key: "${foof}"
						value: <
							schema: <object:<max_length: 4>>
						>
					>
					`)
					So(t.Normalize(), ShouldBeNil)
					_, err := t.RenderL(LiteralMap{"${foof}": map[string]interface{}{"hi": 1}})
					So(err, ShouldErrLike, "param \"${foof}\": value is too large")
				})

				Convey("enum", func() {
					t := parse(`
					body: "{\"key\": ${foof}}"
					param: <
						key: "${foof}"
						value: <
							schema: <enum:<
								entry: <token: "foo">
							>>
						>
					>
					`)
					So(t.Normalize(), ShouldBeNil)
					_, err := t.RenderL(LiteralMap{"${foof}": "bar"})
					So(err, ShouldErrLike, "param \"${foof}\": value does not match enum: \"bar\"")
				})
			})

			Convey("parse", func() {
				Convey("not obj value", func() {
					m, err := (LiteralMap{"$key": &Value_Object{"querp"}}).Convert()
					So(err, ShouldBeNil)
					spec := &Specifier{"thing", m}
					So(spec.Normalize(), ShouldErrLike, "param \"$key\": invalid character 'q'")
				})

				Convey("not ary value", func() {
					m, err := (LiteralMap{"$key": &Value_Array{"querp"}}).Convert()
					So(err, ShouldBeNil)
					spec := &Specifier{"thing", m}
					So(spec.Normalize(), ShouldErrLike, "param \"$key\": invalid character 'q'")
				})
			})

			Convey("required param", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <int: <>>
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				_, err := t.Render(nil)
				So(err, ShouldErrLike, "param \"${value}\": missing")
			})

			Convey("extra param", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <int: <>>
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				_, err := t.RenderL(LiteralMap{"foo": nil, "${value}": 1})
				So(err, ShouldErrLike, "unknown parameters: [\"foo\"]")
			})

			Convey("bad type", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <int: <>>
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				_, err := t.RenderL(LiteralMap{"${value}": nil})
				So(err, ShouldErrLike, "param \"${value}\": not nullable")
			})

			Convey("prevents JSONi", func() {
				Convey("object", func() {
					m, err := (LiteralMap{
						"${value}": &Value_Object{`{}, "otherKey": {}`}}).Convert()
					So(err, ShouldBeNil)
					spec := &Specifier{"thing", m}
					So(spec.Normalize(), ShouldErrLike, "param \"${value}\": got extra junk")

					spec.Params["${value}"].Value.(*Value_Object).Object = `{"extra": "space"}      `
					So(spec.Normalize(), ShouldBeNil)
					So(spec.Params["${value}"].GetObject(), ShouldEqual, `{"extra":"space"}`)
				})

				Convey("array", func() {
					va := &Value_Array{`[], "otherKey": []`}
					So(va.Normalize(), ShouldErrLike, "got extra junk")
				})
			})
		})

		Convey("good", func() {
			Convey("default", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <int: <>>
						default: <int: 20>
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				doc, err := t.Render(nil)
				So(err, ShouldBeNil)
				So(doc, ShouldEqual, `{"key": 20}`)
			})

			Convey("big int", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <int: <>>
						default: <int: 9223372036854775807>
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				doc, err := t.Render(nil)
				So(err, ShouldBeNil)
				So(doc, ShouldEqual, `{"key": "9223372036854775807"}`)
			})

			Convey("big uint", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <uint: <>>
						default: <uint: 18446744073709551615>
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				doc, err := t.Render(nil)
				So(err, ShouldBeNil)
				So(doc, ShouldEqual, `{"key": "18446744073709551615"}`)
			})

			Convey("param", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <uint: <>>
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				doc, err := t.RenderL(LiteralMap{"${value}": uint(19)})
				So(err, ShouldBeNil)
				So(doc, ShouldEqual, `{"key": 19}`)
			})

			Convey("float", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <float: <>>
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				doc, err := t.RenderL(LiteralMap{"${value}": 19.1})
				So(err, ShouldBeNil)
				So(doc, ShouldEqual, `{"key": 19.1}`)
			})

			Convey("boolean", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <bool: <>>
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				doc, err := t.RenderL(LiteralMap{"${value}": true})
				So(err, ShouldBeNil)
				So(doc, ShouldEqual, `{"key": true}`)
			})

			Convey("array", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <array: <>>
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				doc, err := t.RenderL(LiteralMap{"${value}": []interface{}{"hi", 20}})
				So(err, ShouldBeNil)
				So(doc, ShouldEqual, `{"key": ["hi",20]}`)
			})

			Convey("null", func() {
				t := parse(`
				body: "{\"key\": ${value}}"
				param: <
					key: "${value}"
					value: <
						schema: <int: <>>
						nullable: true
					>
				>
				`)
				So(t.Normalize(), ShouldBeNil)
				doc, err := t.RenderL(LiteralMap{"${value}": nil})
				So(err, ShouldBeNil)
				So(doc, ShouldEqual, `{"key": null}`)
			})
		})
	})
}
