// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package teelogger

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/memlogger"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

var (
	ansiRegexp = regexp.MustCompile(`\033\[.+?m`)

	lre = regexp.MustCompile(
		`\[P\d+ \d+:\d+:\d+\.\d+.* (.+?):\d+ ([A-Z]+) \d+\]\s+(.*)`)
)

func normalizeLog(s string) string {
	// Strip ANSI color sequences.
	return ansiRegexp.ReplaceAllString(s, "")
}

func TestTeeLogger(t *testing.T) {
	Convey(`A new TeeLogger instance`, t, func() {
		l1 := logging.Get(
			memlogger.Use(context.Background())).(*memlogger.MemLogger)
		l2 := logging.Get(
			memlogger.Use(context.Background())).(*memlogger.MemLogger)
		l3 := logging.Get(
			memlogger.Use(context.Background())).(*memlogger.MemLogger)

		teeLog := teeImpl{nil, []logging.Logger{l1, l2, l3}}

		for _, entry := range []struct {
			L logging.Level
			F func(string, ...interface{})
			T string
		}{
			{logging.Debug, teeLog.Debugf, "DEBU"},
			{logging.Info, teeLog.Infof, "INFO"},
			{logging.Warning, teeLog.Warningf, "WARN"},
			{logging.Error, teeLog.Errorf, "ERRO"},
		} {
			Convey(fmt.Sprintf("Can log to %s", entry.L), func() {
				entry.F("%s", entry.T)
				for _, logger := range []*memlogger.MemLogger{l1, l2, l3} {
					So(len(logger.Messages()), ShouldEqual, 1)
					msg := logger.Get(entry.L, entry.T, map[string]interface{}(nil))
					So(msg, ShouldNotBeNil)
					So(msg.CallDepth, ShouldEqual, 3)
				}
			})
		}
		Convey("Uses context logger", func() {
			ctx := memlogger.Use(context.Background())
			logger := logging.Get(ctx).(*memlogger.MemLogger)

			tee := Use(ctx)
			logging.Get(tee).Infof("Testing 1 2")

			messages := logger.Messages()

			// Make sure context logger doesn't get called
			So(len(messages), ShouldEqual, 1)
			msg := messages[0]
			So(msg.CallDepth, ShouldEqual, 3)
			So(msg.Msg, ShouldEqual, "Testing 1 2")
		})
	})
}
