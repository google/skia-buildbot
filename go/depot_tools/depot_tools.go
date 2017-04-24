package depot_tools

/*
   Utility for finding a depot_tools checkout.
*/

import (
	"fmt"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
)

const (
	REVISION = "e663133f6f2695efba0705e82ee581f6eb424e6c"
)

// Sync syncs the depot_tools checkout to REVISION.
func Sync(workdir string) (string, error) {
	// Clone the repo if necessary.
	co, err := git.NewCheckout(common.REPO_DEPOT_TOOLS, workdir)
	if err != nil {
		return "", err
	}

	// Avoid doing any syncing if we already have the desired revision.
	hash, err := co.RevParse("HEAD")
	if err != nil {
		return "", err
	}
	if hash == REVISION {
		return co.Dir(), nil
	}

	// Sync the checkout into the desired state.
	if err := co.Fetch(); err != nil {
		return "", err
	}
	if err := co.Cleanup(); err != nil {
		return "", err
	}
	if _, err := co.Git("reset", "--hard", REVISION); err != nil {
		return "", err
	}
	hash, err = co.RevParse("HEAD")
	if err != nil {
		return "", err
	}
	if hash != REVISION {
		return "", fmt.Errorf("Got incorrect depot_tools revision: %s", hash)
	}
	return co.Dir(), nil
}
