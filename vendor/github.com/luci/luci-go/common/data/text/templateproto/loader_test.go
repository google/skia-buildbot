// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package templateproto

import (
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/config/impl/memory"
	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLoadFromConfig(t *testing.T) {
	t.Parallel()

	Convey("LoadFile", t, func() {
		c := config.SetImplementation(context.Background(), memory.New(map[string]memory.ConfigSet{"projects/foo": {"templates/bar.cfg": `
		template: <
			key: "hardcode"
			value: <
				doc: "it's hard-coded"
				body: <<EOF
					{"woot": ["sauce"]}
				EOF
			>
		>

		template: <
			key: "templ_1"
			value: <
				doc: << EOF
					This template is templ_1!
					It's pretty "exciting"!
				EOF
				body: << EOF
					{
						"json_key": ${json_key},
						"cmd": ["array", "of", ${thing}],
						"extra": ${extra}
					}
				EOF
				param: <
					key: "${json_key}"
					value: <
						doc: "it's a json key"
						schema: <int:<>>
					>
				>
				param: <
					key: "${thing}"
					value: <
						doc: << EOF
							${thing} represents a color or a fruit
						EOF
						schema: <enum:<
							entry: < doc: "fruit" token: "banana" >
							entry: < doc: "color" token: "white" >
							entry: < doc: "color" token: "purple" >
						>>
					>
				>
				param: <
					key: "${extra}"
					value: <
						nullable: true
						schema:<object:<>>
						default: <object: <<EOF
							{"yes": "please"}
						EOF
						>
					>
				>
			>
		>
		`}}))

		Convey("basic load", func() {
			file, vers, err := LoadFile(c, "projects/foo", "templates/bar.cfg")
			So(err, ShouldBeNil)
			So(vers, ShouldEqual, "v1:989dbfaacc4565bcf1491388c912e7afc378d19d")
			So(file, ShouldResemble, &File{Template: map[string]*File_Template{
				"hardcode": {
					Doc:  "it's hard-coded",
					Body: `{"woot": ["sauce"]}`,
				},

				"templ_1": {
					Doc:  "This template is templ_1!\nIt's pretty \"exciting\"!",
					Body: "{\n\t\"json_key\": ${json_key},\n\t\"cmd\": [\"array\", \"of\", ${thing}],\n\t\"extra\": ${extra}\n}",
					Param: map[string]*File_Template_Parameter{
						"${json_key}": {
							Doc:    "it's a json key",
							Schema: &Schema{&Schema_Int{&Schema_Atom{}}},
						},
						"${thing}": {
							Doc: "${thing} represents a color or a fruit",
							Schema: &Schema{
								&Schema_Enum{&Schema_Set{Entry: []*Schema_Set_Entry{
									{Doc: "fruit", Token: "banana"},
									{Doc: "color", Token: "white"},
									{Doc: "color", Token: "purple"},
								}}},
							},
						},
						"${extra}": {
							Nullable: true,
							Schema:   &Schema{&Schema_Object{&Schema_JSON{}}},
							Default:  MustNewValue(map[string]interface{}{"yes": "please"}),
						},
					},
				},
			}})

			Convey("basic render", func() {
				ret, err := file.RenderL("templ_1", LiteralMap{"${thing}": "white", "${json_key}": 20})
				So(err, ShouldBeNil)
				So(ret, ShouldEqual, `{
	"json_key": 20,
	"cmd": ["array", "of", "white"],
	"extra": {"yes":"please"}
}`)
			})

			Convey("null override", func() {
				ret, err := file.RenderL("templ_1", LiteralMap{"${thing}": "white", "${json_key}": 20, "${extra}": nil})
				So(err, ShouldBeNil)
				So(ret, ShouldEqual, `{
	"json_key": 20,
	"cmd": ["array", "of", "white"],
	"extra": null
}`)
			})

			Convey("bad render gets context", func() {
				_, err := file.RenderL("templ_1", LiteralMap{"${thing}": 10, "${json_key}": 20, "${extra}": nil})
				So(err, ShouldErrLike, "rendering \"templ_1\": param \"${thing}\": type is \"int\", expected \"enum\"")
			})

			Convey("hardcode", func() {
				ret, err := file.RenderL("hardcode", nil)
				So(err, ShouldBeNil)
				So(ret, ShouldEqual, `{"woot": ["sauce"]}`)
			})
		})

	})
}
