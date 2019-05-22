package state_machine

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/notifier"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/counters"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/state_machine"
)

/*
	State machine for the autoroller.
*/

const (
	// State names.
	S_NORMAL_IDLE                  = "idle"
	S_NORMAL_ACTIVE                = "active"
	S_NORMAL_SUCCESS               = "success"
	S_NORMAL_SUCCESS_THROTTLED     = "success throttled"
	S_NORMAL_FAILURE               = "failure"
	S_NORMAL_FAILURE_THROTTLED     = "failure throttled"
	S_NORMAL_SAFETY_THROTTLED      = "safety throttled"
	S_NORMAL_WAIT_FOR_WINDOW       = "waiting for roll window"
	S_DRY_RUN_IDLE                 = "dry run idle"
	S_DRY_RUN_ACTIVE               = "dry run active"
	S_DRY_RUN_SUCCESS              = "dry run success"
	S_DRY_RUN_SUCCESS_LEAVING_OPEN = "dry run success; leaving open"
	S_DRY_RUN_FAILURE              = "dry run failure"
	S_DRY_RUN_FAILURE_THROTTLED    = "dry run failure throttled"
	S_DRY_RUN_SAFETY_THROTTLED     = "dry run safety throttled"
	S_STOPPED                      = "stopped"

	// Transition function names.
	F_NOOP                    = "no-op"
	F_UPDATE_REPOS            = "update repos"
	F_UPLOAD_ROLL             = "upload roll"
	F_UPLOAD_DRY_RUN          = "upload dry run"
	F_UPDATE_ROLL             = "update roll"
	F_SWITCH_TO_DRY_RUN       = "switch roll to dry run"
	F_SWITCH_TO_NORMAL        = "switch roll to normal"
	F_CLOSE_FAILED            = "close roll (failed)"
	F_CLOSE_STOPPED           = "close roll (stopped)"
	F_CLOSE_DRY_RUN_FAILED    = "close roll (dry run failed)"
	F_CLOSE_DRY_RUN_OUTDATED  = "close roll (dry run outdated)"
	F_WAIT_FOR_LAND           = "wait for roll to land"
	F_RETRY_FAILED_NORMAL     = "retry failed roll"
	F_RETRY_FAILED_DRY_RUN    = "retry failed dry run"
	F_NOTIFY_FAILURE_THROTTLE = "notify failure throttled"
	F_NOTIFY_SAFETY_THROTTLE  = "notify safety throttled"

	// Maximum number of no-op transitions to perform at once. This is an
	// arbitrary limit just to keep us from performing an unbounded number
	// of transitions at a time.
	MAX_NOOP_TRANSITIONS = 10
)

// Interface for interacting with a single autoroll CL.
type RollCLImpl interface {
	// Add a comment to the CL.
	AddComment(context.Context, string) error

	// Close the CL. The first string argument is the result of the roll,
	// and the second is the message to add to the CL on closing.
	Close(context.Context, string, string) error

	// Return true iff the roll has finished (ie. succeeded or failed).
	IsFinished() bool

	// Return true iff the roll succeeded.
	IsSuccess() bool

	// Return true iff the dry run is finished.
	IsDryRunFinished() bool

	// Return true iff the dry run succeeded.
	IsDryRunSuccess() bool

	// Return the issue ID of the roll.
	IssueID() string

	// Return the URL of the roll.
	IssueURL() string

	// Retry the CQ in the case of a failure.
	RetryCQ(context.Context) error

	// Retry a dry run in the case of a failure.
	RetryDryRun(context.Context) error

	// The revision this roll is rolling to.
	RollingTo() string

	// Set the dry run bit on the CL.
	SwitchToDryRun(context.Context) error

	// Set the full CQ bit on the CL.
	SwitchToNormal(context.Context) error

	// Update our local copy of the CL from the codereview server.
	Update(context.Context) error
}

// Interface for interacting with the other elements of an autoroller.
type AutoRollerImpl interface {
	// Return a Throttler indicating that we have failed to roll too many
	// times within a time period.
	FailureThrottle() *Throttler

	// Return the currently-active roll. May be nil if no roll exists.
	GetActiveRoll() RollCLImpl

	// Return the currently-rolled revision of the sub-project.
	GetCurrentRev() string

	// Return the next revision of the sub-project which we want to roll.
	// This is the same as GetCurrentRev when the sub-project is up-to-date.
	GetNextRollRev() string

	// Return the current mode of the AutoRoller.
	GetMode() string

	// InRollWindow returns true iff the roller is inside the configured
	// time window in which it is allowed to roll.
	InRollWindow(time.Time) bool

	// Return true if we have already rolled past the given revision.
	RolledPast(context.Context, string) (bool, error)

	// Return a Throttler indicating that we have attempted to upload too
	// many CLs within a time period.
	SafetyThrottle() *Throttler

	// Return a Throttler indicating whether we have successfully rolled too
	// many times within a time period.
	SuccessThrottle() *Throttler

	// Update the project and sub-project repos.
	UpdateRepos(context.Context) error

	// Upload a new roll. AutoRollerImpl should track the created roll.
	UploadNewRoll(ctx context.Context, from, to string, dryRun bool) error
}

// AutoRollStateMachine is a StateMachine for the AutoRoller.
type AutoRollStateMachine struct {
	a AutoRollerImpl
	s *state_machine.StateMachine
}

// Throttler determines whether we should be throttled.
type Throttler struct {
	c         *counters.PersistentAutoDecrementCounter
	period    time.Duration
	threshold int64
}

// NewThrottler returns a Throttler instance.
func NewThrottler(ctx context.Context, gcsClient gcs.GCSClient, gcsPath string, period time.Duration, attempts int64) (*Throttler, error) {
	rv := &Throttler{
		period:    period,
		threshold: attempts,
	}
	if period > time.Duration(0) && attempts > 0 {
		c, err := counters.NewPersistentAutoDecrementCounter(ctx, gcsClient, gcsPath, period)
		if err != nil {
			return nil, err
		}
		rv.c = c
	}
	return rv, nil
}

// Inc increments the Throttler's counter.
func (t *Throttler) Inc(ctx context.Context) error {
	if t.c == nil {
		return nil
	}
	return t.c.Inc(ctx)
}

// IsThrottled returns true iff we should be throttled.
func (t *Throttler) IsThrottled() bool {
	if t.c == nil {
		return false
	}
	return t.c.Get() >= t.threshold
}

// Reset forcibly unthrottles the Throttler.
func (t *Throttler) Reset(ctx context.Context) error {
	if t.c == nil {
		return nil
	}
	return t.c.Reset(ctx)
}

// ThrottledUntil returns the approximate time when the Throttler will no longer
// be throttled.
func (t *Throttler) ThrottledUntil() time.Time {
	if t.c == nil {
		return time.Time{}
	}
	times := t.c.GetDecrementTimes()
	if int64(len(times)) < t.threshold {
		return time.Time{}
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})
	idx := int64(len(times)) - t.threshold
	return times[idx]
}

// New returns a StateMachine for the autoroller.
func New(ctx context.Context, impl AutoRollerImpl, n *notifier.AutoRollNotifier, gcsClient gcs.GCSClient, gcsPrefix string) (*AutoRollStateMachine, error) {
	s := &AutoRollStateMachine{
		a: impl,
		s: nil, // Filled in later.
	}

	b := state_machine.NewBuilder()

	// Named callback functions.
	b.F(F_NOOP, nil)
	b.F(F_UPDATE_REPOS, func(ctx context.Context) error {
		return s.a.UpdateRepos(ctx)
	})
	b.F(F_UPLOAD_ROLL, func(ctx context.Context) error {
		if err := s.a.SafetyThrottle().Inc(ctx); err != nil {
			return err
		}
		if err := s.a.UploadNewRoll(ctx, s.a.GetCurrentRev(), s.a.GetNextRollRev(), false); err != nil {
			n.SendRollCreationFailed(ctx, err)
			return err
		}
		roll := s.a.GetActiveRoll()
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), fmt.Sprintf("The roller has uploaded a new roll attempt: %s", roll.IssueURL()))
		return nil
	})
	b.F(F_UPLOAD_DRY_RUN, func(ctx context.Context) error {
		if err := s.a.SafetyThrottle().Inc(ctx); err != nil {
			return err
		}
		if err := s.a.UploadNewRoll(ctx, s.a.GetCurrentRev(), s.a.GetNextRollRev(), true); err != nil {
			n.SendRollCreationFailed(ctx, err)
			return err
		}
		roll := s.a.GetActiveRoll()
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), fmt.Sprintf("The roller has uploaded a new dry run attempt: %s", roll.IssueURL()))
		return nil
	})
	b.F(F_UPDATE_ROLL, func(ctx context.Context) error {
		if err := s.a.GetActiveRoll().Update(ctx); err != nil {
			return err
		}
		return s.a.UpdateRepos(ctx)
	})
	b.F(F_CLOSE_FAILED, func(ctx context.Context) error {
		roll := s.a.GetActiveRoll()
		if err := roll.Close(ctx, autoroll.ROLL_RESULT_FAILURE, fmt.Sprintf("Commit queue failed; closing this roll.")); err != nil {
			return err
		}
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), "This CL was abandoned because the commit queue failed and there are new commits to try.")
		return nil
	})
	b.F(F_CLOSE_STOPPED, func(ctx context.Context) error {
		roll := s.a.GetActiveRoll()
		if err := roll.Close(ctx, autoroll.ROLL_RESULT_FAILURE, fmt.Sprintf("AutoRoller is stopped; closing the active roll.")); err != nil {
			return err
		}
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), "This CL was abandoned because the AutoRoller was stopped.")
		return nil
	})
	b.F(F_CLOSE_DRY_RUN_FAILED, func(ctx context.Context) error {
		roll := s.a.GetActiveRoll()
		if err := roll.Close(ctx, autoroll.ROLL_RESULT_DRY_RUN_FAILURE, fmt.Sprintf("Commit queue failed; closing this roll.")); err != nil {
			return err
		}
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), "This CL was abandoned because the commit queue dry run failed and there are new commits to try.")
		return nil
	})
	b.F(F_CLOSE_DRY_RUN_OUTDATED, func(ctx context.Context) error {
		roll := s.a.GetActiveRoll()
		if err := roll.Close(ctx, autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, fmt.Sprintf("Repo has passed %s; will open a new dry run.", roll.RollingTo())); err != nil {
			return err
		}
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), "This CL was abandoned because one or more new commits have landed.")
		return nil
	})
	b.F(F_SWITCH_TO_DRY_RUN, func(ctx context.Context) error {
		roll := s.a.GetActiveRoll()
		if err := roll.SwitchToDryRun(ctx); err != nil {
			return err
		}
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), "This roll was switched to commit queue dry run mode.")
		return nil
	})
	b.F(F_SWITCH_TO_NORMAL, func(ctx context.Context) error {
		roll := s.a.GetActiveRoll()
		if err := roll.SwitchToNormal(ctx); err != nil {
			return err
		}
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), "This roll was switched to normal commit queue mode.")
		return nil
	})
	b.F(F_WAIT_FOR_LAND, func(ctx context.Context) error {
		sklog.Infof("Roll succeeded; syncing the repo until it lands.")
		currentRoll := s.a.GetActiveRoll()
		// If the server restarts during the loop below, we'll end up in this state
		// even though there is no active roll.
		if currentRoll == nil {
			sklog.Warningf("GetActiveRoll returned nil in state %q. Continuing transition under the assumption that the roll has landed.", F_WAIT_FOR_LAND)
			return nil
		}
		for {
			sklog.Infof("Syncing, looking for %s...", currentRoll.RollingTo())
			if err := s.a.UpdateRepos(ctx); err != nil {
				return err
			}
			rolledPast, err := s.a.RolledPast(ctx, currentRoll.RollingTo())
			if err != nil {
				return err
			}
			if rolledPast {
				break
			}
			time.Sleep(10 * time.Second)
		}
		n.SendIssueUpdate(ctx, currentRoll.IssueID(), currentRoll.IssueURL(), "This roll landed successfully.")
		successThrottle := s.a.SuccessThrottle()
		if successThrottle.IsThrottled() {
			n.SendSuccessThrottled(ctx, successThrottle.ThrottledUntil())
		}
		return nil
	})
	b.F(F_RETRY_FAILED_NORMAL, func(ctx context.Context) error {
		// TODO(borenet): The CQ will fail forever in the case of a
		// merge conflict; we should really patch in the CL, rebase and
		// upload again.
		roll := s.a.GetActiveRoll()
		if err := roll.RetryCQ(ctx); err != nil {
			return err
		}
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), "Retrying the commit queue on this CL because there are no new commits.")
		return nil
	})
	b.F(F_RETRY_FAILED_DRY_RUN, func(ctx context.Context) error {
		sklog.Infof("Dry run failed but no new commits; retrying CQ.")
		// TODO(borenet): The CQ will fail forever in the case of a
		// merge conflict; we should really patch in the CL, rebase and
		// upload again.
		roll := s.a.GetActiveRoll()
		if err := roll.RetryDryRun(ctx); err != nil {
			return err
		}
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), "Retrying the CQ dry run on this CL because there are no new commits.")
		return nil
	})
	b.F(F_NOTIFY_FAILURE_THROTTLE, func(ctx context.Context) error {
		roll := s.a.GetActiveRoll()
		until := s.a.FailureThrottle().ThrottledUntil()
		n.SendIssueUpdate(ctx, roll.IssueID(), roll.IssueURL(), fmt.Sprintf("The commit queue failed on this CL but no new commits have landed in the repo. Will retry the CQ at %s if no new commits land.", until))
		return nil
	})
	b.F(F_NOTIFY_SAFETY_THROTTLE, func(ctx context.Context) error {
		n.SendSafetyThrottled(ctx, s.a.SafetyThrottle().ThrottledUntil())
		return nil
	})

	// States and transitions.

	// Stopped state.
	b.T(S_STOPPED, S_STOPPED, F_UPDATE_REPOS)
	b.T(S_STOPPED, S_NORMAL_IDLE, F_NOOP)
	b.T(S_STOPPED, S_DRY_RUN_IDLE, F_NOOP)

	// Normal states.
	b.T(S_NORMAL_IDLE, S_STOPPED, F_NOOP)
	b.T(S_NORMAL_IDLE, S_NORMAL_IDLE, F_UPDATE_REPOS)
	b.T(S_NORMAL_IDLE, S_DRY_RUN_IDLE, F_NOOP)
	b.T(S_NORMAL_IDLE, S_NORMAL_SAFETY_THROTTLED, F_NOTIFY_SAFETY_THROTTLE)
	b.T(S_NORMAL_IDLE, S_NORMAL_SUCCESS_THROTTLED, F_NOOP)
	b.T(S_NORMAL_IDLE, S_NORMAL_ACTIVE, F_UPLOAD_ROLL)
	b.T(S_NORMAL_IDLE, S_NORMAL_WAIT_FOR_WINDOW, F_NOOP)
	b.T(S_NORMAL_ACTIVE, S_NORMAL_ACTIVE, F_UPDATE_ROLL)
	b.T(S_NORMAL_ACTIVE, S_DRY_RUN_ACTIVE, F_SWITCH_TO_DRY_RUN)
	b.T(S_NORMAL_ACTIVE, S_NORMAL_SUCCESS, F_NOOP)
	b.T(S_NORMAL_ACTIVE, S_NORMAL_FAILURE, F_NOOP)
	b.T(S_NORMAL_ACTIVE, S_STOPPED, F_CLOSE_STOPPED)
	b.T(S_NORMAL_SUCCESS, S_NORMAL_IDLE, F_WAIT_FOR_LAND)
	b.T(S_NORMAL_SUCCESS, S_NORMAL_SUCCESS_THROTTLED, F_WAIT_FOR_LAND)
	b.T(S_NORMAL_SUCCESS_THROTTLED, S_NORMAL_SUCCESS_THROTTLED, F_UPDATE_REPOS)
	b.T(S_NORMAL_SUCCESS_THROTTLED, S_NORMAL_IDLE, F_NOOP)
	b.T(S_NORMAL_SUCCESS_THROTTLED, S_DRY_RUN_IDLE, F_NOOP)
	b.T(S_NORMAL_SUCCESS_THROTTLED, S_STOPPED, F_NOOP)
	b.T(S_NORMAL_FAILURE, S_NORMAL_IDLE, F_CLOSE_FAILED)
	b.T(S_NORMAL_FAILURE, S_NORMAL_FAILURE_THROTTLED, F_NOTIFY_FAILURE_THROTTLE)
	b.T(S_NORMAL_FAILURE_THROTTLED, S_NORMAL_FAILURE_THROTTLED, F_UPDATE_ROLL)
	b.T(S_NORMAL_FAILURE_THROTTLED, S_NORMAL_SUCCESS, F_NOOP)
	b.T(S_NORMAL_FAILURE_THROTTLED, S_NORMAL_ACTIVE, F_RETRY_FAILED_NORMAL)
	b.T(S_NORMAL_FAILURE_THROTTLED, S_DRY_RUN_ACTIVE, F_SWITCH_TO_DRY_RUN)
	b.T(S_NORMAL_FAILURE_THROTTLED, S_NORMAL_IDLE, F_CLOSE_FAILED)
	b.T(S_NORMAL_FAILURE_THROTTLED, S_STOPPED, F_CLOSE_STOPPED)
	b.T(S_NORMAL_SAFETY_THROTTLED, S_NORMAL_IDLE, F_NOOP)
	b.T(S_NORMAL_SAFETY_THROTTLED, S_NORMAL_SAFETY_THROTTLED, F_UPDATE_REPOS)
	b.T(S_NORMAL_WAIT_FOR_WINDOW, S_NORMAL_WAIT_FOR_WINDOW, F_UPDATE_REPOS)
	b.T(S_NORMAL_WAIT_FOR_WINDOW, S_NORMAL_IDLE, F_NOOP)

	// Dry run states.
	b.T(S_DRY_RUN_IDLE, S_STOPPED, F_NOOP)
	b.T(S_DRY_RUN_IDLE, S_DRY_RUN_IDLE, F_UPDATE_REPOS)
	b.T(S_DRY_RUN_IDLE, S_NORMAL_IDLE, F_NOOP)
	b.T(S_DRY_RUN_IDLE, S_NORMAL_SUCCESS_THROTTLED, F_NOOP)
	b.T(S_DRY_RUN_IDLE, S_DRY_RUN_SAFETY_THROTTLED, F_NOTIFY_SAFETY_THROTTLE)
	b.T(S_DRY_RUN_IDLE, S_DRY_RUN_ACTIVE, F_UPLOAD_DRY_RUN)
	b.T(S_DRY_RUN_ACTIVE, S_DRY_RUN_ACTIVE, F_UPDATE_ROLL)
	b.T(S_DRY_RUN_ACTIVE, S_NORMAL_ACTIVE, F_SWITCH_TO_NORMAL)
	b.T(S_DRY_RUN_ACTIVE, S_DRY_RUN_SUCCESS, F_NOOP)
	b.T(S_DRY_RUN_ACTIVE, S_DRY_RUN_FAILURE, F_NOOP)
	b.T(S_DRY_RUN_ACTIVE, S_STOPPED, F_CLOSE_STOPPED)
	b.T(S_DRY_RUN_SUCCESS, S_DRY_RUN_IDLE, F_CLOSE_DRY_RUN_OUTDATED)
	b.T(S_DRY_RUN_SUCCESS, S_DRY_RUN_SUCCESS_LEAVING_OPEN, F_NOOP)
	b.T(S_DRY_RUN_SUCCESS_LEAVING_OPEN, S_DRY_RUN_SUCCESS_LEAVING_OPEN, F_UPDATE_REPOS)
	b.T(S_DRY_RUN_SUCCESS_LEAVING_OPEN, S_NORMAL_ACTIVE, F_SWITCH_TO_NORMAL)
	b.T(S_DRY_RUN_SUCCESS_LEAVING_OPEN, S_STOPPED, F_CLOSE_STOPPED)
	b.T(S_DRY_RUN_SUCCESS_LEAVING_OPEN, S_DRY_RUN_IDLE, F_CLOSE_DRY_RUN_OUTDATED)
	b.T(S_DRY_RUN_FAILURE, S_DRY_RUN_IDLE, F_CLOSE_DRY_RUN_FAILED)
	b.T(S_DRY_RUN_FAILURE, S_DRY_RUN_FAILURE_THROTTLED, F_NOTIFY_FAILURE_THROTTLE)
	b.T(S_DRY_RUN_FAILURE_THROTTLED, S_DRY_RUN_FAILURE_THROTTLED, F_UPDATE_ROLL)
	b.T(S_DRY_RUN_FAILURE_THROTTLED, S_DRY_RUN_IDLE, F_NOOP)
	b.T(S_DRY_RUN_FAILURE_THROTTLED, S_DRY_RUN_ACTIVE, F_RETRY_FAILED_DRY_RUN)
	b.T(S_DRY_RUN_FAILURE_THROTTLED, S_DRY_RUN_FAILURE, F_CLOSE_DRY_RUN_FAILED)
	b.T(S_DRY_RUN_FAILURE_THROTTLED, S_NORMAL_ACTIVE, F_SWITCH_TO_NORMAL)
	b.T(S_DRY_RUN_FAILURE_THROTTLED, S_STOPPED, F_CLOSE_STOPPED)
	b.T(S_DRY_RUN_SAFETY_THROTTLED, S_DRY_RUN_IDLE, F_NOOP)
	b.T(S_DRY_RUN_SAFETY_THROTTLED, S_DRY_RUN_SAFETY_THROTTLED, F_UPDATE_REPOS)

	// Build the state machine.
	b.SetInitial(S_NORMAL_IDLE)
	sm, err := b.Build(ctx, gcsClient, gcsPrefix+"/state_machine")
	if err != nil {
		return nil, err
	}
	s.s = sm
	return s, nil
}

// Get the next state.
func (s *AutoRollStateMachine) GetNext(ctx context.Context) (string, error) {
	desiredMode := s.a.GetMode()
	switch state := s.s.Current(); state {
	case S_STOPPED:
		switch desiredMode {
		case modes.MODE_RUNNING:
			return S_NORMAL_IDLE, nil
		case modes.MODE_DRY_RUN:
			return S_DRY_RUN_IDLE, nil
		case modes.MODE_STOPPED:
			return S_STOPPED, nil
		default:
			return "", fmt.Errorf("Invalid mode: %q", desiredMode)
		}
	case S_NORMAL_IDLE:
		switch desiredMode {
		case modes.MODE_RUNNING:
			break
		case modes.MODE_DRY_RUN:
			return S_DRY_RUN_IDLE, nil
		case modes.MODE_STOPPED:
			return S_STOPPED, nil
		default:
			return "", fmt.Errorf("Invalid mode: %q", desiredMode)
		}
		if !s.a.InRollWindow(time.Now()) {
			return S_NORMAL_WAIT_FOR_WINDOW, nil
		}
		current := s.a.GetCurrentRev()
		next := s.a.GetNextRollRev()
		if current == next {
			return S_NORMAL_IDLE, nil
		} else if s.a.SafetyThrottle().IsThrottled() {
			return S_NORMAL_SAFETY_THROTTLED, nil
		} else if s.a.SuccessThrottle().IsThrottled() {
			return S_NORMAL_SUCCESS_THROTTLED, nil
		} else {
			return S_NORMAL_ACTIVE, nil
		}
	case S_NORMAL_ACTIVE:
		currentRoll := s.a.GetActiveRoll()
		if currentRoll.IsFinished() {
			if currentRoll.IsSuccess() {
				return S_NORMAL_SUCCESS, nil
			} else {
				return S_NORMAL_FAILURE, nil
			}
		} else {
			if desiredMode == modes.MODE_DRY_RUN {
				return S_DRY_RUN_ACTIVE, nil
			} else if desiredMode == modes.MODE_STOPPED {
				return S_STOPPED, nil
			} else if desiredMode == modes.MODE_RUNNING {
				return S_NORMAL_ACTIVE, nil
			} else {
				return "", fmt.Errorf("Invalid mode %q", desiredMode)
			}
		}
	case S_NORMAL_SUCCESS:
		throttle := s.a.SuccessThrottle()
		if err := throttle.Inc(ctx); err != nil {
			return "", err
		}
		if throttle.IsThrottled() {
			return S_NORMAL_SUCCESS_THROTTLED, nil
		}
		return S_NORMAL_IDLE, nil
	case S_NORMAL_SUCCESS_THROTTLED:
		if desiredMode == modes.MODE_DRY_RUN {
			return S_DRY_RUN_IDLE, nil
		} else if desiredMode == modes.MODE_STOPPED {
			return S_STOPPED, nil
		} else if s.a.SuccessThrottle().IsThrottled() {
			return S_NORMAL_SUCCESS_THROTTLED, nil
		}
		return S_NORMAL_IDLE, nil
	case S_NORMAL_FAILURE:
		throttle := s.a.FailureThrottle()
		if err := throttle.Inc(ctx); err != nil {
			return "", err
		}
		if s.a.GetNextRollRev() == s.a.GetActiveRoll().RollingTo() {
			// Rather than upload the same CL again, we'll try
			// running the CQ again after a period of throttling.
			if throttle.IsThrottled() {
				return S_NORMAL_FAILURE_THROTTLED, nil
			}
		}
		return S_NORMAL_IDLE, nil
	case S_NORMAL_FAILURE_THROTTLED:
		// The roll may have been manually submitted.
		currentRoll := s.a.GetActiveRoll()
		if currentRoll.IsFinished() && currentRoll.IsSuccess() {
			return S_NORMAL_SUCCESS, nil
		}
		if desiredMode == modes.MODE_STOPPED {
			return S_STOPPED, nil
		} else if s.a.GetNextRollRev() != s.a.GetActiveRoll().RollingTo() {
			return S_NORMAL_IDLE, nil
		} else if desiredMode == modes.MODE_DRY_RUN {
			return S_DRY_RUN_ACTIVE, nil
		} else if s.a.FailureThrottle().IsThrottled() {
			return S_NORMAL_FAILURE_THROTTLED, nil
		}
		return S_NORMAL_ACTIVE, nil
	case S_NORMAL_SAFETY_THROTTLED:
		if s.a.SafetyThrottle().IsThrottled() {
			return S_NORMAL_SAFETY_THROTTLED, nil
		}
		return S_NORMAL_IDLE, nil
	case S_NORMAL_WAIT_FOR_WINDOW:
		if s.a.InRollWindow(time.Now()) {
			return S_NORMAL_IDLE, nil
		}
		return S_NORMAL_WAIT_FOR_WINDOW, nil
	case S_DRY_RUN_IDLE:
		if desiredMode == modes.MODE_RUNNING {
			if s.a.SuccessThrottle().IsThrottled() {
				return S_NORMAL_SUCCESS_THROTTLED, nil
			}
			return S_NORMAL_IDLE, nil
		} else if desiredMode == modes.MODE_STOPPED {
			return S_STOPPED, nil
		} else if desiredMode != modes.MODE_DRY_RUN {
			return "", fmt.Errorf("Invalid mode %q", desiredMode)
		}
		current := s.a.GetCurrentRev()
		next := s.a.GetNextRollRev()
		if current == next {
			return S_DRY_RUN_IDLE, nil
		} else if s.a.SafetyThrottle().IsThrottled() {
			return S_DRY_RUN_SAFETY_THROTTLED, nil
		} else {
			return S_DRY_RUN_ACTIVE, nil
		}
	case S_DRY_RUN_ACTIVE:
		currentRoll := s.a.GetActiveRoll()
		if currentRoll.IsDryRunFinished() {
			if currentRoll.IsDryRunSuccess() {
				return S_DRY_RUN_SUCCESS, nil
			} else {
				return S_DRY_RUN_FAILURE, nil
			}
		} else {
			desiredMode := s.a.GetMode()
			if desiredMode == modes.MODE_RUNNING {
				return S_NORMAL_ACTIVE, nil
			} else if desiredMode == modes.MODE_STOPPED {
				return S_STOPPED, nil
			} else if desiredMode == modes.MODE_DRY_RUN {
				return S_DRY_RUN_ACTIVE, nil
			} else {
				return "", fmt.Errorf("Invalid mode %q", desiredMode)
			}
		}
	case S_DRY_RUN_SUCCESS:
		if s.a.GetNextRollRev() == s.a.GetActiveRoll().RollingTo() {
			// The current dry run is for the commit we want. Leave
			// it open so we can land it if we want.
			return S_DRY_RUN_SUCCESS_LEAVING_OPEN, nil
		}
		return S_DRY_RUN_IDLE, nil
	case S_DRY_RUN_SUCCESS_LEAVING_OPEN:
		if desiredMode == modes.MODE_RUNNING {
			return S_NORMAL_ACTIVE, nil
		} else if desiredMode == modes.MODE_STOPPED {
			return S_STOPPED, nil
		} else if desiredMode != modes.MODE_DRY_RUN {
			return "", fmt.Errorf("Invalid mode %q", desiredMode)
		}

		if s.a.GetNextRollRev() == s.a.GetActiveRoll().RollingTo() {
			// The current dry run is for the commit we want. Leave
			// it open so we can land it if we want.
			return S_DRY_RUN_SUCCESS_LEAVING_OPEN, nil
		}
		return S_DRY_RUN_IDLE, nil
	case S_DRY_RUN_FAILURE:
		if err := s.a.FailureThrottle().Inc(ctx); err != nil {
			return "", err
		}
		if s.a.GetNextRollRev() == s.a.GetActiveRoll().RollingTo() {
			// Rather than upload the same CL again, we'll try
			// running the CQ again after a period of throttling.
			return S_DRY_RUN_FAILURE_THROTTLED, nil
		}
		return S_DRY_RUN_IDLE, nil
	case S_DRY_RUN_FAILURE_THROTTLED:
		// The roll may have been manually submitted.
		currentRoll := s.a.GetActiveRoll()
		if currentRoll.IsFinished() && currentRoll.IsSuccess() {
			return S_DRY_RUN_IDLE, nil
		}
		if desiredMode == modes.MODE_STOPPED {
			return S_STOPPED, nil
		} else if s.a.GetNextRollRev() != s.a.GetActiveRoll().RollingTo() {
			return S_DRY_RUN_FAILURE, nil
		} else if desiredMode == modes.MODE_RUNNING {
			return S_NORMAL_ACTIVE, nil
		} else if s.a.FailureThrottle().IsThrottled() {
			return S_DRY_RUN_FAILURE_THROTTLED, nil
		}
		return S_DRY_RUN_ACTIVE, nil
	case S_DRY_RUN_SAFETY_THROTTLED:
		if s.a.SafetyThrottle().IsThrottled() {
			return S_DRY_RUN_SAFETY_THROTTLED, nil
		}
		return S_DRY_RUN_IDLE, nil
	default:
		return "", fmt.Errorf("Invalid state %q", state)
	}
}

// Attempt to perform the given state transition.
func (s *AutoRollStateMachine) Transition(ctx context.Context, dest string) error {
	fName, err := s.s.GetTransitionName(dest)
	if err != nil {
		return err
	}
	sklog.Infof("Attempting to perform transition from %q to %q: %s", s.s.Current(), dest, fName)
	if err := s.s.Transition(exec.NoInterruptContext(ctx), dest); err != nil {
		return err
	}
	sklog.Infof("Successfully performed transition.")
	return nil
}

// Attempt to perform the next state transition.
func (s *AutoRollStateMachine) NextTransition(ctx context.Context) error {
	next, err := s.GetNext(ctx)
	if err != nil {
		return err
	}
	return s.Transition(ctx, next)
}

// Perform the next state transition, plus any subsequent transitions which are
// no-ops.
func (s *AutoRollStateMachine) NextTransitionSequence(ctx context.Context) error {
	if err := s.NextTransition(ctx); err != nil {
		return err
	}
	// Greedily perform transitions until we reach a transition which is not
	// a no-op, or until we've performed a maximum number of transitions, to
	// keep us from accidentally looping extremely quickly.
	for i := 0; i < MAX_NOOP_TRANSITIONS; i++ {
		next, err := s.GetNext(ctx)
		if err != nil {
			return err
		}
		fName, err := s.s.GetTransitionName(next)
		if err != nil {
			return err
		} else if fName == F_NOOP {
			if err := s.Transition(ctx, next); err != nil {
				return err
			}
		} else {
			return nil
		}
	}
	// If we hit the maximum number of no-op transitions, there's probably
	// a bug in the state machine. Log an error but don't return it, so as
	// not to disrupt normal operation.
	sklog.Errorf("Performed %d no-op transitions in a single tick; is there a bug in the state machine?", MAX_NOOP_TRANSITIONS)
	return nil
}

// Return the current state.
func (s *AutoRollStateMachine) Current() string {
	return s.s.Current()
}
