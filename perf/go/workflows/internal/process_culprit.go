package internal

import (
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	culprit_proto "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/workflows"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/workflow"
)

// Handles processing of identified culprits.
// Stores culprit data in a persistant storage and notifies users accordingly.
func ProcessCulpritWorkflow(ctx workflow.Context, input *workflows.ProcessCulpritParam) (*workflows.ProcessCulpritResult, error) {
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	var resp1 *culprit_proto.PersistCulpritResponse
	var resp2 *culprit_proto.NotifyUserOfCulpritResponse
	var err error
	var csa CulpritServiceActivity

	commits, err := convertPinpointCommits(input.Commits)
	if err != nil {
		return nil, err
	}
	if err = workflow.ExecuteActivity(ctx, csa.PeristCulprit, input.CulpritServiceUrl, &culprit_proto.PersistCulpritRequest{
		Commits:        commits,
		AnomalyGroupId: input.AnomalyGroupId,
	}).Get(ctx, &resp1); err != nil {
		return nil, err
	}
	if err = workflow.ExecuteActivity(ctx, csa.NotifyUserOfCulprit, input.CulpritServiceUrl, &culprit_proto.NotifyUserOfCulpritRequest{
		CulpritIds:     resp1.CulpritIds,
		AnomalyGroupId: input.AnomalyGroupId}).Get(ctx, &resp2); err != nil {
		return nil, err
	}
	return &workflows.ProcessCulpritResult{
		CulpritIds: resp1.CulpritIds,
		IssueIds:   resp2.IssueIds,
	}, nil
}

// convertPinpointCommits converts commits in pinpoint proto to culprit proto.
func convertPinpointCommits(pinpoint_commits []*pinpoint_proto.Commit) ([]*culprit_proto.Commit, error) {
	commits := make([]*culprit_proto.Commit, len(pinpoint_commits))
	var err error
	for i, pinpoint_commit := range pinpoint_commits {
		if commits[i], err = ParsePinpointCommit(pinpoint_commit); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return commits, nil
}

// ParsePinpointCommit parse a single pinpoint commit type into culprit commit type.
// We assume pinpoint_culprit.repository in a format like:
//
//	https://{host}/{project}.git
func ParsePinpointCommit(pinpoint_commit *pinpoint_proto.Commit) (*culprit_proto.Commit, error) {
	pinpoint_commit_repo := pinpoint_commit.Repository
	// Remove the "http://""
	pinpoint_commit_repo, _ = strings.CutPrefix(pinpoint_commit_repo, "https://")
	// Split host from project
	pinpoint_commit_repo_parts := strings.SplitN(pinpoint_commit_repo, "/", 2)
	if len(pinpoint_commit_repo_parts) < 2 {
		return nil, skerr.Fmt("Invalid commit repository: %s", pinpoint_commit_repo)
	}
	host := pinpoint_commit_repo_parts[0]
	project, has_git_suffix := strings.CutSuffix(pinpoint_commit_repo_parts[1], ".git")
	if !has_git_suffix {
		sklog.Warningf("Parsing commit project without seeing .git as suffix: %s", pinpoint_commit_repo)
	}
	if host == "" || project == "" || pinpoint_commit.GitHash == "" {
		return nil, skerr.Fmt("Empty values parsed in Pinpoint commit: %s", pinpoint_commit)
	}
	return &culprit_proto.Commit{
		Host:     host,
		Project:  project,
		Revision: pinpoint_commit.GitHash,
	}, nil
}
