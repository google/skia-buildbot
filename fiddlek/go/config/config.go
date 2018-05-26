package config

import "go.skia.org/infra/go/buildskia"

const (
	// BUILD_TYPE is the type of build we use throughout fiddle.
	BUILD_TYPE = buildskia.RELEASE_BUILD
)

var (
	// GN_FLAGS are the flags to pass to GN.
	GN_FLAGS = []string{"is_debug=false", "skia_use_egl=true", "extra_cflags_cc=[\"-Wno-error\", \"-Wdeprecated-declarations\"]"}

	// EGL_LIB_PATH is the path where the correct libEGL.so can be found.
	EGL_LIB_PATH = "/usr/lib/nvidia-367/"
)
