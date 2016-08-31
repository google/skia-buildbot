// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dmTemplate

import "fmt"

// Normalize will normalize all of the Templates in this message, returning an
// error if any are invalid.
func (f *File) Normalize() error {
	for tempName, t := range f.Template {
		if err := t.Normalize(); err != nil {
			return fmt.Errorf("template %q: %s", tempName, err)
		}
	}
	return nil
}

// Normalize will normalize this Template, returning an error if it is invalid.
func (t *File_Template) Normalize() error {
	if t.DistributorConfigName == "" {
		return fmt.Errorf("missing distributor_config_name")
	}
	if err := t.Parameters.Normalize(); err != nil {
		return err
	}
	if err := t.DistributorParameters.Normalize(); err != nil {
		return err
	}
	return nil
}
