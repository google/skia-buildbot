package repo_root

import (
	"fmt"
	"os"
	"strings"
)

// Get returns the path to the repo root. Note that this will return an error if
// the CWD is not inside a checkout, so this cannot run on production servers.
func Get() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	prefixes := []string{"go.skia.org/infra", "buildbot"}
	for _, prefix := range prefixes {
		if strings.Contains(dir, prefix) {
			return strings.Split(dir, prefix)[0] + prefix, nil
		}
	}
	// If this function is used outside of tests, please remove the following.
	if d := os.Getenv("WORKSPACE_DIR"); d != "" {
		return d, nil
	}
	return "", fmt.Errorf("No repo root found; are we running inside a checkout?")
}
