// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cd

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"go.skia.org/infra/cd/go/stages"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit/rubberstamper"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/louhi"
	"go.skia.org/infra/go/louhi/pubsub"
	"go.skia.org/infra/go/rotations"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_driver/go/td"
)

var uploadedCLRegex = regexp.MustCompile(`https://.*review\.googlesource\.com.*\d+`)

// ResolveReviewersPushTarget builds the Gerrit push refspec for the given
// reviewer emails or rotation URLs, falling back to backupReviewers if rotation
// lookup fails or returns no emails.
func ResolveReviewersPushTarget(client *http.Client, reviewerSources, backupReviewers []string) (string, error) {
	if len(reviewerSources) == 0 && len(backupReviewers) == 0 {
		return rubberstamper.PushRequestAutoSubmit, nil
	}
	emails := util.StringSet{}
	for _, src := range reviewerSources {
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			members, err := rotations.FromURL(client, src)
			if err != nil {
				sklog.Errorf("Failed to resolve reviewer rotation %q: %s", src, err)
				continue
			}
			for _, m := range members {
				m = strings.TrimSpace(m)
				if m == "" {
					continue
				}
				if strings.Count(m, "@") != 1 {
					sklog.Errorf("Ignoring invalid reviewer %q from rotation %q", m, src)
					continue
				}
				emails[m] = true
			}
			continue
		}
		src = strings.TrimSpace(src)
		if strings.Count(src, "@") != 1 {
			return "", skerr.Fmt("invalid reviewer %q: must be an email or URL", src)
		}
		emails[src] = true
	}
	if len(emails) == 0 {
		sklog.Errorf("No reviewers resolved from %v; falling back to backup reviewers %v", reviewerSources, backupReviewers)
		for _, b := range backupReviewers {
			b = strings.TrimSpace(b)
			if strings.Count(b, "@") != 1 {
				return "", skerr.Fmt("invalid backup reviewer %q: must be an email", b)
			}
			emails[b] = true
		}
	}
	if len(emails) == 0 {
		return "", skerr.Fmt("no reviewers resolved from --reviewer or --backup-reviewer")
	}
	sorted := emails.Keys()
	sort.Strings(sorted)
	target := "HEAD:refs/for/" + git.MainBranch + "%ready,l=Auto-Submit+1"
	for _, email := range sorted {
		target += ",r=" + email
	}
	return target, nil
}

// MaybeUploadCL uploads a CL if there are any diffs in checkoutDir. It builds
// the commit message starting with the given commitSubject. If srcRepo and
// srcCommit are provided, a link back to the source commit is added to the
// commit message. If commitBody is non-empty, it is appended before Change-Id.
// If louhiPubsubProject and louhiExecutionID are provided, a pub/sub message
// is sent after the CL is uploaded.
func MaybeUploadCL(ctx context.Context, checkoutDir, commitSubject, commitBody, srcRepo, srcCommit, louhiPubsubProject, louhiExecutionID, pushTarget string) error {
	ctx = td.StartStep(ctx, td.Props("MaybeUploadCL"))
	defer td.EndStep(ctx)

	gitExec, err := git.Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Did we change anything?
	diffOutput, err := exec.RunCwd(ctx, checkoutDir, gitExec, "diff", "--name-only", "HEAD")
	if err != nil {
		return skerr.Wrap(err)
	}
	if strings.TrimSpace(diffOutput) == "" {
		return nil // No diffs, no CL
	}
	// There were diffs, so create a CL.

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
		gitilesRepo, err := gitiles.NewRepo(ctx, srcRepo)
		if err != nil {
			return skerr.Wrap(err)
		}
		commitDetails, err := gitilesRepo.Details(ctx, srcCommit)
		if err != nil {
			return skerr.Wrap(err)
		}
		commitMsg += fmt.Sprintf("%s/+/%s\n\n", srcRepo, srcCommit)
		commitMsg += commitDetails.Subject
		commitMsg += "\n\n"
	}
	if trimmedBody := strings.TrimSpace(commitBody); trimmedBody != "" {
		commitMsg += trimmedBody + "\n\n"
	}
	commitMsg += rubberstamper.RandomChangeID(ctx)

	if pushTarget == "" {
		pushTarget = rubberstamper.PushRequestAutoSubmit
	}

	// Commit and push.
	if _, err := exec.RunCwd(ctx, checkoutDir, gitExec, "commit", "-a", "-m", commitMsg); err != nil {
		return skerr.Wrap(err)
	}
	output, err := exec.RunCwd(ctx, checkoutDir, gitExec, "push", git.DefaultRemote, pushTarget)
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
