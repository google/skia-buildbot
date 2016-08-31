// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gaesettings

import (
	"encoding/json"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/filter/count"
	"github.com/luci/gae/filter/dscache"
	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"

	. "github.com/smartystreets/goconvey/convey"
)

func TestWorks(t *testing.T) {
	Convey("Works", t, func() {
		c := memory.Use(context.Background())
		c = dscache.AlwaysFilterRDS(c, nil)
		c, tc := testclock.UseTime(c, time.Unix(1444945245, 0))

		// Record access to memcache. There should be none.
		c, mcOps := count.FilterMC(c)

		s := Storage{}

		// Nothing's there yet.
		bundle, err := s.FetchAllSettings(c)
		So(err, ShouldBeNil)
		So(bundle.Exp, ShouldResemble, clock.Now(c).Add(time.Second))
		So(len(bundle.Values), ShouldEqual, 0)

		conTime, err := s.GetConsistencyTime(c)
		So(conTime.IsZero(), ShouldBeTrue)
		So(err, ShouldBeNil)

		// Produce a bunch of versions.
		tc.Add(time.Minute)
		So(s.UpdateSetting(c, "key", json.RawMessage(`"val1"`), "who1", "why1"), ShouldBeNil)
		tc.Add(time.Minute)
		So(s.UpdateSetting(c, "key", json.RawMessage(`"val2"`), "who2", "why2"), ShouldBeNil)
		tc.Add(time.Minute)
		So(s.UpdateSetting(c, "key", json.RawMessage(`"val3"`), "who3", "why3"), ShouldBeNil)

		bundle, err = s.FetchAllSettings(c)
		So(err, ShouldBeNil)
		So(bundle.Exp, ShouldResemble, clock.Now(c).Add(time.Second))
		So(*bundle.Values["key"], ShouldResemble, json.RawMessage(`"val3"`))

		conTime, err = s.GetConsistencyTime(c)
		So(conTime, ShouldResemble, clock.Now(c).UTC().Add(time.Second))
		So(err, ShouldBeNil)

		// Check all log entities is there.
		ds := datastore.Get(c)
		ds.Testable().CatchupIndexes()
		entities := []settingsEntity{}
		So(ds.GetAll(datastore.NewQuery("gaesettings.SettingsLog"), &entities), ShouldBeNil)
		So(len(entities), ShouldEqual, 2)
		asMap := map[string]settingsEntity{}
		for _, e := range entities {
			So(e.Kind, ShouldEqual, "gaesettings.SettingsLog")
			So(e.Parent.Kind(), ShouldEqual, "gaesettings.Settings")
			// Clear some fields to simplify assert below.
			e.Kind = ""
			e.Parent = nil
			e.When = time.Time{}
			asMap[e.ID] = e
		}
		So(asMap, ShouldResemble, map[string]settingsEntity{
			"1": {
				ID:      "1",
				Version: 1,
				Value:   "{\n  \"key\": \"val1\"\n}",
				Who:     "who1",
				Why:     "why1",
			},
			"2": {
				ID:      "2",
				Version: 2,
				Value:   "{\n  \"key\": \"val2\"\n}",
				Who:     "who2",
				Why:     "why2",
			},
		})

		// Memcache must not be used even if dscache is installed in the context.
		So(mcOps.AddMulti.Total(), ShouldEqual, 0)
		So(mcOps.GetMulti.Total(), ShouldEqual, 0)

		// TODO(iannucci): There's a bug in dscache that causes calls to memcache
		// sets and deletes even if dscache is disabled. This should be switched to
		// 0 when it is fixed (the test will break at that moment).
		So(mcOps.SetMulti.Total(), ShouldEqual, 3)
		So(mcOps.DeleteMulti.Total(), ShouldEqual, 3)
	})
}
