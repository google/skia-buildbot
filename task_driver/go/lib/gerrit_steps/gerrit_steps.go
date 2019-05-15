package gerrit_steps

/*
	Package gerrit_steps provides Task Driver steps used for interacting
	with the Gerrit API.
*/

import (
	"context"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/td"
)

// Init creates and returns an authenticated GerritInterface, or any error
// which occurred.
func Init(ctx context.Context, local bool, workdir, gerritUrl string) (gerrit.GerritInterface, error) {
	gitcookiesPath, err := git_steps.Init(ctx, local, workdir)
	if err != nil {
		return nil, err
	}
	var rv gerrit.GerritInterface
	err = td.Do(ctx, td.Props("Gerrit Init").Infra(), func(ctx context.Context) error {
		g, err := gerrit.NewGerrit(gerritUrl, gitcookiesPath, td.HttpClient(ctx, nil))
		rv = g
		return err
	})
	return rv, err
}
