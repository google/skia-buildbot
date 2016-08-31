// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/sync/parallel"
	"github.com/luci/luci-go/common/tsmon"
	"github.com/luci/luci-go/server/router"
)

// InstallHandlers installs HTTP handlers for tsmon routes.
func InstallHandlers(r *router.Router, base router.MiddlewareChain) {
	r.GET("/internal/cron/ts_mon/housekeeping", base, housekeepingHandler)
}

// housekeepingHandler is an HTTP handler that should be run every minute by
// cron on App Engine.  It assigns task numbers to datastore entries, and runs
// any global metric callbacks.
func housekeepingHandler(c *router.Context) {
	if !info.Get(c.Context).IsDevAppServer() && c.Request.Header.Get("X-Appengine-Cron") != "true" {
		c.Writer.WriteHeader(http.StatusForbidden)
		http.Error(c.Writer, "request not made from cron", http.StatusForbidden)
		return
	}

	if err := assignTaskNumbers(c.Context); err != nil {
		c.Writer.WriteHeader(http.StatusInternalServerError)
	}

	tsmon.GetState(c.Context).RunGlobalCallbacks(c.Context)
}

// assignTaskNumbers does some housekeeping on the datastore entries for App
// Engine instances - assigning unique task numbers to those without ones set,
// and expiring old entities.
func assignTaskNumbers(c context.Context) error {
	c = info.Get(c).MustNamespace(instanceNamespace)
	ds := datastore.Get(c)

	logger := logging.Get(c)
	now := clock.Now(c)
	expiredTime := now.Add(-instanceExpirationTimeout)

	usedTaskNums := map[int]struct{}{}
	var expiredKeys []*datastore.Key
	var unassigned []*instance

	// Query all instances from datastore.
	if err := ds.Run(datastore.NewQuery("Instance"), func(i *instance) {
		if i.TaskNum >= 0 {
			usedTaskNums[i.TaskNum] = struct{}{}
		}
		if i.LastUpdated.Before(expiredTime) {
			expiredKeys = append(expiredKeys, ds.NewKey("Instance", i.ID, 0, nil))
			logger.Debugf("Expiring %s task_num %d, inactive since %s",
				i.ID, i.TaskNum, i.LastUpdated.String())
		} else if i.TaskNum < 0 {
			unassigned = append(unassigned, i)
		}
	}); err != nil {
		logging.WithError(err).Errorf(c, "Failed to get Instance entities from datastore")
		return err
	}

	logger.Debugf("Found %d expired and %d unassigned instances",
		len(expiredKeys), len(unassigned))

	// Assign task numbers to those that don't have one assigned yet.
	nextNum := gapFinder(usedTaskNums)
	for _, i := range unassigned {
		i.TaskNum = nextNum()
		logger.Debugf("Assigned %s task_num %d", i.ID, i.TaskNum)
	}

	// Update all the entities in datastore.
	if err := parallel.FanOutIn(func(gen chan<- func() error) {
		gen <- func() error {
			return ds.Put(unassigned)
		}
		gen <- func() error {
			return ds.Delete(expiredKeys)
		}
	}); err != nil {
		logging.WithError(err).Errorf(c, "Failed to update task numbers")
		return err
	}
	return nil
}

func gapFinder(used map[int]struct{}) func() int {
	next := 0
	return func() int {
		for {
			n := next
			next++
			_, has := used[n]
			if !has {
				return n
			}
		}
	}
}
