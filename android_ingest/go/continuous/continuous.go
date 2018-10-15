// Package continuous periodically queries the android build api and looks for
// new buildids against a given branch and then updates poprepo with those new
// buildids.
package continuous

import (
	"context"
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
// buildids against a given branch and then updates poprepo with those new
// buildids.
type Process struct {
	Repo *poprepo.PopRepo

	api    *buildapi.API
	branch string
	lookup *lookup.Cache
}

// New returns a new *Process.
//
// The lookupCache has entries added as they are found in Start().
//
// If running in production then 'local' should be false.
func New(branch string, checkout *git.Checkout, lookupCache *lookup.Cache, client *http.Client, local bool, subdomain string) (*Process, error) {
	repo := poprepo.NewPopRepo(checkout, local, subdomain)
	api, err := buildapi.NewAPI(client)
	if err != nil {
		return nil, err
	}
	return &Process{
		api:    api,
		Repo:   repo,
		branch: branch,
		lookup: lookupCache,
	}, nil
}

// Last returns the last buildid, its timestamp, and git hash, or a non-nil
// error if and error occurred.
func (c *Process) Last(ctx context.Context) (int64, int64, string, error) {
	return c.Repo.GetLast(ctx)
}

// Start a Go routine that does the work.
func (c *Process) Start(ctx context.Context) {
	go func() {
		t := metrics2.NewTimer("repobuilder")
		liveness := metrics2.NewLiveness("last_successful_add")
		failures := metrics2.GetCounter("process_failures", nil)
		for range time.Tick(time.Minute) {
			t.Start()
			buildid, _, _, err := c.Repo.GetLast(ctx)
			if err != nil {
				failures.Inc(1)
				sklog.Errorf("Failed to get last buildid: %s", err)
				continue
			}
			builds, err := c.api.List(c.branch, buildid)
			if err != nil {
				failures.Inc(1)
				sklog.Errorf("Failed to get buildids from api: %s", err)
				continue
			}
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
			liveness.Reset()
			t.Stop()
		}
	}()
}
