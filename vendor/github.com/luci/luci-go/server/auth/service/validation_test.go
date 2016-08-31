// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package service

import (
	"testing"

	"github.com/luci/luci-go/server/auth/service/protocol"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestValidateAuthDB(t *testing.T) {
	Convey("validateAuthDB works", t, func() {
		So(validateAuthDB(&protocol.AuthDB{}), ShouldBeNil)
		So(validateAuthDB(&protocol.AuthDB{
			Groups: []*protocol.AuthGroup{
				{Name: strPtr("group")},
			},
			IpWhitelists: []*protocol.AuthIPWhitelist{
				{Name: strPtr("IP whitelist")},
			},
		}), ShouldBeNil)
	})

	Convey("validateAuthDB bad group", t, func() {
		So(validateAuthDB(&protocol.AuthDB{
			Groups: []*protocol.AuthGroup{
				{
					Name:    strPtr("group"),
					Members: []string{"bad identity"},
				},
			},
		}), ShouldErrLike, "invalid identity")
	})

	Convey("validateAuthDB bad IP whitelist", t, func() {
		So(validateAuthDB(&protocol.AuthDB{
			IpWhitelists: []*protocol.AuthIPWhitelist{
				{
					Name:    strPtr("IP whitelist"),
					Subnets: []string{"not a subnet"},
				},
			},
		}), ShouldErrLike, "bad IP whitlist")
	})
}

func TestValidateAuthGroup(t *testing.T) {
	Convey("validateAuthGroup works", t, func() {
		groups := map[string]*protocol.AuthGroup{
			"group1": {
				Name:    strPtr("group1"),
				Members: []string{"user:abc@example.com"},
				Globs:   []string{"service:*"},
				Nested:  []string{"group2"},
			},
			"group2": {
				Name: strPtr("group2"),
			},
		}
		So(validateAuthGroup("group1", groups), ShouldBeNil)
	})

	Convey("validateAuthGroup bad identity", t, func() {
		groups := map[string]*protocol.AuthGroup{
			"group1": {
				Name:    strPtr("group1"),
				Members: []string{"blah"},
			},
		}
		So(validateAuthGroup("group1", groups), ShouldErrLike, "invalid identity")
	})

	Convey("validateAuthGroup bad glob", t, func() {
		groups := map[string]*protocol.AuthGroup{
			"group1": {
				Name:  strPtr("group1"),
				Globs: []string{"blah"},
			},
		}
		So(validateAuthGroup("group1", groups), ShouldErrLike, "invalid glob")
	})

	Convey("validateAuthGroup missing nested group", t, func() {
		groups := map[string]*protocol.AuthGroup{
			"group1": {
				Name:   strPtr("group1"),
				Nested: []string{"missing"},
			},
		}
		So(validateAuthGroup("group1", groups), ShouldErrLike, "unknown nested group")
	})

	Convey("validateAuthGroup dependency cycle", t, func() {
		groups := map[string]*protocol.AuthGroup{
			"group1": {
				Name:   strPtr("group1"),
				Nested: []string{"group1"},
			},
		}
		So(validateAuthGroup("group1", groups), ShouldErrLike, "dependency cycle found")
	})
}

type groupGraph map[string][]string

func TestFindGroupCycle(t *testing.T) {
	call := func(groups groupGraph) []string {
		asProto := make(map[string]*protocol.AuthGroup)
		for k, v := range groups {
			asProto[k] = &protocol.AuthGroup{
				Name:   strPtr(k),
				Nested: v,
			}
		}
		return findGroupCycle("start", asProto)
	}

	Convey("Empty", t, func() {
		So(call(groupGraph{"start": nil}), ShouldResemble, []string{})
	})

	Convey("No cycles", t, func() {
		So(call(groupGraph{
			"start": []string{"A"},
			"A":     []string{"B"},
			"B":     []string{"C"},
		}), ShouldResemble, []string{})
	})

	Convey("Self reference", t, func() {
		So(call(groupGraph{
			"start": []string{"start"},
		}), ShouldResemble, []string{"start"})
	})

	Convey("Simple cycle", t, func() {
		So(call(groupGraph{
			"start": []string{"A"},
			"A":     []string{"start"},
		}), ShouldResemble, []string{"start", "A"})
	})

	Convey("Long cycle", t, func() {
		So(call(groupGraph{
			"start": []string{"A"},
			"A":     []string{"B"},
			"B":     []string{"C"},
			"C":     []string{"start"},
		}), ShouldResemble, []string{"start", "A", "B", "C"})
	})

	Convey("Diamond no cycles", t, func() {
		So(call(groupGraph{
			"start": []string{"A1", "A2"},
			"A1":    []string{"B"},
			"A2":    []string{"B"},
			"B":     nil,
		}), ShouldResemble, []string{})
	})

	Convey("Diamond with cycles", t, func() {
		So(call(groupGraph{
			"start": []string{"A1", "A2"},
			"A1":    []string{"B"},
			"A2":    []string{"B"},
			"B":     []string{"start"},
		}), ShouldResemble, []string{"start", "A1", "B"})
	})
}

///

func strPtr(s string) *string {
	return &s
}
