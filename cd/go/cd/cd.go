package cd

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gerrit/rubberstamper"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/louhi"
	"go.skia.org/infra/go/louhi/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/td"
	"golang.org/x/oauth2/google"
)

// UploadCL uploads a CL with the given changes. It builds the commit message
// starting with the given commitSubject. If srcRepo and srcCommit are provided,
// a link back to the source commit is added to the commit message.  If
// louhiPubsubProject and louhiExecutionID are provided, a pub/sub message is
// sent after the CL is uploaded.
func UploadCL(ctx context.Context, changes map[string]string, dstRepo, baseCommit, commitSubject, srcRepo, srcCommit, louhiPubsubProject, louhiExecutionID string) error {
	ctx = td.StartStep(ctx, td.Props("UploadCL"))
	defer td.EndStep(ctx)

	// Build the commit message.
	commitMsg := commitSubject
	if srcCommit != "" {
		shortCommit := srcCommit
		if len(shortCommit) > 12 {
			shortCommit = shortCommit[:12]
		}
		commitMsg += " for " + shortCommit
	}
	commitMsg += "\n\n"
	if srcRepo != "" && srcCommit != "" {
		ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
		if err != nil {
			return skerr.Wrap(err)
		}
		client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
		gitilesRepo := gitiles.NewRepo(srcRepo, client)
		commitDetails, err := gitilesRepo.Details(ctx, srcCommit)
		if err != nil {
			return skerr.Wrap(err)
		}
		commitMsg += fmt.Sprintf("%s/+/%s\n\n", srcRepo, srcCommit)
		commitMsg += commitDetails.Subject
		commitMsg += "\n\n"
	}

	// Create the CL.
	gerritURL, gerritProject, err := gerrit.ParseGerritURLAndProject(dstRepo)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	ts, err := google.DefaultTokenSource(ctx, gerrit.AuthScope)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	g, err := gerrit.NewGerrit(gerritURL, client)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	reviewers := []string{rubberstamper.RubberStamperUser}
	ci, err := gerrit.CreateCLWithChanges(ctx, g, gerritProject, git.MainBranch, commitMsg, baseCommit, changes, reviewers)
	if err != nil {
		return td.FailStep(ctx, err)
	}

	// Send a pub/sub message.
	if louhiPubsubProject != "" && louhiExecutionID != "" {
		sender, err := pubsub.NewPubSubSender(ctx, louhiPubsubProject)
		if err != nil {
			return skerr.Wrap(err)
		}
		if err := sender.Send(ctx, &louhi.Notification{
			EventAction:         louhi.EventAction_CREATED_ARTIFACT,
			GeneratedCls:        []string{g.Url(ci.Issue)},
			PipelineExecutionId: louhiExecutionID,
		}); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}
