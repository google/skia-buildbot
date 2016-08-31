// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ui

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/luci/gae/service/memcache"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/server/templates"
)

func jobPage(ctx *router.Context) {
	c, w, r, p := ctx.Context, ctx.Writer, ctx.Request, ctx.Params

	projectID := p.ByName("ProjectID")
	jobID := p.ByName("JobID")
	cursor := r.URL.Query().Get("c")

	// Grab the job from the datastore.
	job, err := config(c).Engine.GetCronJob(c, projectID+"/"+jobID)
	if err != nil {
		panic(err)
	}
	if job == nil {
		http.Error(w, "No such job", http.StatusNotFound)
		return
	}

	// Grab latest invocations from the datastore.
	invs, nextCursor, err := config(c).Engine.ListInvocations(c, job.JobID, 50, cursor)
	if err != nil {
		panic(err)
	}

	// memcacheKey hashes cursor to reduce its length, since full cursor doesn't
	// fit into memcache key length limits. Use 'v2' scheme for this ('v1' was
	// used before hashing was added).
	memcacheKey := func(cursor string) string {
		blob := sha1.Sum([]byte(job.JobID + ":" + cursor))
		encoded := base64.StdEncoding.EncodeToString(blob[:])
		return "v2:cursors:list_invocations:" + encoded
	}

	// Cheesy way of implementing bidirectional pagination with forward-only
	// datastore cursors: store mapping from a page cursor to a previous page
	// cursor in the memcache.
	mc := memcache.Get(c)
	prevCursor := ""
	if cursor != "" {
		if itm, err := mc.Get(memcacheKey(cursor)); err == nil {
			prevCursor = string(itm.Value())
		}
	}
	if nextCursor != "" {
		itm := mc.NewItem(memcacheKey(nextCursor))
		if cursor == "" {
			itm.SetValue([]byte("NULL"))
		} else {
			itm.SetValue([]byte(cursor))
		}
		itm.SetExpiration(24 * time.Hour)
		mc.Set(itm)
	}

	now := clock.Now(c).UTC()
	templates.MustRender(c, w, "pages/job.html", map[string]interface{}{
		"Job":         makeCronJob(job, now),
		"Invocations": convertToInvocations(job.ProjectID, job.JobID, invs, now),
		"PrevCursor":  prevCursor,
		"NextCursor":  nextCursor,
	})
}

////////////////////////////////////////////////////////////////////////////////
// Actions.

func runJobAction(ctx *router.Context) {
	c, w, r, p := ctx.Context, ctx.Writer, ctx.Request, ctx.Params

	projectID := p.ByName("ProjectID")
	jobID := p.ByName("JobID")
	if !isJobOwner(c, projectID, jobID) {
		http.Error(w, "Forbidden", 403)
		return
	}

	// genericReply renders "we did something (or we failed to do something)"
	// page, shown on error or if invocation is starting for too long.
	genericReply := func(err error) {
		templates.MustRender(c, w, "pages/run_job_result.html", map[string]interface{}{
			"ProjectID": projectID,
			"JobID":     jobID,
			"Error":     err,
		})
	}

	// Enqueue new invocation request, and wait for corresponding invocation to
	// appear. Give up if task queue or datastore indexes are lagging too much.
	e := config(c).Engine
	fullJobID := projectID + "/" + jobID
	invNonce, err := e.TriggerInvocation(c, fullJobID, auth.CurrentIdentity(c))
	if err != nil {
		genericReply(err)
		return
	}

	invID := int64(0)
	deadline := clock.Now(c).Add(10 * time.Second)
	for invID == 0 && deadline.Sub(clock.Now(c)) > 0 {
		// Asking for invocation immediately after triggering it never works,
		// so sleep a bit first.
		if tr := clock.Sleep(c, 600*time.Millisecond); tr.Incomplete() {
			// The Context was canceled before the Sleep completed. Terminate the
			// loop.
			break
		}
		// Find most recent invocation with requested nonce. Ignore errors here,
		// since GetInvocationsByNonce can return only transient ones.
		invs, _ := e.GetInvocationsByNonce(c, invNonce)
		bestTS := time.Time{}
		for _, inv := range invs {
			if inv.JobKey.StringID() == fullJobID && inv.Started.Sub(bestTS) > 0 {
				invID = inv.ID
				bestTS = inv.Started
			}
		}
	}

	if invID != 0 {
		http.Redirect(w, r, fmt.Sprintf("/jobs/%s/%s/%d", projectID, jobID, invID), http.StatusFound)
	} else {
		genericReply(nil) // deadline
	}
}

func pauseJobAction(c *router.Context) {
	handleJobAction(c, func(jobID string) error {
		who := auth.CurrentIdentity(c.Context)
		return config(c.Context).Engine.PauseJob(c.Context, jobID, who)
	})
}

func resumeJobAction(c *router.Context) {
	handleJobAction(c, func(jobID string) error {
		who := auth.CurrentIdentity(c.Context)
		return config(c.Context).Engine.ResumeJob(c.Context, jobID, who)
	})
}

func abortJobAction(c *router.Context) {
	handleJobAction(c, func(jobID string) error {
		who := auth.CurrentIdentity(c.Context)
		return config(c.Context).Engine.AbortJob(c.Context, jobID, who)
	})
}

func handleJobAction(c *router.Context, cb func(string) error) {
	projectID := c.Params.ByName("ProjectID")
	jobID := c.Params.ByName("JobID")
	if !isJobOwner(c.Context, projectID, jobID) {
		http.Error(c.Writer, "Forbidden", 403)
		return
	}
	if err := cb(projectID + "/" + jobID); err != nil {
		panic(err)
	}
	http.Redirect(c.Writer, c.Request, fmt.Sprintf("/jobs/%s/%s", projectID, jobID), http.StatusFound)
}
