// Copyright 2024 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run_benchmark

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBenchmark_TelemetryTestBuilder_TelemetryTests(t *testing.T) {
	expect := func(commit, botConfig, browser, benchmark, story, storyTags, want string) {
		bt, _ := NewBenchmarkTest(commit, botConfig, browser, benchmark, story, storyTags, nil)
		assert.Equal(t, want, reflect.TypeOf(bt).String())
	}

	c, b := "fake-commit", "fake-browser"
	expect(c, "android-pixel6-perf", b, "components_perftests", "", "", "*run_benchmark.telemetryTest")
	expect(c, "android-pixel4_webview-perf", b, "performance_browser_tests", "story", "all", "*run_benchmark.telemetryTest")
}
