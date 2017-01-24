package depot_tools

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"go.skia.org/infra/go/sklog"
)

/*
	Utility for finding a depot_tools checkout.
*/

func Find() (string, error) {
	// First, check the environment.
	depotTools := os.Getenv("DEPOT_TOOLS")
	if depotTools != "" {
		if _, err := os.Stat(depotTools); err == nil {
			return depotTools, nil
		}
		sklog.Errorf("DEPOT_TOOLS=%s but dir does not exist!", depotTools)
	}

	// If "gclient" is in PATH, then we know where to get depot_tools.
	gclient, err := exec.LookPath("gclient")
	if err == nil && gclient != "" {
		return path.Dir(gclient), nil
	}

	// Give up.
	return "", fmt.Errorf("Unable to find depot_tools.")
}
