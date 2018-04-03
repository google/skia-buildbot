package repo_root

import (
	"fmt"
	"os"
	"strings"
)

// Return the path to the repo root. Note that this will return an error if
// the CWD is not inside a checkout, so this cannot run on production servers.
func Get() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	prefix := "go.skia.org/infra"
	if !strings.Contains(dir, prefix) {
		return "", fmt.Errorf("No repo root found; are we running inside a checkout?")
	}
	return strings.Split(dir, prefix)[0] + prefix, nil
}
