// Package clusters contains the current cluster config json file as an embedded
// string.
package clusters

import (
	_ "embed"
)

//go:embed config.json
var ClusterConfig string
