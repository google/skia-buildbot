// Package continuous periodically queries the android build api and looks for
// new buildids against a given branch and then updates poprepo with those new
// buildids.
package continuous

import (
	"net/http"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/android_ingest/go/buildapi"
	"go.skia.org/infra/android_ingest/go/lookup"
	"go.skia.org/infra/android_ingest/go/poprepo"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/metrics2"
)

// Process periodically queries the android build api and looks for new
// buildids against a given branch and then updates poprepo with those new
// buildids.
type Process struct {
	api    *buildapi.API
	repo   *poprepo.PopRepo
	branch string
	lookup *lookup.Cache
}

// New returns a new *Process.
//
// The lookupCache has entries added as they are found in Start().
//
// If running in production then 'local' should be false.
func New(branch string, checkout *git.Checkout, lookupCache *lookup.Cache, client *http.Client, local bool) (*Process, error) {
	repo := poprepo.NewPopRepo(checkout, local)
	api, err := buildapi.NewAPI(client)
	if err != nil {
		return nil, err
	}
	return &Process{
		api:    api,
		repo:   repo,
		branch: branch,
		lookup: lookupCache,
	}, nil
}

// Last returns the last buildid, its timestamp, and git hash, or a non-nil
// error if and error occurred.
func (c *Process) Last() (int64, int64, string, error) {
	return c.repo.GetLast()
}

// Start a Go routine that does the work.
func (c *Process) Start() {
	go func() {
		t := metrics2.NewTimer("repobuilder")
		for _ = range time.Tick(time.Minute) {
			t.Start()
			buildid, _, _, err := c.repo.GetLast()
			if err != nil {
				glog.Errorf("Failed to get last buildid: %s", err)
				continue
			}
			builds, err := c.api.List(c.branch, buildid)
			if err != nil {
				glog.Errorf("Failed to get buildids from api: %s", err)
				continue
			}
			for _, b := range builds {
				if err := c.repo.Add(b.BuildId, b.TS); err != nil {
					glog.Errorf("Failed to add new buildid to repo: %s", err)
					// Break since we don't want to add anymore buildids until this one
					// lands successfully.
					break
				}
				// Keep lookup.Cache up to date.
				buildid, _, hash, err := c.repo.GetLast()
				if err != nil {
					glog.Errorf("Failed to lookup newly added buildid to repo: %s", err)
					break
				}
				c.lookup.Add(buildid, hash)
			}
			t.Stop()
		}
	}()
}
