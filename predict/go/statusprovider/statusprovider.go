package statusprovider

import (
	"fmt"
	"time"

	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

// Provider.Get implements flaky.FlakyProvider.
type Provider struct {
	client db.RemoteDB
	repos  []string
}

func New(repos []string) (*Provider, error) {
	client, err := remote_db.NewClient("https://task-scheduler.skia.org/db/")
	if err != nil {
		return nil, err
	}
	return &Provider{
		client: client,
		repos:  repos,
	}, nil
}

func processComments(comments []*db.RepoComments) (map[string]time.Time, error) {
	ret := map[string]time.Time{}
	for _, c := range comments {
		for botname, specComments := range c.TaskSpecComments {
			for _, sc := range specComments {
				if sc.Flaky || sc.IgnoreFailure {
					if currentTime, ok := ret[botname]; !ok || currentTime.After(sc.Timestamp) {
						ret[botname] = sc.Timestamp
					}
				}
			}
		}
	}

	return ret, nil
}

// Get returns the bots that are flaky and the timestamp of when the comment was made.
func (p *Provider) Get() (map[string]time.Time, error) {
	comments, err := p.client.GetCommentsForRepos(p.repos, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve comments from db: %s", err)
	}
	return processComments(comments)
}
