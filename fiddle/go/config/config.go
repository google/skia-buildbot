package config

import "go.skia.org/infra/go/buildskia"

const (
	// BUILD_TYPE is the type of build we use throughout fiddle.
	BUILD_TYPE = buildskia.RELEASE_BUILD
)

var (
	// GN_FLAGS are the flags to pass to GN.
	GN_FLAGS = []string{"is_debug=false", "skia_use_mesa=true", "extra_cflags_cc=[\"-Wno-error\"]"}
)
