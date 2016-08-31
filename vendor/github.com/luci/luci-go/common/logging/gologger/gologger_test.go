// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gologger

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"

	"github.com/luci/luci-go/common/logging"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

var (
	ansiRegexp = regexp.MustCompile(`\033\[.+?m`)

	lre = regexp.MustCompile(
		`\[([A-Z])\d+\-\d+\-\d+T\d+:\d+:\d+\.\d+.* \d+ 0 (.+?):\d+\]\s+(.*)`)
)

func normalizeLog(s string) string {
	// Strip ANSI color sequences.
	return ansiRegexp.ReplaceAllString(s, "")
}

func TestGoLogger(t *testing.T) {
	Convey(`A new Go Logger instance`, t, func() {
		// Regex to pull log information from "formatString".

		buf := bytes.Buffer{}
		cfg := LoggerConfig{Out: &buf}
		l := cfg.NewLogger(nil)

		for _, entry := range []struct {
			L logging.Level
			F func(string, ...interface{})
			T string
		}{
			{logging.Debug, l.Debugf, "D"},
			{logging.Info, l.Infof, "I"},
			{logging.Warning, l.Warningf, "W"},
			{logging.Error, l.Errorf, "E"},
		} {
			Convey(fmt.Sprintf("Can log to: %s", entry.L), func() {
				entry.F("Test logging %s", entry.L)
				matches := lre.FindAllStringSubmatch(normalizeLog(buf.String()), -1)
				So(len(matches), ShouldEqual, 1)
				So(len(matches[0]), ShouldEqual, 4)
				So(matches[0][1], ShouldEqual, entry.T)
				So(matches[0][2], ShouldEqual, "gologger_test.go")
				So(matches[0][3], ShouldEqual, fmt.Sprintf("Test logging %s", entry.L))
			})
		}
	})

	Convey(`A Go Logger instance installed in a Context at Info.`, t, func() {
		buf := bytes.Buffer{}
		lc := &LoggerConfig{
			Format: StdConfig.Format,
			Out:    &buf,
		}
		c := logging.SetLevel(lc.Use(context.Background()), logging.Info)

		Convey(`Should log through top-level Context methods.`, func() {
			for _, entry := range []struct {
				L logging.Level
				F func(context.Context, string, ...interface{})
				T string
			}{
				{logging.Info, logging.Infof, "I"},
				{logging.Warning, logging.Warningf, "W"},
				{logging.Error, logging.Errorf, "E"},
			} {
				Convey(fmt.Sprintf("Can log to: %s", entry.L), func() {
					entry.F(c, "Test logging %s", entry.L)
					matches := lre.FindAllStringSubmatch(normalizeLog(buf.String()), -1)
					So(len(matches), ShouldEqual, 1)
					So(len(matches[0]), ShouldEqual, 4)
					So(matches[0][1], ShouldEqual, entry.T)
					So(matches[0][2], ShouldEqual, "gologger_test.go")
					So(matches[0][3], ShouldEqual, fmt.Sprintf("Test logging %s", entry.L))
				})
			}
		})

		Convey(`With Fields installed in the Context`, func() {
			c = logging.SetFields(c, logging.Fields{
				logging.ErrorKey: "An error!",
				"reason":         "test",
			})

			Convey(`Should log Fields.`, func() {
				logging.Infof(c, "Here is a %s", "log")
				matches := lre.FindAllStringSubmatch(normalizeLog(buf.String()), -1)
				So(len(matches), ShouldEqual, 1)
				So(len(matches[0]), ShouldEqual, 4)
				So(matches[0][1], ShouldEqual, "I")
				So(matches[0][2], ShouldEqual, "gologger_test.go")
				So(matches[0][3], ShouldEqual,
					`Here is a log                               {"error":"An error!", "reason":"test"}`)
			})

			Convey(`Should log fields installed immediately`, func() {
				logging.Fields{
					"foo":    "bar",
					"reason": "override",
				}.Infof(c, "Here is another %s", "log")

				matches := lre.FindAllStringSubmatch(normalizeLog(buf.String()), -1)
				So(len(matches), ShouldEqual, 1)
				So(len(matches[0]), ShouldEqual, 4)
				So(matches[0][1], ShouldEqual, "I")
				So(matches[0][2], ShouldEqual, "gologger_test.go")
				So(matches[0][3], ShouldEqual,
					`Here is another log                         {"error":"An error!", "foo":"bar", "reason":"override"}`)
			})

			Convey(`Will not treat format args as format.`, func() {
				logging.Infof(c, "%s", "Here is an %s")
				matches := lre.FindAllStringSubmatch(normalizeLog(buf.String()), -1)
				So(len(matches), ShouldEqual, 1)
				So(len(matches[0]), ShouldEqual, 4)
				So(matches[0][1], ShouldEqual, "I")
				So(matches[0][2], ShouldEqual, "gologger_test.go")
				So(matches[0][3], ShouldEqual,
					`Here is an %s                               {"error":"An error!", "reason":"test"}`)
			})
		})

		Convey(`Will not log to Debug, as it's below level.`, func() {
			logging.Debugf(c, "Hello!")
			So(buf.Len(), ShouldEqual, 0)
		})
	})
}
