package flakyprovider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

// Hits the status backend and
type Provider struct {
	client *http.Client
}

func jsonToFlaky(r io.Reader) (map[string]time.Time, error) {
	comments := []*db.RepoComments{}
	if err := json.NewDecoder(r).Decode(comments); err != nil {
		return nil, fmt.Errorf("Failed to decode task comments from status: %s", err)
	}
	ret := map[string]time.Time{}
	for _, rc := range comments {
		for botname, taskComments := range rc.TaskSpecComments {
			for _, c := range taskComments {
				if c.Flaky {
					ret[botname] = c.Timestamp
				}
			}
		}
	}
	return ret, nil
}

func (p *Provider) Get() (map[string]time.Time, error) {
	resp, err := p.client.Get("https://status.skia.org/json/skia/all_comments")
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve task state from status: %s", err)
	}
	defer util.Close(resp.Body)

	return jsonToFlaky(resp.Body)
}
