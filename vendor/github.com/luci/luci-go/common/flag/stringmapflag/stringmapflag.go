// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package stringmapflag provides a flag.Value that, when parsed, augments a
// map[string]string with the supplied parameter. The parameter is expressed as
// a key[=value] option.
//
// Example
//
// Assuming the flag option, "opt", is bound to a stringmapflag.Value, and the
// following arguments are parsed:
//    -opt foo=bar
//    -opt baz
//
// The resulting map would be equivalent to:
// map[string]string {"foo": "bar", "baz": ""}
package stringmapflag

import (
	"flag"
	"fmt"
	"sort"
	"strings"
)

// Value is a flag.Value implementation that stores arbitrary "key[=value]"
// command-line flags as a string map.
type Value map[string]string

// Assert that Value conforms to the "flag.Value" interface.
var _ = flag.Value(new(Value))

func (v *Value) String() string {
	if len(*v) == 0 {
		return ""
	}

	keys := make([]string, 0, len(*v))
	for k := range *v {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for idx, k := range keys {
		if value := (*v)[k]; value != "" {
			keys[idx] = fmt.Sprintf("%s=%s", k, value)
		}
	}
	return strings.Join(keys, ",")
}

// Set implements flag.Value.
func (v *Value) Set(key string) error {
	key = strings.TrimSpace(key)
	if len(key) == 0 {
		return fmt.Errorf("Cannot specify an empty tag.")
	}

	value := ""
	idx := strings.Index(key, "=")
	switch {
	case idx == -1:
		break

	case idx == 0:
		return fmt.Errorf("Cannot have tag with empty key.")

	case idx > 0:
		key, value = key[:idx], key[idx+1:]
	}

	// Add the entry to our tag map.
	if len(*v) > 0 {
		if _, ok := (*v)[key]; ok {
			return fmt.Errorf("The tag '%s' has already been defined.", key)
		}
	}

	// Record this tag; create a new Value, if necessary.
	if *v == nil {
		*v = make(Value)
	}
	(*v)[key] = value
	return nil
}
