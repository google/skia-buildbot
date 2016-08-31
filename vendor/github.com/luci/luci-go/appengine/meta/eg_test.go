// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package meta

import (
	"errors"
	"testing"

	"github.com/luci/gae/filter/featureBreaker"
	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestGetEntityGroupVersion(t *testing.T) {
	t.Parallel()

	Convey("GetEntityGroupVersion", t, func() {
		c := memory.Use(context.Background())
		c, fb := featureBreaker.FilterRDS(c, errors.New("INTERNAL_ERROR"))
		ds := datastore.Get(c)

		pm := datastore.PropertyMap{
			"$key": {datastore.MkPropertyNI(ds.MakeKey("A", ""))},
			"Val":  {datastore.MkProperty(10)},
		}
		So(ds.Put(pm), ShouldBeNil)
		aKey := ds.KeyForObj(pm)
		So(aKey, ShouldNotBeNil)

		v, err := GetEntityGroupVersion(c, aKey)
		So(err, ShouldBeNil)
		So(v, ShouldEqual, 1)

		So(ds.Delete(aKey), ShouldBeNil)

		v, err = GetEntityGroupVersion(c, ds.NewKey("madeUp", "thing", 0, aKey))
		So(err, ShouldBeNil)
		So(v, ShouldEqual, 2)

		v, err = GetEntityGroupVersion(c, ds.NewKey("madeUp", "thing", 0, nil))
		So(err, ShouldBeNil)
		So(v, ShouldEqual, 0)

		fb.BreakFeatures(nil, "GetMulti")

		v, err = GetEntityGroupVersion(c, aKey)
		So(err.Error(), ShouldContainSubstring, "INTERNAL_ERROR")
	})
}
