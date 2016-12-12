// Package continuous provides ...
package continuous

import (
	"net/http"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/android_ingest/go/buildapi"
	"go.skia.org/infra/android_ingest/go/poprepo"
)

type Continuous struct {
	api    *buildapi.API
	repo   *poprepo.PopRepo
	branch string
}

func New(branch, repoUrl, workdir string, client *http.Client) (*Continuous, error) {
	repo, err := poprepo.NewPopRepo(repoUrl, workdir)
	if err != nil {
		return nil, err
	}
	api, err := buildapi.NewAPI(client)
	if err != nil {
		return nil, err
	}
	return &Continuous{
		api:    api,
		repo:   repo,
		branch: branch,
	}, nil
}

func (c *Continuous) Start() {
	go func() {
		for _ = range time.Tick(time.Minute) {
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
					// break since we don't want to add anymore buildids until this one
					// lands successfully.
					break
				}
			}
		}
	}()
}
