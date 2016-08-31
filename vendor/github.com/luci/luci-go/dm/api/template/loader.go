// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dmTemplate

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/text/templateproto"
	"github.com/luci/luci-go/common/proto"
	dm "github.com/luci/luci-go/dm/api/service/v1"
)

// LoadFile loads a File by configSet and path.
func LoadFile(c context.Context, project, ref string) (file *File, vers string, err error) {
	cfgSet := "projects/" + project
	if ref != "" {
		cfgSet += "/" + ref
	}
	templateData, err := config.GetConfig(c, cfgSet, "dm/quest_templates.cfg", false)
	if err != nil {
		return
	}
	file = &File{}
	vers = templateData.ContentHash
	err = proto.UnmarshalTextML(templateData.Content, file)
	if err != nil {
		return
	}
	err = file.Normalize()
	return
}

// Render renders the specified template with the given parameters.
func (f *File) Render(spec *templateproto.Specifier) (*dm.Quest_Desc, error) {
	t := f.Template[spec.TemplateName]
	params, err := t.Parameters.Render(spec.Params)
	if err != nil {
		return nil, fmt.Errorf("rendering %q: field distributor parameters: %s", spec.TemplateName, err)
	}
	distribParams, err := t.DistributorParameters.Render(spec.Params)
	if err != nil {
		return nil, fmt.Errorf("rendering %q: field distributor parameters: %s", spec.TemplateName, err)
	}
	return &dm.Quest_Desc{
		DistributorConfigName: t.DistributorConfigName,
		Parameters:            params,
		DistributorParameters: distribParams,
		Meta: t.Meta,
	}, nil
}
