// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package templates

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRender(t *testing.T) {
	assets := map[string]string{
		"includes/base": `
{{define "base"}}
base
{{template "content" .}}
{{end}}
`,
		"pages/page": `
{{define "content"}}
content {{.arg1}} {{.arg2}}
{{end}}
`,
	}

	Convey("AssetsLoader works", t, func(conv C) {
		loaderTest(conv, AssetsLoader(assets))
	})

	Convey("FileSystemLoader works", t, func(conv C) {
		dir, err := ioutil.TempDir("", "luci-go-templates")
		So(err, ShouldBeNil)
		defer os.RemoveAll(dir)

		for k, v := range assets {
			path := filepath.Join(dir, filepath.FromSlash(k))
			So(os.MkdirAll(filepath.Dir(path), 0777), ShouldBeNil)
			So(ioutil.WriteFile(path, []byte(v), 0666), ShouldBeNil)
		}

		loaderTest(conv, FileSystemLoader(dir))
	})
}

func loaderTest(conv C, l Loader) {
	b := Bundle{
		Loader:          l,
		DefaultTemplate: "base",
		DefaultArgs: func(c context.Context) (Args, error) {
			return Args{"arg1": "val1"}, nil
		},
	}
	c := Use(context.Background(), &b)

	tmpl, err := Get(c, "pages/page")
	conv.So(tmpl, ShouldNotBeNil)
	conv.So(err, ShouldBeNil)

	blob, err := Render(c, "pages/page", Args{"arg2": "val2"})
	conv.So(err, ShouldBeNil)
	conv.So(string(blob), ShouldEqual, "\nbase\n\ncontent val1 val2\n\n")

	buf := bytes.Buffer{}
	MustRender(c, &buf, "pages/page", Args{"arg2": "val2"})
	conv.So(buf.String(), ShouldEqual, "\nbase\n\ncontent val1 val2\n\n")
}
