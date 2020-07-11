package codereview

import (
	"context"
	"errors"
	"fmt"

	github_api "github.com/google/go-github/v29/github"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/travisci"
)

type RollImpl interface {
	state_machine.RollCLImpl

	// Insert the roll into the DB.
	InsertIntoDB(ctx context.Context) error
}

// updateIssueFromGerrit loads details about the issue from the Gerrit API and
// updates the AutoRollIssue accordingly.
func updateIssueFromGerrit(ctx context.Context, cfg *GerritConfig, a *autoroll.AutoRollIssue, g gerrit.GerritInterface) (*gerrit.ChangeInfo, error) {
	info, err := g.GetIssueProperties(ctx, a.Issue)
	if err != nil {
		return nil, fmt.Errorf("Failed to get issue properties: %s", err)
	}
	if cfg.CanQueryTrybots() {
		// Use try results from the most recent non-trivial patchset.
		if len(info.Patchsets) == 0 {
			return nil, fmt.Errorf("Issue %d has no patchsets!", a.Issue)
		}
		nontrivial := info.GetNonTrivialPatchSets()
		if len(nontrivial) == 0 {
			msg := fmt.Sprintf("No non-trivial patchsets for %d; trivial patchsets:\n", a.Issue)
			for _, ps := range info.Patchsets {
				msg += fmt.Sprintf("  %+v\n", ps)
			}
			return nil, errors.New(msg)
		}
		tries, err := g.GetTrybotResults(ctx, a.Issue, nontrivial[len(nontrivial)-1].Number)
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve try results: %s", err)
		}
		tryResults, err := autoroll.TryResultsFromBuildbucket(tries)
		if err != nil {
			return nil, fmt.Errorf("Failed to process try results: %s", err)
		}
		a.TryResults = tryResults
	}
	if err := updateIssueFromGerritChangeInfo(a, info, g.Config()); err != nil {
		return nil, fmt.Errorf("Failed to convert issue format: %s", err)
	}
	return info, nil
}

// updateIssueFromGerritChangeInfo updates the AutoRollIssue instance based on
// the given gerrit.ChangeInfo.
func updateIssueFromGerritChangeInfo(i *autoroll.AutoRollIssue, ci *gerrit.ChangeInfo, gc *gerrit.Config) error {
	if i.Issue != ci.Issue {
		return fmt.Errorf("CL ID %d differs from existing issue number %d!", ci.Issue, i.Issue)
	}
	i.CqFinished = !i.IsDryRun && !gc.CqRunning(ci)
	i.CqSuccess = !i.IsDryRun && gc.CqSuccess(ci)
	i.DryRunFinished = i.IsDryRun && !gc.DryRunRunning(ci)
	i.DryRunSuccess = i.IsDryRun && gc.DryRunSuccess(ci, gc.DryRunUsesTryjobResults && i.AllTrybotsSucceeded())

	ps := make([]int64, 0, len(ci.Patchsets))
	for _, p := range ci.Patchsets {
		ps = append(ps, p.Number)
	}
	i.Closed = ci.IsClosed()
	i.Committed = ci.Committed
	i.Created = ci.Created
	i.Modified = ci.Updated
	i.Patchsets = ps
	i.Subject = ci.Subject
	i.Result = autoroll.RollResult(i)
	// TODO(borenet): If this validation fails, it's likely that it will
	// continue to fail indefinitely, resulting in a stuck roller.
	// Additionally, this AutoRollIssue instance persists in the AutoRoller
	// for its entire lifetime; it's possible to partially fail to update
	// it and end up in an inconsistent state.
	return i.Validate()
}

// gerritRoll is an implementation of RollImpl.
type gerritRoll struct {
	ci               *gerrit.ChangeInfo
	issue            *autoroll.AutoRollIssue
	issueUrl         string
	finishedCallback func(context.Context, RollImpl) error
	g                gerrit.GerritInterface
	recent           *recent_rolls.RecentRolls
	retrieveRoll     func(context.Context) (*gerrit.ChangeInfo, error)
	result           string
	rollingTo        *revision.Revision
}

// newGerritRoll obtains a gerritRoll instance from the given Gerrit issue
// number.
func newGerritRoll(ctx context.Context, cfg *GerritConfig, issue *autoroll.AutoRollIssue, g gerrit.GerritInterface, recent *recent_rolls.RecentRolls, issueUrlBase string, rollingTo *revision.Revision, cb func(context.Context, RollImpl) error) (RollImpl, error) {
	ci, err := updateIssueFromGerrit(ctx, cfg, issue, g)
	if err != nil {
		return nil, err
	}
	return &gerritRoll{
		ci:               ci,
		issue:            issue,
		issueUrl:         fmt.Sprintf("%s%d", issueUrlBase, issue.Issue),
		finishedCallback: cb,
		g:                g,
		recent:           recent,
		retrieveRoll: func(ctx context.Context) (*gerrit.ChangeInfo, error) {
			return updateIssueFromGerrit(ctx, cfg, issue, g)
		},
		rollingTo: rollingTo,
	}, nil
}

// See documentation for RollImpl interface.
func (r *gerritRoll) InsertIntoDB(ctx context.Context) error {
	return r.recent.Add(ctx, r.issue)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) AddComment(ctx context.Context, msg string) error {
	return r.g.AddComment(ctx, r.ci, msg)
}

// Helper function for modifying a roll CL which might fail due to the CL being
// closed by a human or some other process, in which case we don't want to error
// out.
func (r *gerritRoll) withModify(ctx context.Context, action string, fn func() error) error {
	if err := fn(); err != nil {
		// It's possible that somebody abandoned the CL (or the CL
		// landed) while we were working. If that's the case, log an
		// error and move on.
		if err2 := r.Update(ctx); err2 != nil {
			return fmt.Errorf("Failed to %s with error:\n%s\nAnd failed to update it with error:\n%s", action, err, err2)
		}
		if r.ci.IsClosed() {
			sklog.Errorf("Attempted to %s but it is already closed! Error: %s", action, err)
			return nil
		}
		return err
	}
	return r.Update(ctx)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) Close(ctx context.Context, result, msg string) error {
	sklog.Infof("Closing issue %d (result %q) with message: %s", r.ci.Issue, result, msg)
	r.result = result
	return r.withModify(ctx, "close the CL", func() error {
		return r.g.Abandon(ctx, r.ci, msg)
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IsClosed() bool {
	return r.issue.Closed
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IsFinished() bool {
	return r.issue.CqFinished
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IsSuccess() bool {
	return r.issue.CqSuccess
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IsDryRunFinished() bool {
	return r.issue.DryRunFinished
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IsDryRunSuccess() bool {
	return r.issue.DryRunSuccess
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) RollingTo() *revision.Revision {
	return r.rollingTo
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) SwitchToDryRun(ctx context.Context) error {
	return r.withModify(ctx, "switch the CL to dry run", func() error {
		if err := r.g.SendToDryRun(ctx, r.ci, "Mode was changed to dry run"); err != nil {
			return err
		}
		r.issue.IsDryRun = true
		return nil
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) SwitchToNormal(ctx context.Context) error {
	return r.withModify(ctx, "switch the CL out of dry run", func() error {
		if err := r.g.SendToCQ(ctx, r.ci, "Mode was changed to normal"); err != nil {
			return err
		}
		r.issue.IsDryRun = false
		return nil
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) RetryCQ(ctx context.Context) error {
	return r.withModify(ctx, "retry the CQ", func() error {
		if err := r.g.SendToCQ(ctx, r.ci, "CQ failed but there are no new commits. Retrying..."); err != nil {
			return err
		}
		r.issue.IsDryRun = false
		return nil
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) RetryDryRun(ctx context.Context) error {
	return r.withModify(ctx, "retry the CQ (dry run)", func() error {
		if err := r.g.SendToDryRun(ctx, r.ci, "Dry run failed but there are no new commits. Retrying..."); err != nil {
			return err
		}
		r.issue.IsDryRun = true
		return nil
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) Update(ctx context.Context) error {
	alreadyFinished := r.IsFinished()
	ci, err := r.retrieveRoll(ctx)
	if err != nil {
		return err
	}
	r.ci = ci
	if r.result != "" {
		r.issue.Result = r.result
	}
	if err := r.recent.Update(ctx, r.issue); err != nil {
		return err
	}
	if r.IsFinished() && !alreadyFinished && r.finishedCallback != nil {
		return r.finishedCallback(ctx, r)
	}
	return nil
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IssueID() string {
	return fmt.Sprintf("%d", r.issue.Issue)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IssueURL() string {
	return r.issueUrl
}

// githubRoll is an implementation of RollImpl.
// TODO(rmistry): Add tests after a code-review abstraction later exists.
type githubRoll struct {
	finishedCallback func(context.Context, RollImpl) error
	g                *github.GitHub
	issue            *autoroll.AutoRollIssue
	issueUrl         string
	pullRequest      *github_api.PullRequest
	recent           *recent_rolls.RecentRolls
	result           string
	retrieveRoll     func(context.Context) (*github_api.PullRequest, error)
	rollingTo        *revision.Revision
	t                *travisci.TravisCI
}

// updateIssueFromGitHub loads details about the pull request from the GitHub
// API and updates the AutoRollIssue accordingly.
func updateIssueFromGitHub(ctx context.Context, a *autoroll.AutoRollIssue, g *github.GitHub, checksWaitFor []string) (*github_api.PullRequest, error) {

	// Retrieve the pull request from github.
	pullRequest, err := g.GetPullRequest(int(a.Issue))
	if err != nil {
		return nil, fmt.Errorf("Failed to get pull request for %d: %s", a.Issue, err)
	}

	// Get all checks for this pull request and convert to try results.
	checks, err := g.GetChecks(pullRequest.Head.GetSHA())
	if err != nil {
		return nil, err
	}
	if a.IsDryRun {
		// Do not wait for any checks if it is a dry run.
		checksWaitFor = []string{}
	}
	a.TryResults = autoroll.TryResultsFromGithubChecks(checks, checksWaitFor)

	if err := updateIssueFromGitHubPullRequest(a, pullRequest); err != nil {
		return nil, fmt.Errorf("Failed to convert issue format: %s", err)
	}

	return pullRequest, nil
}

// updateIssueFromGitHubPullRequest updates the AutoRollIssue instance based on the
// given PullRequest.
func updateIssueFromGitHubPullRequest(i *autoroll.AutoRollIssue, pullRequest *github_api.PullRequest) error {
	prNum := int64(pullRequest.GetNumber())
	if i.Issue != prNum {
		return fmt.Errorf("Pull request number %d differs from existing issue number %d!", prNum, i.Issue)
	}
	doesWaitingForTreeLabelExist := false
	for _, l := range pullRequest.Labels {
		if l.GetName() == github.WAITING_FOR_GREEN_TREE_LABEL {
			doesWaitingForTreeLabelExist = true
			break
		}
	}

	if i.IsDryRun {
		i.CqFinished = false
		i.CqSuccess = false
		// TODO(rmistry): Sometimes the github API does not return the correct number of checks, so this might
		// return some false positives.
		i.DryRunFinished = pullRequest.GetState() == github.CLOSED_STATE || pullRequest.GetMerged() || (len(i.TryResults) > 0 && i.AllTrybotsFinished())
		i.DryRunSuccess = pullRequest.GetMerged() || (i.DryRunFinished && i.AllTrybotsSucceeded())
	} else {
		i.CqFinished = pullRequest.GetState() == github.CLOSED_STATE || pullRequest.GetMerged() || !doesWaitingForTreeLabelExist || (pullRequest.GetMergeableState() == github.MERGEABLE_STATE_DIRTY)
		i.CqSuccess = pullRequest.GetMerged()
		i.DryRunFinished = false
		i.DryRunSuccess = false
	}

	ps := make([]int64, 0, *pullRequest.Commits)
	for i := 1; i <= *pullRequest.Commits; i++ {
		ps = append(ps, int64(i))
	}
	i.Closed = pullRequest.GetState() == github.CLOSED_STATE
	i.Committed = pullRequest.GetMerged()
	i.Created = pullRequest.GetCreatedAt()
	i.Modified = pullRequest.GetUpdatedAt()
	i.Patchsets = ps
	i.Subject = pullRequest.GetTitle()
	i.Result = autoroll.RollResult(i)
	// TODO(borenet): If this validation fails, it's likely that it will
	// continue to fail indefinitely, resulting in a stuck roller.
	// Additionally, this AutoRollIssue instance persists in the AutoRoller
	// for its entire lifetime; it's possible to partially fail to update
	// it and end up in an inconsistent state.
	return i.Validate()
}

// newGithubRoll obtains a githubRoll instance from the given Gerrit issue number.
func newGithubRoll(ctx context.Context, issue *autoroll.AutoRollIssue, g *github.GitHub, recent *recent_rolls.RecentRolls, issueUrlBase string, config *GithubConfig, rollingTo *revision.Revision, cb func(context.Context, RollImpl) error) (RollImpl, error) {
	pullRequest, err := updateIssueFromGitHub(ctx, issue, g, config.ChecksWaitFor)
	if err != nil {
		return nil, err
	}
	return &githubRoll{
		finishedCallback: cb,
		g:                g,
		issue:            issue,
		issueUrl:         fmt.Sprintf("%s%d", issueUrlBase, issue.Issue),
		pullRequest:      pullRequest,
		recent:           recent,
		retrieveRoll: func(ctx context.Context) (*github_api.PullRequest, error) {
			return updateIssueFromGitHub(ctx, issue, g, config.ChecksWaitFor)
		},
		rollingTo: rollingTo,
	}, nil
}

// See documentation for state_machine.RollImpl interface.
func (r *githubRoll) InsertIntoDB(ctx context.Context) error {
	return r.recent.Add(ctx, r.issue)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) AddComment(ctx context.Context, msg string) error {
	return r.g.AddComment(r.pullRequest.GetNumber(), msg)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) Close(ctx context.Context, result, msg string) error {
	sklog.Infof("Closing pull request %d (result %q) with message: %s", r.pullRequest.GetNumber(), result, msg)
	r.result = result
	return r.withModify(ctx, "close the pull request", func() error {
		if err := r.g.AddComment(r.pullRequest.GetNumber(), msg); err != nil {
			return err
		}
		_, err := r.g.ClosePullRequest(r.pullRequest.GetNumber())
		return err
	})
}

// Helper function for modifying a roll CL which might fail due to the CL being
// closed by a human or some other process, in which case we don't want to error
// out.
func (r *githubRoll) withModify(ctx context.Context, action string, fn func() error) error {
	if err := fn(); err != nil {
		// It's possible that somebody abandoned the CL (or the CL
		// landed) while we were working. If that's the case, log an
		// error and move on.
		if err2 := r.Update(ctx); err2 != nil {
			return fmt.Errorf("Failed to %s with error:\n%s\nAnd failed to update it with error:\n%s", action, err, err2)
		}
		if r.pullRequest.GetState() == github.CLOSED_STATE {
			sklog.Errorf("Attempted to %s but it is already closed! Error: %s", action, err)
			return nil
		}
		return err
	}
	return r.Update(ctx)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) Update(ctx context.Context) error {
	alreadyFinished := r.IsFinished()
	pullRequest, err := r.retrieveRoll(ctx)
	if err != nil {
		return err
	}
	r.pullRequest = pullRequest
	if r.result != "" {
		r.issue.Result = r.result
	}
	if err := r.recent.Update(ctx, r.issue); err != nil {
		return err
	}
	if r.IsFinished() && !alreadyFinished && r.finishedCallback != nil {
		return r.finishedCallback(ctx, r)
	}
	return nil
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) IsClosed() bool {
	return r.issue.Closed
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) IsFinished() bool {
	return r.issue.CqFinished
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) IsSuccess() bool {
	return r.issue.CqSuccess
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) IsDryRunFinished() bool {
	return r.issue.DryRunFinished
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) IsDryRunSuccess() bool {
	return r.issue.DryRunSuccess
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) RollingTo() *revision.Revision {
	return r.rollingTo
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) SwitchToDryRun(ctx context.Context) error {
	return r.withModify(ctx, "switch the CL to dry run", func() error {
		if err := r.g.RemoveLabel(r.pullRequest.GetNumber(), github.WAITING_FOR_GREEN_TREE_LABEL); err != nil {
			return err
		}
		r.issue.IsDryRun = true
		return nil
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) SwitchToNormal(ctx context.Context) error {
	return r.withModify(ctx, "switch the CL out of dry run", func() error {
		if err := r.g.AddLabel(r.pullRequest.GetNumber(), github.WAITING_FOR_GREEN_TREE_LABEL); err != nil {
			return err
		}
		r.issue.IsDryRun = false
		return nil
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) RetryCQ(ctx context.Context) error {
	return r.withModify(ctx, "re-trigger checks and re-apply the waiting for green label", func() error {
		if err := r.g.ReRequestLatestCheckSuite(r.pullRequest.Head.GetSHA()); err != nil {
			return err
		}
		if err := r.g.AddLabel(r.pullRequest.GetNumber(), github.WAITING_FOR_GREEN_TREE_LABEL); err != nil {
			return err
		}
		r.issue.IsDryRun = false
		return nil
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) RetryDryRun(ctx context.Context) error {
	return r.withModify(ctx, "re-trigger checks", func() error {
		if err := r.g.ReRequestLatestCheckSuite(r.pullRequest.Head.GetSHA()); err != nil {
			return err
		}
		r.issue.IsDryRun = true
		return nil
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) IssueID() string {
	return fmt.Sprintf("%d", r.issue.Issue)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) IssueURL() string {
	return r.issueUrl
}
