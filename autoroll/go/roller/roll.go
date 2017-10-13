package roller

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
	ci           *gerrit.ChangeInfo
	issue        *autoroll.AutoRollIssue
	g            *gerrit.Gerrit
	recent       *recent_rolls.RecentRolls
	retrieveRoll func(int64) (*gerrit.ChangeInfo, *autoroll.AutoRollIssue, error)
	result       string
	rm           repo_manager.RepoManager
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
		retrieveRoll: func(issueNum int64) (*gerrit.ChangeInfo, *autoroll.AutoRollIssue, error) {
			return retrieveGerritIssue(g, rm, false, issueNum)
		},
		rm: rm,
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

// Helper function for modifying a roll CL which might fail due to the CL being
// closed by a human or some other process, in which case we don't want to error
// out.
func (r *gerritRoll) withModify(action string, fn func() error) error {
	if err := fn(); err != nil {
		// It's possible that somebody abandoned the CL (or the CL
		// landed) while we were working. If that's the case, log an
		// error and move on.
		if err2 := r.Update(); err2 != nil {
			return fmt.Errorf("Failed to %s with error:\n%s\nAnd failed to update it with error:\n%s", action, err, err2)
		}
		if r.ci.IsClosed() {
			sklog.Errorf("Attempted to %s but it is already closed! Error: %s", action, err)
			return nil
		}
		return err
	}
	return r.Update()
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) Close(result, msg string) error {
	sklog.Infof("Closing issue %d (result %q) with message: %s", r.ci.Issue, result, msg)
	r.result = result
	return r.withModify("close the CL", func() error {
		return r.g.Abandon(r.ci, msg)
	})
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
	return r.withModify("switch the CL to dry run", func() error {
		return r.g.SendToDryRun(r.ci, "Mode was changed to dry run")
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) SwitchToNormal() error {
	return r.withModify("switch the CL out of dry run", func() error {
		return r.g.SendToCQ(r.ci, "Mode was changed to normal")
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritRoll) Update() error {
	ci, issue, err := r.retrieveRoll(r.issue.Issue)
	if err != nil {
		return err
	}
	r.ci = ci
	if r.result != "" {
		issue.Result = r.result
	}
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
		retrieveRoll: func(issueNum int64) (*gerrit.ChangeInfo, *autoroll.AutoRollIssue, error) {
			return retrieveGerritIssue(g, rm, true, issueNum)
		},
		rm: rm,
	}}, nil
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritAndroidRoll) IsDryRunFinished() bool {
	if _, ok := r.ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL]; ok {
		for _, lb := range r.ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL].All {
			if lb.Value != gerrit.PRESUBMIT_VERIFIED_LABEL_RUNNING {
				return true
			}
		}
	}
	return false
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritAndroidRoll) IsDryRunSuccess() bool {
	presubmit, ok := r.ci.Labels[gerrit.PRESUBMIT_VERIFIED_LABEL]
	if !ok || len(presubmit.All) == 0 {
		// Not done yet.
		return false
	}
	for _, lb := range presubmit.All {
		if lb.Value == gerrit.PRESUBMIT_VERIFIED_LABEL_ACCEPTED {
			return true
		}
	}
	return false
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritAndroidRoll) SwitchToDryRun() error {
	return r.withModify("switch the CL to dry run", func() error {
		return r.g.SetReview(r.ci, "Mode was changed to dry run", map[string]interface{}{gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_NONE})
	})
}

// See documentation for state_machine.RollCLImpl interface.
func (r *gerritAndroidRoll) SwitchToNormal() error {
	return r.withModify("switch the CL out of dry run", func() error {
		return r.g.SetReview(r.ci, "Mode was changed to normal", map[string]interface{}{gerrit.AUTOSUBMIT_LABEL: gerrit.AUTOSUBMIT_LABEL_SUBMIT})
	})
}
