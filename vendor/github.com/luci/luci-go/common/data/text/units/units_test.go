// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package units

import (
	"testing"
	"time"

	"github.com/maruel/ut"
)

func TestSizeToString(t *testing.T) {
	t.Parallel()
	data := []struct {
		in       int64
		expected string
	}{
		{0, "0b"},
		{1, "1b"},
		{1000, "1000b"},
		{1023, "1023b"},
		{1024, "1.00Kib"},
		{1029, "1.00Kib"},
		{1030, "1.01Kib"},
		{10234, "9.99Kib"},
		{10239, "10.00Kib"},
		{10240, "10.0Kib"},
		{1048575, "1024.0Kib"},
		{1048576, "1.00Mib"},
	}
	for i, line := range data {
		ut.AssertEqualIndex(t, i, line.expected, SizeToString(line.in))
	}
}

func TestRound(t *testing.T) {
	t.Parallel()
	data := []struct {
		in       time.Duration
		round    time.Duration
		expected time.Duration
	}{
		{-time.Second, time.Second, -time.Second},
		{-500 * time.Millisecond, time.Second, -time.Second},
		{-499 * time.Millisecond, time.Second, 0},
		{0, time.Second, 0},
		{499 * time.Millisecond, time.Second, 0},
		{500 * time.Millisecond, time.Second, time.Second},
		{time.Second, time.Second, time.Second},
	}
	for i, line := range data {
		ut.AssertEqualIndex(t, i, line.expected, Round(line.in, line.round))
	}
}
