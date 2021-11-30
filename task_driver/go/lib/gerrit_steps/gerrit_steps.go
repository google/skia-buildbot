package gerrit_steps

/*
	Package gerrit_steps provides Task Driver steps used for interacting
	with the Gerrit API.
*/

import (
	"context"
	"fmt"
	"path"
	"strings"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

// Init creates and returns an authenticated GerritInterface, or any error
// which occurred.
func Init(ctx context.Context, local bool, gerritUrl string) (gerrit.GerritInterface, error) {
	ts, err := git_steps.Init(ctx, local)
	if err != nil {
		return nil, err
	}
	var rv gerrit.GerritInterface
	err = td.Do(ctx, td.Props("Gerrit Init").Infra(), func(ctx context.Context) error {
		client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
		g, err := gerrit.NewGerrit(gerritUrl, td.HttpClient(ctx, client))
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
					gerrit.LabelBotCommit:   gerrit.LabelBotCommitApproved,
					gerrit.LabelCommitQueue: gerrit.LabelCommitQueueSubmit,
				}
			}
			if err := g.SetReview(ctx, ci, "Ready for review.", labels, reviewers, "", nil, "", 0); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// GetIssueProperties is a wrapper around GerritInterface.GetIssueProperties.
func GetIssueProperties(ctx context.Context, g gerrit.GerritInterface, issue int64) (*gerrit.ChangeInfo, error) {
	var rv *gerrit.ChangeInfo
	return rv, td.Do(ctx, td.Props(fmt.Sprintf("Get Issue %d", issue)).Infra(), func(ctx context.Context) error {
		var err error
		rv, err = g.GetIssueProperties(ctx, issue)
		return err
	})
}
