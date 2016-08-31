// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package annotation

import (
	"os"

	"github.com/luci/luci-go/common/system/environ"
)

// Execution describes the high-level execution metadata.
type Execution struct {
	Name    string
	Command []string
	Dir     string
	Env     map[string]string
}

// ProbeExecution loads Execution parameters by probing the current runtime
// environment.
func ProbeExecution(argv, env []string, cwd string) *Execution {
	if env == nil {
		env = os.Environ()
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	return probeExecutionImpl(argv, env, cwd)
}

func probeExecutionImpl(argv []string, env []string, cwd string) *Execution {
	e := &Execution{
		Dir: cwd,
	}
	e.Command = make([]string, len(argv))
	copy(e.Command, argv)

	e.Env = make(map[string]string, len(env))
	for k, v := range environ.New(env) {
		_, e.Env[k] = environ.Split(v)
	}

	return e
}
