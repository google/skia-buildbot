// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ui

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/server/templates"
)

func invocationPage(c *router.Context) {
	projectID := c.Params.ByName("ProjectID")
	jobID := c.Params.ByName("JobID")
	invID, err := strconv.ParseInt(c.Params.ByName("InvID"), 10, 64)
	if err != nil {
		http.Error(c.Writer, "No such invocation", http.StatusNotFound)
		return
	}

	inv, err := config(c.Context).Engine.GetInvocation(c.Context, projectID+"/"+jobID, invID)
	if err != nil {
		panic(err)
	}
	if inv == nil {
		http.Error(c.Writer, "No such invocation", http.StatusNotFound)
		return
	}

	now := clock.Now(c.Context).UTC()
	templates.MustRender(c.Context, c.Writer, "pages/invocation.html", map[string]interface{}{
		"ProjectID": projectID,
		"JobID":     jobID,
		"Inv":       makeInvocation(projectID, jobID, inv, now),
	})
}

////////////////////////////////////////////////////////////////////////////////
// Actions.

func abortInvocationAction(c *router.Context) {
	handleInvAction(c, func(jobID string, invID int64) error {
		who := auth.CurrentIdentity(c.Context)
		return config(c.Context).Engine.AbortInvocation(c.Context, jobID, invID, who)
	})
}

func handleInvAction(c *router.Context, cb func(string, int64) error) {
	projectID := c.Params.ByName("ProjectID")
	jobID := c.Params.ByName("JobID")
	invID := c.Params.ByName("InvID")
	if !isJobOwner(c.Context, projectID, jobID) {
		http.Error(c.Writer, "Forbidden", 403)
		return
	}
	invIDAsInt, err := strconv.ParseInt(invID, 10, 64)
	if err != nil {
		http.Error(c.Writer, "Bad invocation ID", 400)
		return
	}
	if err := cb(projectID+"/"+jobID, invIDAsInt); err != nil {
		panic(err)
	}
	http.Redirect(c.Writer, c.Request, fmt.Sprintf("/jobs/%s/%s/%s", projectID, jobID, invID), http.StatusFound)
}
