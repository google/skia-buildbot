package validbots

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"go.skia.org/infra/go/util"
)

type tasks struct {
	Jobs map[string]interface{} `json:"jobs"`
}

// ValidBots returns the list of valid bots by reading the infra/bots/tasks.json
// file in the given repo.
func ValidBots(gitRepoDir string) ([]string, error) {
	f, err := os.Open(path.Join(gitRepoDir, "skia", "infra", "bots", "tasks.json"))
	if err != nil {
		return nil, fmt.Errorf("Failed to open tasks.json: %s", err)
	}
	defer util.Close(f)
	var t tasks
	if err := json.NewDecoder(f).Decode(&t); err != nil {
		return nil, fmt.Errorf("Failed to decode tasks.json: %s", err)
	}
	ret := []string{}
	for key, _ := range t.Jobs {
		ret = append(ret, key)
	}
	return ret, nil
}
