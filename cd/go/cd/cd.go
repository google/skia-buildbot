package cd

import (
	"context"
	"fmt"
	"regexp"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
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

var uploadedCLRegex = regexp.MustCompile(`https://.*review\.googlesource\.com.*\d+`)

// MaybeUploadCL uploads a CL if there are any diffs in checkoutDir. It builds
// the commit message starting with the given commitSubject. If srcRepo and
// srcCommit are provided, a link back to the source commit is added to the
// commit message.  If louhiPubsubProject and louhiExecutionID are provided,
// a pub/sub message is sent after the CL is uploaded.
func MaybeUploadCL(ctx context.Context, checkoutDir, commitSubject, srcRepo, srcCommit, louhiPubsubProject, louhiExecutionID string) error {
	ctx = td.StartStep(ctx, td.Props("MaybeUploadCL"))
	defer td.EndStep(ctx)

	gitExec, err := git.Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Did we change anything?
	if _, err := exec.RunCwd(ctx, checkoutDir, gitExec, "diff", "HEAD", "--exit-code"); err != nil {
		// If so, create a CL.

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
		commitMsg += rubberstamper.RandomChangeID()

		// Commit and push.
		if _, err := exec.RunCwd(ctx, checkoutDir, gitExec, "commit", "-a", "-m", commitMsg); err != nil {
			return skerr.Wrap(err)
		}
		output, err := exec.RunCwd(ctx, checkoutDir, gitExec, "push", git.DefaultRemote, rubberstamper.PushRequestAutoSubmit)
		if err != nil {
			return skerr.Wrap(err)
		}

		// Send a pub/sub message.
		if louhiPubsubProject != "" && louhiExecutionID != "" {
			match := uploadedCLRegex.FindString(output)
			if match == "" {
				return skerr.Fmt("Failed to parse CL link from:\n%s", output)
			}
			sender, err := pubsub.NewPubSubSender(ctx, louhiPubsubProject)
			if err != nil {
				return skerr.Wrap(err)
			}
			if err := sender.Send(ctx, &louhi.Notification{
				EventAction:         louhi.EventAction_CREATED_ARTIFACT,
				GeneratedCls:        []string{match},
				PipelineExecutionId: louhiExecutionID,
			}); err != nil {
				return skerr.Wrap(err)
			}
		}
	}
	return nil
}
