package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestExtractPathAndVersionFromJSONRoute_VersionedPath_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(jsonRoute, expectedPath string, expectedVersion int) {
		path, version := extractPathAndVersionFromJSONRoute(jsonRoute)
		require.Equal(t, expectedPath, path)
		require.Equal(t, expectedVersion, version)
	}

	test("/json/v1/foo", "/foo", 1)
	test("/json/v2/foo/", "/foo/", 2)
	test("/json/v3/foo/bar", "/foo/bar", 3)
	test("/json/v4/foo/bar/", "/foo/bar/", 4)
	test("/json/v10/foo/bar/{baz}", "/foo/bar/{baz}", 10)
}

func TestExtractPathAndVersionFromJSONRoute_UnversionedPath_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(jsonRoute, expectedPath string) {
		path, version := extractPathAndVersionFromJSONRoute(jsonRoute)
		require.Equal(t, expectedPath, path)
		require.Equal(t, 0, version)
	}

	test("/json/v", "/v") // Edge case.
	test("/json/foo", "/foo")
	test("/json/foo/", "/foo/")
	test("/json/foo/bar", "/foo/bar")
	test("/json/foo/bar/", "/foo/bar/")
	test("/json/foo/bar/{baz}", "/foo/bar/{baz}")
}

func TestExtractPathAndVersionFromJSONRoute_InvalidPath_Panics(t *testing.T) {
	unittest.SmallTest(t)

	test := func(jsonRoute string) {
		require.Panics(t, func() {
			extractPathAndVersionFromJSONRoute(jsonRoute)
		}, jsonRoute)
	}

	// Interesting edge cases.
	test("/json")
	test("/json/")
	test("/json/v1")
	test("/json/v1/")
	test("/json/v0/myrpc") // Using version 0 explicitly is disallowed.

	// Other invalid routes.
	test("")
	test("/")
	test("/foo")
	test("/foo/")
	test("/foo/bar")
}
