// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package internal

import (
	"strings"
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/cipd/client/cipd/common"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTagCacheWorks(t *testing.T) {
	ctx := context.Background()

	Convey("Works", t, func(c C) {
		tc := TagCache{}
		So(tc.ResolveTag(ctx, "pkg", "tag:1"), ShouldResemble, common.Pin{})
		So(tc.Dirty(), ShouldBeFalse)

		// Add new.
		tc.AddTag(ctx, common.Pin{
			PackageName: "pkg",
			InstanceID:  strings.Repeat("a", 40),
		}, "tag:1")
		So(tc.Dirty(), ShouldBeTrue)
		So(tc.ResolveTag(ctx, "pkg", "tag:1"), ShouldResemble, common.Pin{
			PackageName: "pkg",
			InstanceID:  strings.Repeat("a", 40),
		})

		// Replace existing.
		tc.AddTag(ctx, common.Pin{
			PackageName: "pkg",
			InstanceID:  strings.Repeat("b", 40),
		}, "tag:1")
		So(tc.Dirty(), ShouldBeTrue)
		So(tc.ResolveTag(ctx, "pkg", "tag:1"), ShouldResemble, common.Pin{
			PackageName: "pkg",
			InstanceID:  strings.Repeat("b", 40),
		})

		// Save\load.
		blob, err := tc.Save(ctx)
		So(err, ShouldBeNil)
		So(tc.Dirty(), ShouldBeFalse)
		another := TagCache{}
		So(another.Load(ctx, blob), ShouldBeNil)
		So(another.ResolveTag(ctx, "pkg", "tag:1"), ShouldResemble, common.Pin{
			PackageName: "pkg",
			InstanceID:  strings.Repeat("b", 40),
		})
	})
}
