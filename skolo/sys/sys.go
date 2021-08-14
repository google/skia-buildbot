package sys

import (
	"embed" // Enable go:embed.
)

// Sys is a filesystem with all the config files.
//go:embed *.json5
var Sys embed.FS
