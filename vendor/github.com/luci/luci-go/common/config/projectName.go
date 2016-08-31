// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package config

import (
	"errors"
	"flag"
	"fmt"
)

// ProjectName is a luci-config project name.
//
// A valid project name may only include:
//	- Lowercase letters [a-z]
//	- Numbers [0-9]
//	- Hyphen (-)
//	- Underscore (_)
//
// It also must begin with a letter.
//
// See:
// https://github.com/luci/luci-py/blob/8e594074929871a9761d27e814541bc0d7d84744/appengine/components/components/config/common.py#L41
type ProjectName string

var _ flag.Value = (*ProjectName)(nil)

// Validate returns an error if the supplied ProjectName is not a valid project
// name.
func (p ProjectName) Validate() error {
	if len(p) == 0 {
		return errors.New("cannot have empty name")
	}

	for idx, r := range p {
		switch {
		case r >= 'a' && r <= 'z':

		case (r >= '0' && r <= '9'), r == '-', r == '_':
			if idx == 0 {
				return errors.New("must begin with a letter")
			}

		default:
			return fmt.Errorf("invalid character at %d (%c)", idx, r)
		}
	}

	return nil
}

// String implements flag.Value.
func (p *ProjectName) String() string {
	return string(*p)
}

// Set implements flag.Value.
func (p *ProjectName) Set(v string) error {
	vp := ProjectName(v)
	if err := vp.Validate(); err != nil {
		return err
	}
	*p = vp
	return nil
}
