package configs

import (
	"embed" // Enable go:embed.
)

// go:embed *.json
var Configs embed.FS
