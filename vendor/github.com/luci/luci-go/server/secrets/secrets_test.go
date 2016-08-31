// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package secrets

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestClone(t *testing.T) {
	Convey("Clone works", t, func() {
		s1 := Secret{
			Current: NamedBlob{"id1", []byte{1, 2, 3}},
			Previous: []NamedBlob{
				{"id2", []byte{4, 5, 6}},
				{"id3", []byte{7, 8, 9}},
			},
		}
		s2 := s1.Clone()
		So(s1, ShouldResemble, s2)
		So(s1.Current.Blob, ShouldNotEqual, s2.Current.Blob)
		So(s1.Previous[0].Blob, ShouldNotEqual, s2.Previous[0].Blob)
	})
}

func TestBlobs(t *testing.T) {
	Convey("Blobs works", t, func() {
		s := Secret{
			Current: NamedBlob{"id1", nil},
			Previous: []NamedBlob{
				{"id2", nil},
				{"id3", nil},
			},
		}
		So(s.Blobs(), ShouldResemble, []NamedBlob{
			{"id1", nil},
			{"id2", nil},
			{"id3", nil},
		})
	})
}

func TestContext(t *testing.T) {
	Convey("Works", t, func() {
		c := Set(context.Background(), StaticStore{
			"key": Secret{Current: NamedBlob{ID: "secret"}},
		})
		s, err := GetSecret(c, "key")
		So(err, ShouldBeNil)
		So(s.Current.ID, ShouldEqual, "secret")

		_, err = GetSecret(c, "missing")
		So(err, ShouldEqual, ErrNoSuchSecret)

		// For code coverage.
		c = Set(c, nil)
		So(Get(c), ShouldBeNil)
		_, err = GetSecret(c, "key")
		So(err, ShouldEqual, ErrNoStoreConfigured)
	})
}
