// Package continuous periodically queries the android build api and looks for
// new buildids against a given branch and then updates poprepo with those new
// buildids.
package continuous

import (
	"net/http"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/android_ingest/go/buildapi"
	"go.skia.org/infra/android_ingest/go/poprepo"
	"go.skia.org/infra/go/metrics2"
)

// Process periodically queries the android build api and looks for new
// buildids against a given branch and then updates poprepo with those new
// buildids.
type Process struct {
	api    *buildapi.API
	repo   *poprepo.PopRepo
	branch string
}

// New returns a new *Process.
func New(branch, repoUrl, workdir string, client *http.Client, local bool) (*Process, error) {
	repo, err := poprepo.NewPopRepo(repoUrl, workdir, local)
	if err != nil {
		return nil, err
	}
	api, err := buildapi.NewAPI(client)
	if err != nil {
		return nil, err
	}
	return &Process{
		api:    api,
		repo:   repo,
		branch: branch,
	}, nil
}

// Last returns the last buildid, its timestamp, or a non-nil error if
// and error occurred.
func (c *Process) Last() (int64, int64, error) {
	return c.repo.GetLast()
}

// Start a Go routine that does the work.
func (c *Process) Start() {
	go func() {
		t := metrics2.NewTimer("repobuilder")
		for _ = range time.Tick(time.Minute) {
			t.Start()
			buildid, _, err := c.repo.GetLast()
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
			}
			t.Stop()
		}
	}()
}
