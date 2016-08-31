// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package environ is a simple environment variable manipulation library. It
// couples the system's environment, representated as a []string of KEY[=VALUE]
// strings, into a key/value map and back.
package environ

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Env is a mutable map of environment variables. It preserves each
// environment variable verbatim (even if invalid).
//
// The keys in an Env are case-sensitive enviornment variable keys. The values
// are the complete enviornment variable value (e.g., "KEY=VALUE"). To get just
// the VALUE component, use Get.
//
// A nil Env map is not valid.
type Env map[string]string

// System returns an Env instance instantiated with the current os.Environ
// values.
func System() Env {
	return New(os.Environ())
}

// New instantiates a new Env instance from the supplied set of environment
// KEY=VALUE strings.
func New(s []string) Env {
	e := make(Env, len(s))
	for _, v := range s {
		k, _ := Split(v)
		e[k] = v
	}
	return e
}

// Get returns the environment value for the supplied key.
//
// If the value is defined, ok will be true and v will be set to its value (note
// that this can be empty if the environment has an empty value). If the value
// is not defined, ok will be false.
func (e Env) Get(k string) (v string, ok bool) {
	if v, ok = e[k]; ok {
		_, v = Split(v)
	}
	return
}

// Set sets the supplied environment key and value.
func (e Env) Set(k, v string) {
	e[k] = Join(k, v)
}

// Sorted returns the contents of the environment, sorted by key.
func (e Env) Sorted() []string {
	r := make([]string, 0, len(e))
	for _, v := range e {
		r = append(r, v)
	}
	sort.Strings(r)
	return r
}

// Clone creates a new Env instance that is identical to, but independent from,
// e.
func (e Env) Clone() Env {
	clone := make(Env, len(e))
	for k, v := range e {
		clone[k] = v
	}
	return clone
}

// Split splits the supplied environment variable value into a key/value pair.
func Split(v string) (key, value string) {
	parts := strings.SplitN(v, "=", 2)
	switch len(parts) {
	case 2:
		value = parts[1]
		fallthrough

	case 1:
		key = parts[0]
	}
	return
}

// Join creates an environment variable definition for the supplied key/value.
func Join(k, v string) string {
	return fmt.Sprintf("%s=%s", k, v)
}
