// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbot

import (
	"fmt"
	"strings"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/sync/parallel"
	"github.com/luci/luci-go/milo/api/resp"
	"golang.org/x/net/context"
)

// getFullBuilds fetches all of the recent builds from the datastore.
func getFullBuilds(c context.Context, masterName, builderName string, finished bool) ([]*buildbotBuild, error) {
	// TODO(hinoka): Builder specific structs.
	ds := datastore.Get(c)
	q := datastore.NewQuery("buildbotBuild")
	q = q.Eq("finished", finished)
	q = q.Eq("master", masterName)
	q = q.Eq("builder", builderName)
	q = q.Limit(25) // TODO(hinoka): This should be adjustable
	q = q.Order("-number")
	q.Finalize()
	buildbots := make([]*buildbotBuild, 0, 25)
	err := ds.GetAll(q, &buildbots)
	return buildbots, err
}

// GetConsoleBuilds takes commits and builders and returns a matrix of
// resp.ConsoleBuild objects.  The expected format of the result
// is [len(commits)][len(builders)]*resp.ConsoleBuild.  The first level matches
// 1:1 to a commit, and the second level matches 1:1 to a builder.
func GetConsoleBuilds(
	c context.Context, builders []resp.BuilderRef, commits []string) (
	[][]*resp.ConsoleBuild, error) {

	results := make([][]*resp.ConsoleBuild, len(commits))
	for i := range results {
		results[i] = make([]*resp.ConsoleBuild, len(builders))
	}
	// HACK(hinoka): This fetches 25 full builds and then filters them. Replace this
	// with something more reasonable.
	// This is kind of a hack but it's okay for now.
	err := parallel.FanOutIn(func(taskC chan<- func() error) {
		for i, builder := range builders {
			i := i
			builder := builder
			builderComponents := strings.SplitN(builder.Name, "/", 2)
			if len(builderComponents) != 2 {
				taskC <- func() error {
					return fmt.Errorf("%s is an invalid builder name", builder.Name)
				}
				return
			}
			master := builderComponents[0]
			builderName := builderComponents[1]
			taskC <- func() error {
				t1 := clock.Now(c)
				builds, err := getFullBuilds(c, master, builderName, true)
				if err != nil {
					return err
				}
				t2 := clock.Now(c)
				var currentStatus *resp.Status
				for j, commit := range commits {
					for _, build := range builds {
						if build.Sourcestamp.Revision == commit {
							results[j][i] = &resp.ConsoleBuild{
								Link: &resp.Link{
									Label: strings.Join(build.Text, " "),
									URL: fmt.Sprintf(
										"/buildbot/%s/%s/%d", master, builderName, build.Number),
								},
								Status: build.toStatus(),
							}
							currentStatus = &results[j][i].Status
						}
					}
					if currentStatus != nil && results[j][i] == nil {
						results[j][i] = &resp.ConsoleBuild{Status: *currentStatus}
					}
				}
				log.Debugf(c,
					"Builder %s took %s to query, %s to compute.", builderName,
					t2.Sub(t1), clock.Since(c, t2))
				return nil
			}
		}
	})
	return results, err
}
