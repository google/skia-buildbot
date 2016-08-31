// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package memlogger

import (
	"bytes"
	"sync"
	"testing"

	"golang.org/x/net/context"

	cv "github.com/smartystreets/goconvey/convey"

	"github.com/luci/luci-go/common/logging"
)

func TestLogger(t *testing.T) {
	cv.Convey("Zero", t, func() {
		var l MemLogger
		l.Debugf("test %s", logging.Debug)
		l.Infof("test %s", logging.Info)

		cv.So(&l, ShouldHaveLog, logging.Debug, "test debug")
		cv.So(&l, ShouldHaveLog, logging.Info, "test info")
	})
	cv.Convey("logger", t, func() {
		c := Use(logging.SetLevel(context.Background(), logging.Debug))
		l := logging.Get(c)
		cv.So(l, cv.ShouldNotBeNil)
		l.Debugf("test %s", logging.Debug)
		l.Infof("test %s", logging.Info)
		l.Warningf("test %s", logging.Warning)
		l.Errorf("test %s", logging.Error)
		l.Errorf("test WAT: %s", logging.Level(9001))
		ml := l.(*MemLogger)

		cv.So(ml, ShouldHaveLog, logging.Debug, "test debug")
		cv.So(ml, ShouldHaveLog, logging.Info, "test info")
		cv.So(ml, ShouldHaveLog, logging.Warning, "test warning")
		cv.So(ml, ShouldHaveLog, logging.Error, "test error")
		cv.So(ml, ShouldHaveLog, logging.Error, "test WAT: unknown")
	})

	cv.Convey("logger context", t, func() {
		c := Use(context.Background())
		l := logging.Get(c)
		cv.So(l, cv.ShouldNotBeNil)
		ml := l.(*MemLogger)

		l.Infof("totally works: %s", "yes")

		cv.So(ml, ShouldHaveLog, logging.Info, "totally works: yes")
		cv.So(ml, ShouldNotHaveLog, logging.Warning, "totally works: yes")
	})

	cv.Convey("field data", t, func() {
		c := Use(context.Background())
		data := map[string]interface{}{
			"trombone": 50,
			"cat":      "amazing",
		}
		c = logging.SetFields(c, logging.NewFields(data))
		l := logging.Get(c)
		ml := l.(*MemLogger)

		l.Infof("Some unsuspecting log")
		msgs := ml.Messages()
		cv.So(msgs[0].Data["trombone"], cv.ShouldEqual, 50)
		cv.So(msgs[0].Data["cat"], cv.ShouldEqual, "amazing")
	})

	cv.Convey("reset", t, func() {
		c := Use(context.Background())
		l := logging.Get(c).(*MemLogger)

		l.Infof("hello")
		cv.So(len(l.Messages()), cv.ShouldEqual, 1)

		l.Reset()
		cv.So(len(l.Messages()), cv.ShouldEqual, 0)

		l.Infof("shweeet")
		cv.So(len(l.Messages()), cv.ShouldEqual, 1)
	})

	cv.Convey("dump", t, func() {
		var l MemLogger
		l.fields = map[string]interface{}{"key": 100}
		l.Debugf("test %s", logging.Debug)
		l.Infof("test %s", logging.Info)

		buf := bytes.Buffer{}
		l.Dump(&buf)

		cv.So(buf.String(), cv.ShouldEqual, `
DUMP LOG:
  debug: test debug: {"key":100}
  info: test info: {"key":100}
`)
	})
}

func TestLoggerAssertion(t *testing.T) {
	t.Parallel()

	cv.Convey("ShouldHaveLog", t, func() {
		cv.Convey("basic", func() {
			m := &MemLogger{
				lock: &sync.Mutex{},
				data: &[]LogEntry{
					{
						Level:     logging.Error,
						Msg:       "HI THAR",
						Data:      map[string]interface{}{"hi": 3},
						CallDepth: 47,
					},
				},
			}

			cv.So(ShouldHaveLog(m, logging.Error, "HI THAR", map[string]interface{}{"hi": 3}), cv.ShouldEqual, "")
			cv.So(ShouldHaveLog(m, logging.Error, "HI THAR", map[string]interface{}{"hi": 4}), cv.ShouldNotEqual, "")
			cv.So(ShouldHaveLog(m, logging.Error, "Hi THAR", map[string]interface{}{"hi": 4}), cv.ShouldNotEqual, "")
			cv.So(ShouldHaveLog(m, logging.Error, "THAR", map[string]interface{}{"hi": 4}), cv.ShouldNotEqual, "")
		})

		cv.Convey("level and message", func() {
			m := &MemLogger{
				lock: &sync.Mutex{},
				data: &[]LogEntry{
					{
						Level: logging.Error,
						Msg:   "HI THAR",
						Data:  map[string]interface{}{"hi": 3},
					},
				},
			}

			cv.So(ShouldHaveLog(m, logging.Error, "HI THAR"), cv.ShouldEqual, "")
		})

		cv.Convey("level only", func() {
			m := &MemLogger{
				lock: &sync.Mutex{},
				data: &[]LogEntry{
					{
						Level:     logging.Error,
						Msg:       "HI THAR",
						Data:      nil,
						CallDepth: 47,
					},
				},
			}

			cv.So(ShouldHaveLog(m, logging.Error, "BYE"), cv.ShouldNotEqual, "")
			cv.So(ShouldHaveLog(m, logging.Error), cv.ShouldEqual, "")
		})

		cv.Convey("bad logger", func() {
			cv.So(ShouldHaveLog(nil), cv.ShouldNotEqual, "")
		})

		cv.Convey("bad level", func() {
			m := &MemLogger{}

			cv.So(ShouldHaveLog(m, "BOO"), cv.ShouldNotEqual, "")
		})

		cv.Convey("bad message", func() {
			m := &MemLogger{}

			cv.So(ShouldHaveLog(m, logging.Error, 48), cv.ShouldNotEqual, "")
		})

		cv.Convey("bad depth", func() {
			m := &MemLogger{}

			cv.So(ShouldHaveLog(m, logging.Error, "HI THAR", "NO BAD"), cv.ShouldNotEqual, "")
		})

		cv.Convey("not found", func() {
			m := &MemLogger{
				lock: &sync.Mutex{},
				data: &[]LogEntry{},
			}

			cv.So(ShouldHaveLog(m, logging.Error, "BYE THAR", 47), cv.ShouldNotEqual, "")
		})

		cv.Convey("need at least one argument", func() {
			m := &MemLogger{}

			cv.So(ShouldHaveLog(m), cv.ShouldNotEqual, "")
		})
	})
}
