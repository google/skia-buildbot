// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/luci/gae/service/datastore"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/milo/api/resp"
	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/milo/common/miloerror"
	"golang.org/x/net/context"
)

// builderRef is used for keying specific builds in a master json.
type builderRef struct {
	builder  string
	buildNum int
}

// buildMap contains all of the current build within a master json.  We use this
// because buildbot returns all current builds as within the slaves portion, whereas
// it's eaiser to map thenm by builders instead.
type buildMap map[builderRef]*buildbotBuild

// createRunningBuildMap extracts all of the running builds in a master json
// from the various slaves and dumps it into a map for easy reference.
func createRunningBuildMap(master *buildbotMaster) buildMap {
	result := buildMap{}
	for _, slave := range master.Slaves {
		for _, build := range slave.Runningbuilds {
			result[builderRef{build.Buildername, build.Number}] = &build
		}
	}
	return result
}

func getBuildSummary(b *buildbotBuild) *resp.BuildSummary {
	started, finished, duration := parseTimes(b.Times)
	return &resp.BuildSummary{
		Link: &resp.Link{
			URL:   fmt.Sprintf("%d", b.Number),
			Label: fmt.Sprintf("#%d", b.Number),
		},
		Status: b.toStatus(),
		ExecutionTime: resp.Interval{
			Started:  started,
			Finished: finished,
			Duration: duration,
		},
		Text:     b.Text,
		Blame:    blame(b),
		Revision: b.Sourcestamp.Revision,
	}
}

// getBuilds fetches all of the recent builds from the datastore.  Note that
// getBuilds() does not perform ACL checks.
func getBuilds(c context.Context, masterName, builderName string, finished bool) ([]*resp.BuildSummary, error) {
	// TODO(hinoka): Builder specific structs.
	result := []*resp.BuildSummary{}
	ds := datastore.Get(c)
	q := datastore.NewQuery("buildbotBuild")
	q = q.Eq("finished", finished)
	q = q.Eq("master", masterName)
	q = q.Eq("builder", builderName)
	q = q.Limit(25) // TODO(hinoka): This should be adjustable
	q = q.Order("-number")
	buildbots := []*buildbotBuild{}
	err := ds.GetAll(q, &buildbots)
	if err != nil {
		return nil, err
	}
	for _, b := range buildbots {
		result = append(result, getBuildSummary(b))
	}
	return result, nil
}

// getCurrentBuild extracts a build from a map of current builds, and translates
// it into the milo version of the build.
func getCurrentBuild(c context.Context, bMap buildMap, builder string, buildNum int) *resp.BuildSummary {
	b, ok := bMap[builderRef{builder, buildNum}]
	if !ok {
		logging.Warningf(c, "Could not find %s/%d in builder map:\n %s", builder, buildNum, bMap)
		return nil
	}
	return getBuildSummary(b)
}

// getCurrentBuilds extracts the list of all the current builds from a master json
// from the slaves' runningBuilds portion.
func getCurrentBuilds(c context.Context, master *buildbotMaster, builderName string) []*resp.BuildSummary {
	b := master.Builders[builderName]
	results := []*resp.BuildSummary{}
	bMap := createRunningBuildMap(master)
	for _, bn := range b.Currentbuilds {
		cb := getCurrentBuild(c, bMap, builderName, bn)
		if cb != nil {
			results = append(results, cb)
		}
	}
	return results
}

var errMasterNotFound = miloerror.Error{
	Message: "Master not found",
	Code:    http.StatusNotFound,
}

// builderImpl is the implementation for getting a milo builder page from buildbot.
// This gets:
// * Current Builds from querying the master json from the datastore.
// * Recent Builds from a cron job that backfills the recent builds.
func builderImpl(c context.Context, masterName, builderName string) (*resp.Builder, error) {
	result := &resp.Builder{}
	master, internal, t, err := getMasterJSON(c, masterName)
	if internal {
		allowed, err := settings.IsAllowedInternal(c)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, errMasterNotFound
		}
	}
	if err != nil {
		return nil, fmt.Errorf("Cannot find master %s\n%s", masterName, err.Error())
	}
	if clock.Now(c).Sub(t) > 2*time.Minute {
		warning := fmt.Sprintf(
			"WARNING: Master data is stale (last updated %s)", t)
		logging.Warningf(c, warning)
		result.Warning = warning
	}

	s, _ := json.Marshal(master)
	logging.Debugf(c, "Master: %s", s)

	_, ok := master.Builders[builderName]
	if !ok {
		// This long block is just to return a good error message when an invalid
		// buildbot builder is specified.
		keys := make([]string, 0, len(master.Builders))
		for k := range master.Builders {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		avail := strings.Join(keys, "\n")
		return nil, fmt.Errorf(
			"Cannot find builder %s in master %s.\nAvailable builders: \n%s",
			builderName, masterName, avail)
	}

	recentBuilds, err := getBuilds(c, masterName, builderName, true)
	if err != nil {
		return nil, err // Or maybe not?
	}
	currentBuilds := getCurrentBuilds(c, master, builderName)
	fmt.Fprintf(os.Stderr, "Number of current builds: %d\n", len(currentBuilds))
	result.CurrentBuilds = currentBuilds
	for _, fb := range recentBuilds {
		// Yes recent builds is synonymous with finished builds.
		// TODO(hinoka): Implement limits.
		if fb != nil {
			result.FinishedBuilds = append(result.FinishedBuilds, fb)
		}
	}
	return result, nil
}
