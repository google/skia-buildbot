package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeDistResourcePath_ResourcesInDist_PathsChanged(t *testing.T) {
	test := func(name, expected, input string) {
		t.Run(name, func(t *testing.T) {
			changed, s := changeDistResourcePath(input)
			require.True(t, changed)
			require.Equal(t, expected, s)
		})
	}
	test("ImageAtRoot", "/img/foo.png", "/dist/foo.png")
	test("ImageInSubDir", "/img/dist/foo.png", "/dist/dist/foo.png")
	test("EmptyString", "/img/", "/dist/")
	test("Unicode", "/img/世界.png", "/dist/世界.png")
}

func TestChangeDistResourcePath_ResourcesNotInDist_PathsUnchanged(t *testing.T) {
	test := func(name, input string) {
		t.Run(name, func(t *testing.T) {
			changed, _ := changeDistResourcePath(input)
			require.False(t, changed)
		})
	}
	test("StartsWithDiff", "/dist_foo.png")
	test("SubdirNamedDist", "/img/dist/foo.png")
	test("URLWithDist", "http://example.com/dist/foo.png")
}
