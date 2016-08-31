// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package console

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/milo/common/miloerror"
	"github.com/luci/luci-go/server/templates"
)

type Console struct{}

// GetTemplateName returns the template name for console pages.
func (x Console) GetTemplateName(t settings.Theme) string {
	return "console.html"
}

// Render renders the console page.
func (x Console) Render(c context.Context, r *http.Request, p httprouter.Params) (*templates.Args, error) {
	project := p.ByName("project")
	if project == "" {
		return nil, &miloerror.Error{
			Message: "Missing project",
			Code:    http.StatusBadRequest,
		}
	}
	name := p.ByName("name")

	result, err := console(c, project, name)
	if err != nil {
		return nil, err
	}

	// Render into the template
	args := &templates.Args{
		"Console": result,
	}
	return args, nil
}
