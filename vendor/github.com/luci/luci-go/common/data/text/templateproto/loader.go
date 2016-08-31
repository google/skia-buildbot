// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package templateproto

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/proto"
)

// LoadFile loads a File from config service by configSet and path.
//
// Expects config.Interface to be in the context already.
func LoadFile(c context.Context, configSet, path string) (file *File, vers string, err error) {
	templateData, err := config.GetConfig(c, configSet, path, false)
	if err != nil {
		return
	}
	file = &File{}
	vers = templateData.ContentHash
	if err = proto.UnmarshalTextML(templateData.Content, file); err != nil {
		return
	}
	err = file.Normalize()
	return
}

// Render renders the specified template with the given parameters.
func (f *File) Render(spec *Specifier) (string, error) {
	ret, err := f.Template[spec.TemplateName].Render(spec.Params)
	if err != nil {
		err = fmt.Errorf("rendering %q: %s", spec.TemplateName, err)
	}
	return ret, err
}

// RenderL renders a specified template with go literal arguments.
func (f *File) RenderL(templName string, params LiteralMap) (ret string, err error) {
	spec := &Specifier{TemplateName: templName}
	spec.Params, err = params.Convert()
	if err != nil {
		return
	}
	return f.Render(spec)
}
