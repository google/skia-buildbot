// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"path/filepath"
	"unicode"

	"github.com/luci/luci-go/common/errors"
)

type title string

func (t title) validate() error {
	if len(t) == 0 {
		return errors.New("cannot be empty")
	}

	idx := 0
	for _, r := range t {
		if !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-') {
			return errors.Reason("character at %(pos)d (%(char)c) is not permitted in a title").
				D("pos", idx).D("char", r).Err()
		}
		idx++
	}
	return nil
}

// titleFromConfigPath returns the title of a configuration item identified by
// the specified configuration file.
//
// If the file was not a valid config path, or the title was not valid, an error
// will be returned.
func titleFromConfigPath(path string) (title, error) {
	path = filepath.Base(path)
	if filepath.Ext(path) == configExt {
		t := title(path[:len(path)-len(configExt)])
		if err := t.validate(); err != nil {
			return "", err
		}
		return t, nil
	}
	return "", errors.Reason("missing config extension [" + configExt + "]").Err()
}
