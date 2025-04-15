package depot_tools_auth

import (
	"context"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// GitConfig sets up the git config for depot tools authentication.
//
// Note: This requires `git-credential-luci` to be present and in PATH.
func GitConfig(ctx context.Context) error {
	gitExec, err := git.Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	if _, err := exec.RunCwd(ctx, ".", gitExec, "config", "--global", "credential.https://*.googlesource.com.helper", "luci"); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := exec.RunCwd(ctx, ".", gitExec, "config", "--global", "--unset", "http.cookiefile"); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
