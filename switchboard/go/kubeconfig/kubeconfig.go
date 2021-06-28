package kubeconfig

import (
	_ "embed" // Enables go:embed functionality.
)

// Config contains the contents of the kubeconfig file needed to connect to the
// switchboard cluster. See the README.md file for details on how this file was
// generated.
//
//go:embed kubeconfig.yaml
var Config []byte
