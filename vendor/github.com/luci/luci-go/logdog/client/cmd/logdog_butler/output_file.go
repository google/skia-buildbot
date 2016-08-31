// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"errors"

	"github.com/luci/luci-go/common/flag/multiflag"
	"github.com/luci/luci-go/logdog/client/butler/output"
	fileOutput "github.com/luci/luci-go/logdog/client/butler/output/file"
)

func init() {
	registerOutputFactory(&fileOutputFactory{})
}

type fileOutputFactory struct {
	fileOutput.Options
}

func (f *fileOutputFactory) option() multiflag.Option {
	opt := newOutputOption("file", "Debug output that writes stream data to files.", f)

	flags := opt.Flags()
	flags.StringVar(&f.Path, "path", "", "Stream output text protobuf path.")
	flags.BoolVar(&f.Track, "track", false,
		"Track each sent message and dump at the end. This adds CPU/memory overhead.")

	return opt
}

func (f *fileOutputFactory) configOutput(a *application) (output.Output, error) {
	if f.Path == "" {
		return nil, errors.New("missing required output path")
	}
	return f.New(a), nil
}

func (f *fileOutputFactory) scopes() []string { return nil }
