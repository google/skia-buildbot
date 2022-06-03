package codereview

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	github_api "github.com/google/go-github/v29/github"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/travisci"
)

const (
	// GitHubPRDurationForChecks is the duration after a PR is created that
	// checks should be looked at.
	GitHubPRDurationForChecks = time.Minute * 15
)

var (
	// Create exponential backoff config to use for Github calls.
	//
	// The below example demonstrates what a series of
	// retries will look like. retry_interval is 10 seconds,
	// randomization_factor is 0.5, multiplier is 2 and the max_interval is 3
	// minutes. For 5 tries the sequence will be (values in seconds) and
	// assuming we go over the max_elapsed_time on the 5th try:
	//
	//  attempt#      retry_interval      randomized_interval
	//  1              10                 [5,   15]
	//  2              20                 [10,  30]
	//  3              40                 [20,  60]
	//  4              60                 [30,  90]
	//  5             120                 backoff.Stop
	GithubBackOffConfig = &backoff.ExponentialBackOff{
		InitialInterval:     10 * time.Second,
		RandomizationFactor: 0.5,
		Multiplier:          2,
		MaxInterval:         3 * time.Minute,
		MaxElapsedTime:      5 * time.Minute,
		Clock:               backoff.SystemClock,
	}
)

// RollImpl describes the behavior of an autoroll CL.
type RollImpl interface {
	state_machine.RollCLImpl

	// Insert the roll into the DB.
	InsertIntoDB(ctx context.Context) error
}

// updateIssueFromGerrit loads details about the issue from the Gerrit API and
// updates the AutoRollIssue accordingly.
func updateIssueFromGerrit(ctx context.Context, cfg *config.GerritConfig, a *autoroll.AutoRollIssue, g gerrit.GerritInterface) (*gerrit.ChangeInfo, error) {
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
	gitiles          *gitiles.Repo
	recent           *recent_rolls.RecentRolls
	retrieveRoll     func(context.Context) (*gerrit.ChangeInfo, error)
	result           string
	rollingTo        *revision.Revision
}

// newGerritRoll obtains a gerritRoll instance from the given Gerrit issue
// number.
func newGerritRoll(ctx context.Context, cfg *config.GerritConfig, issue *autoroll.AutoRollIssue, g gerrit.GerritInterface, client *http.Client, recent *recent_rolls.RecentRolls, issueUrlBase string, rollingTo *revision.Revision, cb func(context.Context, RollImpl) error) (RollImpl, error) {
	ci, err := updateIssueFromGerrit(ctx, cfg, issue, g)
	if err != nil {
		return nil, err
	}
	gitiles := gitiles.NewRepo(g.GetRepoUrl()+"/"+ci.Project, client)
	return &gerritRoll{
		ci:               ci,
		issue:            issue,
		issueUrl:         fmt.Sprintf("%s%d", issueUrlBase, issue.Issue),
		finishedCallback: cb,
		g:                g,
		gitiles:          gitiles,
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
func (r *gerritRoll) IsCommitted() bool {
	return r.issue.Committed
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

// maybeRebaseCL determines whether the CL needs to be rebased and does so if
// necessary.
func (r *gerritRoll) maybeRebaseCL(ctx context.Context) error {
	head, err := r.gitiles.Details(ctx, r.ci.Branch)
	if err != nil {
		return skerr.Wrap(err)
	}
	rollCommit, err := r.g.GetCommit(ctx, r.ci.Issue, r.ci.Patchsets[len(r.ci.Patchsets)-1].ID)
	if err != nil {
		return skerr.Wrap(err)
	}
	if len(rollCommit.Parents) == 0 {
		sklog.Errorf("Commit %s returned by Gerrit.GetCommit has no parents.", rollCommit.Commit)
	} else if rollCommit.Parents[0].Commit != head.Hash {
		sklog.Infof("HEAD is %s and CL is based on %s; attempting rebase.", head.Hash, rollCommit.Parents[0].Commit)
		if err := r.g.Rebase(ctx, r.ci, "", false); err != nil {
			if strings.Contains(err.Error(), gerrit.ErrMergeConflict) {
				if err2 := r.g.Abandon(ctx, r.ci, "Failed to rebase due to merge conflict; closing CL."); err2 != nil {
					return skerr.Wrapf(err, "failed to rebase due to merge conflict and failed to abandon CL with: %s", err2)
				}
			}
			return skerr.Wrap(err)
		}
		return r.Update(ctx)
	}
	return nil
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) RetryCQ(ctx context.Context) error {
	return r.withModify(ctx, "retry the CQ", func() error {
		if err := r.maybeRebaseCL(ctx); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.g.SendToCQ(ctx, r.ci, "CQ failed but there are no new commits. Retrying..."); err != nil {
			return skerr.Wrap(err)
		}
		r.issue.IsDryRun = false
		r.issue.Attempt++
		return nil
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) RetryDryRun(ctx context.Context) error {
	return r.withModify(ctx, "retry the CQ (dry run)", func() error {
		if err := r.maybeRebaseCL(ctx); err != nil {
			return skerr.Wrap(err)
		}
		if err := r.g.SendToDryRun(ctx, r.ci, "Dry run failed but there are no new commits. Retrying..."); err != nil {
			return skerr.Wrap(err)
		}
		r.issue.IsDryRun = true
		r.issue.Attempt++
		return nil
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) Update(ctx context.Context) error {
	alreadyClosed := r.IsClosed()
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
	if r.IsClosed() && !alreadyClosed && r.finishedCallback != nil {
		return r.finishedCallback(ctx, r)
	}
	return nil
}

// See documentation for state_machine.RollClImpl interface.
func (r *gerritRoll) Attempt() int {
	return r.issue.Attempt
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

	// Retrieve the pull request from github using exponential backoff.
	var pullRequest *github_api.PullRequest
	var err error
	getPullRequestFunc := func() error {
		pullRequest, err = g.GetPullRequest(int(a.Issue))
		return err
	}
	if err := backoff.Retry(getPullRequestFunc, GithubBackOffConfig); err != nil {
		return nil, skerr.Wrapf(err, "Failed to get pull request for %d", a.Issue)
	}

	// Get all checks for this pull request using exponential backoff.
	var checks []*github.Check
	getChecksFunc := func() error {
		checks, err = g.GetChecks(pullRequest.Head.GetSHA())
		return err
	}
	if err := backoff.Retry(getChecksFunc, GithubBackOffConfig); err != nil {
		return nil, skerr.Wrapf(err, "Failed to get checks for %d", a.Issue)
	}
	if a.IsDryRun {
		// Do not wait for any checks if it is a dry run.
		checksWaitFor = []string{}
	}
	// Convert checks to try results.
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
		// The roller should not be looking at github checks to determine when the dry run passes,
		// it should be relying on the project's "commit queue" to do this instead.
		// Flutter's merge bot does not handle dry runs yet. https://github.com/flutter/flutter/issues/61955
		// has been filed to add this functionality.
		//
		// Sometimes the github API does not return the correct number of checks, try to workaround
		// this by only looking at a PR if it is > GITHUB_PR_DURATION_FOR_CHECKS old (Flutter's merge
		// bot waits for 1 hour).
		if time.Since(pullRequest.GetCreatedAt()) > GitHubPRDurationForChecks {
			i.DryRunFinished = pullRequest.GetState() == github.CLOSED_STATE || pullRequest.GetMerged() || (len(i.TryResults) > 0 && i.AllTrybotsFinished())
			i.DryRunSuccess = pullRequest.GetMerged() || (i.DryRunFinished && i.AllTrybotsSucceeded())
		} else {
			i.DryRunFinished = false
			i.DryRunSuccess = false
		}
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
func newGithubRoll(ctx context.Context, issue *autoroll.AutoRollIssue, g *github.GitHub, recent *recent_rolls.RecentRolls, issueUrlBase string, config *config.GitHubConfig, rollingTo *revision.Revision, cb func(context.Context, RollImpl) error) (RollImpl, error) {
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
	alreadyClosed := r.IsClosed()
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
	if r.IsClosed() && !alreadyClosed && r.finishedCallback != nil {
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
func (r *githubRoll) IsCommitted() bool {
	return r.issue.Committed
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
		r.issue.Attempt++
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
		r.issue.Attempt++
		return nil
	})
}

// See documentation for state_machine.RollClImpl interface.
func (r *githubRoll) Attempt() int {
	return r.issue.Attempt
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) IssueID() string {
	return fmt.Sprintf("%d", r.issue.Issue)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *githubRoll) IssueURL() string {
	return r.issueUrl
}
