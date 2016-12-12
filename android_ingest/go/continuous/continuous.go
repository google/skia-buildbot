// Package continuous provides ...
package continuous

import (
	"net/http"
	"time"

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
	api, err := NewAPI(client)
	if err != nil {
		return nil, err
	}
	return &Continuous{
		api:    api,
		repo:   repo,
		branch: branch,
	}
}

func (c *Continuous) Start() {
	go func() {
		for _ = range time.Tick(time.Minute) {
		}
	}()
}
