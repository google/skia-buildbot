package config

import (
	"embed" // Enable go:embed.
)

// Configs is a filesystem with all the config files.
//go:embed *.json
var Configs embed.FS
