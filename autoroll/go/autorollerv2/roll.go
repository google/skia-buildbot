package autorollerv2

import (
	"fmt"

	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
)

type RollImpl interface {
	state_machine.RollCLImpl

	// Insert the roll into the DB.
	InsertIntoDB() error
}

func retrieveGerritIssue(g *gerrit.Gerrit, rm repo_manager.RepoManager, rollIntoAndroid bool, issueNum int64) (*gerrit.ChangeInfo, *autoroll.AutoRollIssue, error) {
	info, err := g.GetIssueProperties(issueNum)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get issue properties: %s", err)
	}
	a, err := autoroll.FromGerritChangeInfo(info, rm.FullChildHash, rollIntoAndroid)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to convert issue format: %s", err)
	}
	tryResults, err := autoroll.GetTryResultsFromGerrit(g, a)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to retrieve try results: %s", err)
	}
	a.TryResults = tryResults
	return info, a, nil
}

// gerritRoll is an implementation of RollImpl.
type gerritRoll struct {
	ci     *gerrit.ChangeInfo
	issue  *autoroll.AutoRollIssue
	g      *gerrit.Gerrit
	recent *recent_rolls.RecentRolls
	rm     repo_manager.RepoManager
}

// newGerritRoll obtains a gerritRoll instance from the given Gerrit issue
// number.
func newGerritRoll(g *gerrit.Gerrit, rm repo_manager.RepoManager, recent *recent_rolls.RecentRolls, issueNum int64) (RollImpl, error) {
	ci, issue, err := retrieveGerritIssue(g, rm, false, issueNum)
	if err != nil {
		return nil, err
	}
	return &gerritRoll{
		ci:     ci,
		issue:  issue,
		g:      g,
		recent: recent,
		rm:     rm,
	}, nil
}

// See documentation for RollImpl interface.
func (r *gerritRoll) InsertIntoDB() error {
	return r.recent.Add(r.issue)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) AddComment(msg string) error {
	return r.g.AddComment(r.ci, msg)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) Close(result, msg string) error {
	sklog.Infof("Closing issue %d (result %q) with message: %s", r.ci.Issue, result, msg)
	if err := r.g.Abandon(r.ci, msg); err != nil {
		return err
	}
	return r.Update()
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IsFinished() bool {
	return r.ci.IsClosed() || !r.issue.CommitQueue
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IsSuccess() bool {
	return r.ci.Status == gerrit.CHANGE_STATUS_MERGED
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IsDryRunFinished() bool {
	// The CQ removes the CQ+1 label when the dry run finishes, regardless
	// of success or failure. Since we uploaded with the dry run label set,
	// we know the roll is in progress if the label is still set, and done
	// otherwise.
	return !r.issue.CommitQueueDryRun
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) IsDryRunSuccess() bool {
	return r.issue.AllTrybotsSucceeded()
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) RollingTo() string {
	return r.issue.RollingTo
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) SwitchToDryRun() error {
	if err := r.g.SendToDryRun(r.ci, ""); err != nil {
		return err
	}
	return r.Update()
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) SwitchToNormal() error {
	if err := r.g.SendToCQ(r.ci, ""); err != nil {
		return err
	}
	return r.Update()
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) Update() error {
	ci, issue, err := retrieveGerritIssue(r.g, r.rm, false, r.issue.Issue)
	if err != nil {
		return nil
	}
	r.ci = ci
	r.issue = issue
	return r.recent.Update(r.issue)
}

// Special type for Android rolls.
type gerritAndroidRoll struct {
	*gerritRoll
}

// newGerritAndroidRoll obtains a gerritAndroidRoll instance from the given Gerrit issue number.
func newGerritAndroidRoll(g *gerrit.Gerrit, rm repo_manager.RepoManager, recent *recent_rolls.RecentRolls, issueNum int64) (RollImpl, error) {
	ci, issue, err := retrieveGerritIssue(g, rm, true, issueNum)
	if err != nil {
		return nil, err
	}
	return &gerritAndroidRoll{&gerritRoll{
		ci:     ci,
		issue:  issue,
		g:      g,
		recent: recent,
		rm:     rm,
	}}, nil
}

// See documentation for RollImpl interface.
func (r *gerritAndroidRoll) InsertIntoDB() error {
	return r.recent.Add(r.issue)
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritAndroidRoll) IsDryRunFinished() bool {
	if _, ok := r.ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL]; ok {
		for _, lb := range r.ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL].All {
			if lb.Value != 0 {
				return true
			}
		}
	}
	return false
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritAndroidRoll) IsDryRunSuccess() bool {
	for _, lb := range r.ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL].All {
		if lb.Value == -1 {
			return false
		}
	}
	return true
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritAndroidRoll) SwitchToDryRun() error {
	if err := r.g.SetReview(r.ci, "Mode was changed to dry run", map[string]interface{}{gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_NONE}); err != nil {
		return err
	}
	return r.Update()
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritAndroidRoll) SwitchToNormal() error {
	if err := r.g.SetReview(r.ci, "Mode was changed to normal", map[string]interface{}{gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_SUBMIT}); err != nil {
		return err
	}
	return r.Update()
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritAndroidRoll) Update() error {
	ci, issue, err := retrieveGerritIssue(r.g, r.rm, true, r.issue.Issue)
	if err != nil {
		return nil
	}
	r.ci = ci
	r.issue = issue
	return r.recent.Update(r.issue)
}
