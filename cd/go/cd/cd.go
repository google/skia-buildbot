package cd

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.skia.org/infra/cd/go/stages"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit/rubberstamper"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/louhi"
	"go.skia.org/infra/go/louhi/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
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

// DockerImageWithGitCommit pairs a Docker image instance with the Git commit at
// which the image was built.
type DockerImageWithGitCommit struct {
	Digest string
	Commit *vcsinfo.LongCommit
	Tags   []string
}

// MatchDockerImagesToGitCommits retrieves all versions of a given Docker image
// and maps them to the Git commits at which they were built.
func MatchDockerImagesToGitCommits(ctx context.Context, dockerClient docker.Client, repo gitiles.GitilesRepo, image string, limit int) ([]*DockerImageWithGitCommit, error) {
	registry, repository, _, err := docker.SplitImage(image)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	instances, err := dockerClient.ListInstances(ctx, registry, repository)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	instancesByCommit := make(map[string]*DockerImageWithGitCommit, len(instances))

	for _, instance := range instances {
		var commitHash string
		for _, tag := range instance.Tags {
			if strings.HasPrefix(tag, stages.GitTagPrefix) {
				commitHash = strings.TrimPrefix(tag, stages.GitTagPrefix)
				break
			}
		}
		if !git.IsFullCommitHash(commitHash) {
			continue
		}
		instancesByCommit[commitHash] = &DockerImageWithGitCommit{
			Digest: instance.Digest,
			Tags:   instance.Tags,
		}
	}

	rv := make([]*DockerImageWithGitCommit, 0, len(instances))
	if err := repo.LogFnBatch(ctx, git.MainBranch, func(ctx context.Context, commits []*vcsinfo.LongCommit) error {
		for _, c := range commits {
			instance, ok := instancesByCommit[c.Hash]
			if ok {
				instance.Commit = c
				rv = append(rv, instance)
				delete(instancesByCommit, c.Hash)
				if len(instancesByCommit) == 0 || len(rv) >= limit {
					return gitiles.ErrStopIteration
				}
			}
		}
		return nil
	}, gitiles.LogBatchSize(limit)); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}
