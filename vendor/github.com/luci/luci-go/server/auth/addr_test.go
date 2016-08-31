// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"net"
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestParseRemoteIP(t *testing.T) {
	Convey(`Test suites`, t, func() {
		for _, tc := range []struct {
			v   string
			exp string
			err string
		}{
			{"", net.IPv6loopback.String(), ""},
			{"1.2.3.4", "1.2.3.4", ""},
			{"1.2.3.4:1337", "1.2.3.4", ""},
			{"1::1", "1::1", ""},
			{"[1::1]", "1::1", ""},
			{"[1::1]:1337", "1::1", ""},
			{"[abc:abc:abc:abc:abc:abc:abc:abc]:80", "abc:abc:abc:abc:abc:abc:abc:abc", ""},
			{"[1.2.3.4]:1337", "1.2.3.4", ""},
			{"[1::1:1337", "", "missing closing brace"},
			{"[1.2.3.4", "", "missing closing brace"},
			{"1.3.4:1337", "", "don't know how to parse"},
			{"1:3:5:9:1337", "", "don't know how to parse"},
		} {
			if tc.err == "" {
				Convey(fmt.Sprintf(`Successfully parses %q into %q.`, tc.v, tc.exp), func() {
					ip, err := parseRemoteIP(tc.v)
					So(err, ShouldBeNil)
					So(ip, ShouldResemble, net.ParseIP(tc.exp))
				})
			} else {
				Convey(fmt.Sprintf(`Fails to parse %q with %q.`, tc.v, tc.err), func() {
					_, err := parseRemoteIP(tc.v)
					So(err, ShouldErrLike, tc.err)
				})
			}
		}
	})
}
