// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package testsecrets

import (
	"testing"

	"github.com/luci/luci-go/server/secrets"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStore(t *testing.T) {
	Convey("Autogeneration enabled", t, func() {
		store := Store{}

		// Autogenerate one.
		s1, err := store.GetSecret("key1")
		So(err, ShouldBeNil)
		So(s1, ShouldResemble, secrets.Secret{
			Current: secrets.NamedBlob{
				ID:   "secret_1",
				Blob: []byte{0xfa, 0x12, 0xf9, 0x2a, 0xfb, 0xe0, 0xf, 0x85},
			},
		})

		// Getting same one back.
		s2, err := store.GetSecret("key1")
		So(err, ShouldBeNil)
		So(s2, ShouldResemble, secrets.Secret{
			Current: secrets.NamedBlob{
				ID:   "secret_1",
				Blob: []byte{0xfa, 0x12, 0xf9, 0x2a, 0xfb, 0xe0, 0xf, 0x85},
			},
		})
	})

	Convey("Autogeneration disabled", t, func() {
		store := Store{NoAutogenerate: true}
		_, err := store.GetSecret("key1")
		So(err, ShouldEqual, secrets.ErrNoSuchSecret)
	})
}
