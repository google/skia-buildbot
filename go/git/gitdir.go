package git

/*
	Common utils used by Repo and Checkout.
*/

import (
	"fmt"
	"os"
	"path"

	"go.skia.org/infra/go/exec"
)

// GitDir is a directory in which one may run Git commands.
type GitDir string

// newGitDir creates a GitDir instance based in the given directory.
func newGitDir(repoUrl, dir string, mirror bool) (GitDir, error) {
	dest := path.Join(dir, path.Base(repoUrl))
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			// Clone the repo.
			cmd := []string{"git", "clone"}
			if mirror {
				cmd = append(cmd, "--mirror")
			}
			cmd = append(cmd, repoUrl, dest)
			if _, err := exec.RunCwd(dir, cmd...); err != nil {
				return "", fmt.Errorf("Failed to clone repo: %s", err)
			}
		} else {
			return "", fmt.Errorf("Failed to create git.Repo: %s", err)
		}
	}
	return GitDir(dest), nil
}

// Dir returns the working directory of the GitDir.
func (g GitDir) Dir() string {
	return string(g)
}

// Git runs the given git command in the GitDir.
func (g GitDir) Git(cmd ...string) (string, error) {
	return exec.RunCwd(string(g), append([]string{"git"}, cmd...)...)
}
