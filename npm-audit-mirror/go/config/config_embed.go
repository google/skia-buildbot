package config

import (
	_ "embed"
)

//go:embed config.json
var NpmAuditMirrorConfig string

//go:embed verdaccio-config.tmpl
var VerdaccioConfigTemplate string
