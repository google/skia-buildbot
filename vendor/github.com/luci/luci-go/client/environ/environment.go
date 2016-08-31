// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package environ

import (
	"os"
	"strings"
)

// Environment is a key/value mapping of environment variables.
type Environment map[string]string

// Load loads an Environment from a series of KEY or KEY=VALUE pairs.
func Load(env []string) Environment {
	if len(env) == 0 {
		return nil
	}

	environ := make(Environment, len(env))
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts[0]) > 0 {
			switch len(parts) {
			case 1:
				environ[parts[0]] = ""
			case 2:
				environ[parts[0]] = parts[1]
			}
		}
	}
	return environ
}

// Get returns the current Environment from os.Environ.
func Get() Environment {
	return Load(os.Environ())
}
