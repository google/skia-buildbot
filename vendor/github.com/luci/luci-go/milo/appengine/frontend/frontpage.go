// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package frontend

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/luci/luci-go/server/templates"
	"golang.org/x/net/context"

	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/milo/api/resp"
	"github.com/luci/luci-go/milo/appengine/buildbot"
	"github.com/luci/luci-go/milo/appengine/settings"
)

type frontpage struct{}

func (f frontpage) GetTemplateName(t settings.Theme) string {
	return "frontpage.html"
}

func (f frontpage) Render(c context.Context, r *http.Request, p httprouter.Params) (*templates.Args, error) {
	fp := resp.FrontPage{}
	mBuildbot, err := buildbot.GetAllBuilders(c)
	if err != nil {
		log.Errorf(c, "Encountered error while loading buildbot module: %s", err)
	} else {
		fp.Module = append(fp.Module, *mBuildbot)
	}

	return &templates.Args{"frontpage": fp}, nil
}

type testableFrontpage struct{ frontpage }

func (l testableFrontpage) TestData() []settings.TestBundle {
	data := &templates.Args{
		"frontpage": resp.FrontPage{
			Module: []resp.Module{
				{
					Name: "Module 1",
					Masters: []resp.MasterListing{
						{
							Name: "Example master A",
							Builders: []resp.Link{
								{
									Label: "Example builder",
									URL:   "/master1/buildera",
								},
								{
									Label: "Example builder 2",
									URL:   "/master1/builderb",
								},
							},
						},
					},
				},
			},
		},
	}
	return []settings.TestBundle{
		{
			Description: "Basic frontpage",
			Data:        *data,
		},
	}
}
