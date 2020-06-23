package repo_root

import (
	"fmt"
	"os"
	"strings"
)

// Return the path to the repo root. Note that this will return an error if
// the CWD is not inside a checkout, so this cannot run on production servers.
func Get() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return GetFromString(cwd)
}

// Return the path to the repo root, assuming the given working directory.
func GetFromString(cwd string) (string, error) {
	prefixes := []string{"go.skia.org/infra", "buildbot"}
	for _, prefix := range prefixes {
		if strings.Contains(cwd, prefix) {
			return strings.Split(cwd, prefix)[0] + prefix, nil
		}
	}
	return "", fmt.Errorf("No repo root found; are we running inside a checkout?")
}
