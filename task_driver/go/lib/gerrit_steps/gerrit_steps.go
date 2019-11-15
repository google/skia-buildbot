package gerrit_steps

/*
	Package gerrit_steps provides Task Driver steps used for interacting
	with the Gerrit API.
*/

import (
	"context"
	"path"
	"strings"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
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

// UploadCL uploads a CL containing any changes to the given git.Checkout. This
// is a no-op if there are no changes.
func UploadCL(ctx context.Context, g gerrit.GerritInterface, co *git.Checkout, project, branch, baseRevision, commitMsg string, reviewers []string, isTryJob bool) error {
	diff, err := co.Git(ctx, "diff", "--name-only")
	if err != nil {
		return err
	}
	diff = strings.TrimSpace(diff)
	modFiles := strings.Split(diff, "\n")
	if len(modFiles) > 0 && diff != "" {
		if err := td.Do(ctx, td.Props("Upload CL").Infra(), func(ctx context.Context) error {
			ci, err := gerrit.CreateAndEditChange(ctx, g, project, branch, commitMsg, baseRevision, func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
				for _, f := range modFiles {
					contents, err := os_steps.ReadFile(ctx, path.Join(co.Dir(), f))
					if err != nil {
						return err
					}
					if err := g.EditFile(ctx, ci, f, string(contents)); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
			var labels map[string]int
			if !isTryJob {
				labels = map[string]int{
					gerrit.CODEREVIEW_LABEL:  gerrit.CODEREVIEW_LABEL_APPROVE,
					gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_SUBMIT,
				}
			}
			if err := g.SetReview(ctx, ci, "Ready for review.", labels, reviewers); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
