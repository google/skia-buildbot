package validbots

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"go.skia.org/infra/go/util"
)

// ValidBots returns the list of valid bots by reading the infra/bots/jobs.json
// file in the given repo.
func ValidBots(gitRepoDir string) ([]string, error) {
	f, err := os.Open(path.Join(gitRepoDir, "skia", "infra", "bots", "jobs.json"))
	if err != nil {
		return nil, fmt.Errorf("Failed to open jobs.json: %s", err)
	}
	defer util.Close(f)
	ret := []string{}
	if err := json.NewDecoder(f).Decode(&ret); err != nil {
		return nil, fmt.Errorf("Failed to decode jobs.json: %s", err)
	}
	return ret, nil
}
