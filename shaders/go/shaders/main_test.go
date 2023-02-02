package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCorsResourcePath_PngResource_AllowsCors(t *testing.T) {
	test := func(name, resourcePath string) {
		t.Run(name, func(t *testing.T) {
			require.True(t, isResourcePathCorsSafe(resourcePath))
		})
	}
	test("Mandrill", "mandrill.png")
	test("Soccer", "soccer.png")
}

func TestCorsResourcePath_NonPngResource_DisallowsCors(t *testing.T) {
	test := func(name, resourcePath string) {
		t.Run(name, func(t *testing.T) {
			require.False(t, isResourcePathCorsSafe(resourcePath))
		})
	}
	test("CanvasKit", "canvaskit.js")
	test("CanvasKitWASM", "canvaskit.wasm")
}
