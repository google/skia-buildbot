// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package errors

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAnnotation(t *testing.T) {
	t.Parallel()

	fixSkip := regexp.MustCompile(`skipped \d+ frames`)
	fixNum := regexp.MustCompile(`^#\d+`)

	fix := func(lines Lines) {
		for i, l := range lines {
			switch {
			case strings.HasPrefix(l, "goroutine"):
				l = "GOROUTINE LINE"
			case strings.HasPrefix(l, "... skipped"):
				l = fixSkip.ReplaceAllLiteralString(l, "skipped SOME frames")
			}
			lines[i] = fixNum.ReplaceAllLiteralString(l, "#?")
		}
	}

	Convey("Test annotation struct", t, func() {
		e := (Annotate(New("bad thing")).
			Reason("%(first)d some error: %(second)q").
			D("extra", 8.2, "%.3f").
			D("first", 20, "0x%08x").
			D("second", "stringy").Err())
		ae := e.(*annotatedError)

		Convey("annotation can render itself for public usage", func() {
			So(ae.Error(), ShouldEqual, `20 some error: "stringy": bad thing`)
		})

		Convey("annotation can render itself for internal usage", func() {
			lines := RenderStack(e).ToLines(
				`runtime`, `github.com/jtolds/gls`,
				`github.com/smartystreets/goconvey/convey`)
			fix(lines)

			So(lines, ShouldResemble, Lines{
				`original error: bad thing`,
				``,
				`GOROUTINE LINE`,
				`#? github.com/luci/luci-go/common/errors/annotate_test.go:39 - errors.TestAnnotation.func2()`,
				`  reason: "20 some error: \"stringy\""`,
				`  "extra" = 8.200`,
				`  "first" = 0x00000014`,
				`  "second" = "stringy"`,
				``,
				`... skipped SOME frames in pkg "github.com/smartystreets/goconvey/convey"...`,
				`... skipped SOME frames in pkg "github.com/jtolds/gls"...`,
				`... skipped SOME frames in pkg "github.com/smartystreets/goconvey/convey"...`,
				``,
				`#? github.com/luci/luci-go/common/errors/annotate_test.go:102 - errors.TestAnnotation()`,
				`#? testing/testing.go:473 - testing.tRunner()`,
				`... skipped SOME frames in pkg "runtime"...`,
			})
		})

		Convey("can render whole stack", func() {
			e = Annotate(e).Reason("outer frame %(first)s").D("first", "outer").Err()
			lines := RenderStack(e).ToLines(
				`runtime`, `github.com/jtolds/gls`,
				`github.com/smartystreets/goconvey/convey`)
			fix(lines)

			So(lines, ShouldResemble, Lines{
				`original error: bad thing`,
				``,
				`GOROUTINE LINE`,
				`#? github.com/luci/luci-go/common/errors/annotate_test.go:39 - errors.TestAnnotation.func2()`,
				`  annotation #0:`,
				`    reason: "outer frame outer"`,
				`    "first" = "outer"`,
				`  annotation #1:`,
				`    reason: "20 some error: \"stringy\""`,
				`    "extra" = 8.200`,
				`    "first" = 0x00000014`,
				`    "second" = "stringy"`,
				``,
				`... skipped SOME frames in pkg "github.com/smartystreets/goconvey/convey"...`,
				`... skipped SOME frames in pkg "github.com/jtolds/gls"...`,
				`... skipped SOME frames in pkg "github.com/smartystreets/goconvey/convey"...`,
				``,
				`#? github.com/luci/luci-go/common/errors/annotate_test.go:102 - errors.TestAnnotation()`,
				`#? testing/testing.go:473 - testing.tRunner()`,
				`... skipped SOME frames in pkg "runtime"...`,
			})
		})
	})
}

func TestDataFormat(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		format   string
		expected string
	}{
		{"", ""},
		{"no replacements", `no replacements`},
		{"%(foo)s", `bar`},
		{"%%(foo)s", `%(foo)s`},
		{"%(foo)s|%(foo)s|%(foo)s", `bar|bar|bar`},
		{"|%(foo)s|%(foo)s|%(foo)s|", `|bar|bar|bar|`},
		{"%(missing)s", `MISSING(key="missing")`},
		{"replacing %(foo)q", `replacing "bar"`},
		{"replacing (%(foo)q)", `replacing ("bar")`},
	}

	Convey(`A testing Data object`, t, func() {
		data := Data{
			"foo": {"bar", ""},
		}

		for _, testCase := range testCases {
			Convey(fmt.Sprintf(`Formatting %q yields: %q`, testCase.format, testCase.expected), func() {
				So(data.Format(testCase.format), ShouldEqual, testCase.expected)
			})
		}
	})
}
