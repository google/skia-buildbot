package git_steps

/*
   Task Driver utilities for working with Git.
*/

import (
	"context"
	"path"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/td"
)

// Init initializes gitauth for a Task Driver. Returns the path to the
// gitcookies and any error which occurred.
func Init(ctx context.Context, local bool, workdir string) (string, error) {
	ts, err := auth_steps.Init(ctx, local, auth.SCOPE_GERRIT, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		return "", err
	}
	var gitcookiesPath string
	if local {
		gitcookiesPath = gerrit.DefaultGitCookiesPath()
	} else {
		gitcookiesPath = path.Join(workdir, ".gitcookies")
	}
	err = td.Do(ctx, td.Props("Gitauth Init").Infra(), func(ctx context.Context) error {
		_, err := gitauth.New(ts, gitcookiesPath, true, "")
		return err
	})
	return gitcookiesPath, err
}
