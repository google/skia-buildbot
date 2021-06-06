// Package continuous periodically queries the android build api and looks for
// new buildids against a given list of branches and then updates poprepo with
// those new buildids.
package continuous

import (
	"context"
	"math"
	"net/http"
	"time"

	"go.skia.org/infra/android_ingest/go/buildapi"
	"go.skia.org/infra/android_ingest/go/lookup"
	"go.skia.org/infra/android_ingest/go/poprepo"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

// Process periodically queries the android build api and looks for new
// buildids against a given list of branches and then updates poprepo with
// those new buildids.
type Process struct {
	Repo *poprepo.PopRepo

	api    *buildapi.API
	lookup *lookup.Cache
}

// New returns a new *Process.
//
// The lookupCache has entries added as they are found in Start().
//
// If running in production then 'local' should be false.
func New(checkout *git.Checkout, lookupCache *lookup.Cache, client *http.Client, local bool, subdomain string) (*Process, error) {
	repo := poprepo.NewPopRepo(checkout, local, subdomain)
	api, err := buildapi.NewAPI(client)
	if err != nil {
		return nil, err
	}
	return &Process{
		api:    api,
		Repo:   repo,
		lookup: lookupCache,
	}, nil
}

// Last returns the last buildid, its timestamp, and git hash, or a non-nil
// error if and error occurred.
func (c *Process) Last(ctx context.Context) (int64, int64, string, error) {
	return c.Repo.GetLast(ctx)
}

// rationalizeTimestamps Fixes the timestamps so they are all ascending,
// are greater than startTS, and are at least 1 second apart.
func rationalizeTimestamps(builds []buildapi.Build, startTS int64) []buildapi.Build {
	ret := []buildapi.Build{}
	earliest := int64(math.MaxInt64)
	for _, b := range builds {
		if b.TS < earliest {
			earliest = b.TS
		}
	}
	if earliest < startTS {
		earliest = startTS + 1
	}
	for i, b := range builds {
		b.TS = earliest + int64(i)
		ret = append(ret, b)
	}
	return ret
}

// Build represents a single build at the earliest timestamp that it was committed
// to any target. I.e. we find all the timestamps of when the buildid landed in
// all the targets and then take the earliest value.
type Build struct {
	BuildId int64
	TS      int64
}

type BuildSlice []Build

func (p BuildSlice) Len() int           { return len(p) }
func (p BuildSlice) Less(i, j int) bool { return p[i].BuildId < p[j].BuildId }
func (p BuildSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Start a Go routine that does the work.
func (c *Process) Start(ctx context.Context) {
	go func() {
		t := metrics2.NewTimer("repobuilder")
		liveness := metrics2.NewLiveness("last_successful_add")
		failures := metrics2.GetCounter("process_failures", nil)
		for range time.Tick(time.Second) {
			begin := time.Now()
			sklog.Info("= START")
			t.Start()
			buildid, startTS, _, err := c.Repo.GetLast(ctx)
			if err != nil {
				failures.Inc(1)
				sklog.Errorf("Failed to get last buildid: %s", err)
				continue
			}
			sklog.Info("= After GetLast")
			sklog.Info("= Before List")
			mostRecentBuildID, mostRecentBuildTimestamp, err := c.api.GetMostRecentBuildID()
			if err != nil {
				failures.Inc(1)
				sklog.Errorf("Failed to get buildids from api: %s", err)
				continue
			}

			builds := []Build{}
			ts := startTS + 1
			for b := buildid + 1; b <= mostRecentBuildID; b++ {
				builds = append(builds, Build{
					BuildId: b,
					TS:      ts,
				})
				ts += 1
			}
			sklog.Infof("= List Timer: %s", time.Now().Sub(begin).String())
			begin = time.Now()
			sklog.Info("= After List")
			builds = rationalizeTimestamps(builds, startTS)
			for _, b := range builds {
				if err := c.Repo.Add(ctx, b.BuildId, b.TS); err != nil {
					failures.Inc(1)
					sklog.Errorf("Failed to add new buildid to repo: %s", err)
					// Break since we don't want to add anymore buildids until this one
					// lands successfully.
					break
				}
				// Keep lookup.Cache up to date.
				buildid, _, hash, err := c.Repo.GetLast(ctx)
				if err != nil {
					failures.Inc(1)
					sklog.Errorf("Failed to lookup newly added buildid to repo: %s", err)
					break
				}
				c.lookup.Add(buildid, hash)
			}
			sklog.Infof("= Gerrit Timer: %s", time.Now().Sub(begin).String())
			liveness.Reset()
			t.Stop()
		}
	}()
}
