// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/memlogger"

	. "github.com/smartystreets/goconvey/convey"
)

type exampleSettings struct {
	Greetings string `json:"greetings"`
}

type anotherSettings struct{}

func TestSettings(t *testing.T) {
	Convey("with in-memory settings", t, func() {
		ctx, tc := testclock.UseTime(context.Background(), time.Unix(1444945245, 0))
		ctx = memlogger.Use(ctx)
		log := logging.Get(ctx).(*memlogger.MemLogger)

		settings := New(&MemoryStorage{Expiration: time.Second})
		s := exampleSettings{}

		Convey("settings API works", func() {
			// Nothing is set yet.
			So(settings.Get(ctx, "key", &s), ShouldEqual, ErrNoSettings)

			// Set something.
			So(settings.Set(ctx, "key", &exampleSettings{"hi"}, "who", "why"), ShouldBeNil)

			// Old value (the lack of there of) is still cached.
			So(settings.Get(ctx, "key", &s), ShouldEqual, ErrNoSettings)

			// Non-caching version works.
			So(settings.GetUncached(ctx, "key", &s), ShouldBeNil)
			So(s, ShouldResemble, exampleSettings{"hi"})

			// Advance time to make old value expired.
			tc.Add(2 * time.Second)
			So(settings.Get(ctx, "key", &s), ShouldBeNil)
			So(s, ShouldResemble, exampleSettings{"hi"})

			// Not a pointer.
			So(settings.Get(ctx, "key", s), ShouldEqual, ErrBadType)

			// Not *exampleSettings.
			So(settings.Get(ctx, "key", &anotherSettings{}), ShouldEqual, ErrBadType)
		})

		Convey("SetIfChanged works", func() {
			// Initial value. New change notification.
			So(settings.SetIfChanged(ctx, "key", &exampleSettings{"hi"}, "who", "why"), ShouldBeNil)
			So(len(log.Messages()), ShouldEqual, 1)
			log.Reset()

			// Noop change. No change notification.
			So(settings.SetIfChanged(ctx, "key", &exampleSettings{"hi"}, "who", "why"), ShouldBeNil)
			So(len(log.Messages()), ShouldEqual, 0)

			// Some real change. New change notification.
			So(settings.SetIfChanged(ctx, "key", &exampleSettings{"boo"}, "who", "why"), ShouldBeNil)
			So(len(log.Messages()), ShouldEqual, 1)
		})
	})
}

func TestContext(t *testing.T) {
	Convey("Works", t, func() {
		ctx := context.Background()
		s := exampleSettings{}

		So(Get(ctx, "key", &exampleSettings{}), ShouldEqual, ErrNoSettings)
		So(GetUncached(ctx, "key", &exampleSettings{}), ShouldEqual, ErrNoSettings)
		So(Set(ctx, "key", &exampleSettings{}, "who", "why"), ShouldEqual, ErrNoSettings)
		So(SetIfChanged(ctx, "key", &exampleSettings{}, "who", "why"), ShouldEqual, ErrNoSettings)

		ctx = Use(ctx, New(&MemoryStorage{}))
		So(Set(ctx, "key", &exampleSettings{"hi"}, "who", "why"), ShouldBeNil)
		So(SetIfChanged(ctx, "key", &exampleSettings{"hi"}, "who", "why"), ShouldBeNil)

		So(Get(ctx, "key", &s), ShouldBeNil)
		So(s, ShouldResemble, exampleSettings{"hi"})

		So(GetUncached(ctx, "key", &s), ShouldBeNil)
		So(s, ShouldResemble, exampleSettings{"hi"})
	})
}
