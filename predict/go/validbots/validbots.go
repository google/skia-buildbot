package validbots

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"go.skia.org/infra/go/util"
)

type Tasks struct {
	Jobs map[string]interface{} `json:"jobs"`
}

// ValidBots returns the list of valid bots by reading the infra/bots/tasks.json
// file in the given repo.
//
// Note that if we start sharding jobs then the task names won't line up with
// the job names, so we'll need to look up the tasks in 'tasks.json' and see
// which jobs are associated with that task.  Today that's only the coverage
// bot, so we will take the easy path for now.
func ValidBots(gitRepoDir string) ([]string, error) {
	f, err := os.Open(path.Join(gitRepoDir, "infra", "bots", "tasks.json"))
	if err != nil {
		return nil, fmt.Errorf("Failed to open tasks.json: %s", err)
	}
	defer util.Close(f)
	var t Tasks
	if err := json.NewDecoder(f).Decode(&t); err != nil {
		return nil, fmt.Errorf("Failed to decode tasks.json: %s", err)
	}

	jobs := []string{}
	for key, _ := range t.Jobs {
		jobs = append(jobs, key)
	}
	return jobs, nil

}
