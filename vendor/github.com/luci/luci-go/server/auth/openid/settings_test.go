// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package openid

import (
	"testing"

	"github.com/luci/luci-go/server/settings"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSettings(t *testing.T) {
	Convey("Works", t, func() {
		c := context.Background()
		c = settings.Use(c, settings.New(&settings.MemoryStorage{}))

		cfg, err := fetchCachedSettings(c)
		So(err, ShouldBeNil)
		So(cfg, ShouldResemble, &Settings{})
	})
}
