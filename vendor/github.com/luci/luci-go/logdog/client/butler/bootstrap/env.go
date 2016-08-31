// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package bootstrap handles Butler-side bootstrapping functionality.
package bootstrap

import (
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/system/environ"
	"github.com/luci/luci-go/logdog/client/butlerlib/bootstrap"
	"github.com/luci/luci-go/logdog/common/types"
)

// Environment is the set of configuration parameters for the bootstrap.
type Environment struct {
	// Project is the project name. If not empty, this will be exported to
	// subprocesses.
	Project config.ProjectName
	// Prefix is the prefix name. If not empty, this will be exported to
	// subprocesses.
	Prefix types.StreamName
	// StreamServerURI is the streamserver URI. If not empty, this will be
	// exported to subprocesses.
	StreamServerURI string
}

// Augment augments the supplied base environment with LogDog Butler bootstrap
// parameters.
func (e *Environment) Augment(base environ.Env) {
	exportIf := func(envKey, v string) {
		if v != "" {
			base.Set(envKey, v)
		}
	}

	exportIf(bootstrap.EnvStreamPrefix, string(e.Prefix))
	exportIf(bootstrap.EnvStreamProject, string(e.Project))
	exportIf(bootstrap.EnvStreamServerPath, e.StreamServerURI)
}
