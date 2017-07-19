package state_machine

import (
	"fmt"
	"path"
	"time"

	"go.skia.org/infra/autoroll/go/autoroll_modes"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/state_machine"
	"go.skia.org/infra/go/util"
)

/*
	State machine for the autoroller.
*/

const (
	// Throttling parameters.
	ROLL_ATTEMPT_THROTTLE_TIME = 30 * time.Minute
	ROLL_ATTEMPT_THROTTLE_NUM  = 3

	// State names.
	S_NORMAL_IDLE                  = "idle"
	S_NORMAL_ACTIVE                = "active"
	S_NORMAL_SUCCESS               = "success"
	S_NORMAL_FAILURE               = "failure"
	S_NORMAL_THROTTLED             = "throttled"
	S_DRY_RUN_IDLE                 = "dry run idle"
	S_DRY_RUN_ACTIVE               = "dry run active"
	S_DRY_RUN_SUCCESS              = "dry run success"
	S_DRY_RUN_SUCCESS_LEAVING_OPEN = "dry run success; leaving open"
	S_DRY_RUN_FAILURE              = "dry run failure"
	S_DRY_RUN_THROTTLED            = "dry run throttled"
	S_STOPPED                      = "stopped"

	// Transition function names.
	F_NOOP                   = "no-op"
	F_UPDATE_REPOS           = "update repos"
	F_UPLOAD_ROLL            = "upload roll"
	F_UPLOAD_DRY_RUN         = "upload dry run"
	F_UPDATE_ROLL            = "update roll"
	F_STOPPED_WAIT           = "waiting (stopped)"
	F_SWITCH_TO_DRY_RUN      = "switch roll to dry run"
	F_SWITCH_TO_NORMAL       = "switch roll to normal"
	F_CLOSE_FAILED           = "close roll (failed)"
	F_CLOSE_STOPPED          = "close roll (stopped)"
	F_CLOSE_DRY_RUN_FAILED   = "close roll (dry run failed)"
	F_CLOSE_DRY_RUN_OUTDATED = "close roll (dry run outdated)"
	F_THROTTLE_WAIT          = "waiting (throttled)"
	F_WAIT_FOR_LAND          = "wait for roll to land"

	// Maximum number of no-op transitions to perform at once. This is an
	// arbitrary limit just to keep us from performing an unbounded number
	// of transitions at a time.
	MAX_NOOP_TRANSITIONS = 10
)

// Interface for interacting with a single autoroll CL.
type RollCLImpl interface {
	// Add a comment to the CL.
	AddComment(string) error

	// Close the CL. The first string argument is the result of the roll,
	// and the second is the message to add to the CL on closing.
	Close(string, string) error

	// Return true iff the roll has finished (ie. succeeded or failed).
	IsFinished() bool

	// Return true iff the roll succeeded.
	IsSuccess() bool

	// Return true iff the dry run is finished.
	IsDryRunFinished() bool

	// Return true iff the dry run succeeded.
	IsDryRunSuccess() bool

	// The revision this roll is rolling to.
	RollingTo() string

	// Set the dry run bit on the CL.
	SwitchToDryRun() error

	// Set the full CQ bit on the CL.
	SwitchToNormal() error

	// Update our local copy of the CL from the codereview server.
	Update() error
}

// Interface for interacting with the other elements of an autoroller.
type AutoRollerImpl interface {
	// Upload a new roll. AutoRollerImpl should track the created roll.
	UploadNewRoll(from, to string, dryRun bool) error

	// Return the currently-active roll. May be nil if no roll exists.
	GetActiveRoll() RollCLImpl

	// Return the currently-rolled revision of the sub-project.
	GetCurrentRev() string

	// Return the next revision of the sub-project which we want to roll.
	// This is the same as GetCurrentRev when the sub-project is up-to-date.
	GetNextRollRev() string

	// Return the current mode of the AutoRoller.
	GetMode() string

	// Return true if we have already rolled past the given revision.
	RolledPast(string) (bool, error)

	// Update the project and sub-project repos.
	UpdateRepos() error
}

// AutoRollStateMachine is a StateMachine for the AutoRoller.
type AutoRollStateMachine struct {
	a AutoRollerImpl
	c *util.PersistentAutoDecrementCounter
	s *state_machine.StateMachine
}

// New returns a StateMachine for the autoroller.
func New(impl AutoRollerImpl, workdir string) (*AutoRollStateMachine, error) {
	// Global state.
	attemptCounter, err := util.NewPersistentAutoDecrementCounter(path.Join(workdir, "attempt_counter"), ROLL_ATTEMPT_THROTTLE_TIME)
	if err != nil {
		return nil, err
	}

	s := &AutoRollStateMachine{
		a: impl,
		c: attemptCounter,
		s: nil, // Filled in later.
	}

	b := state_machine.NewBuilder()

	// Named callback functions.
	b.F(F_NOOP, nil)
	b.F(F_UPDATE_REPOS, func() error {
		return s.a.UpdateRepos()
	})
	b.F(F_UPLOAD_ROLL, func() error {
		if err := s.c.Inc(); err != nil {
			return err
		}
		return s.a.UploadNewRoll(s.a.GetCurrentRev(), s.a.GetNextRollRev(), false)
	})
	b.F(F_UPLOAD_DRY_RUN, func() error {
		if err := s.c.Inc(); err != nil {
			return err
		}
		return s.a.UploadNewRoll(s.a.GetCurrentRev(), s.a.GetNextRollRev(), true)
	})
	b.F(F_UPDATE_ROLL, func() error {
		return s.a.GetActiveRoll().Update()
	})
	b.F(F_CLOSE_FAILED, func() error {
		return s.a.GetActiveRoll().Close(autoroll.ROLL_RESULT_FAILURE, fmt.Sprintf("Commit queue failed; closing this roll."))
	})
	b.F(F_CLOSE_STOPPED, func() error {
		return s.a.GetActiveRoll().Close(autoroll.ROLL_RESULT_FAILURE, fmt.Sprintf("AutoRoller is stopped; closing the active roll."))
	})
	b.F(F_CLOSE_DRY_RUN_FAILED, func() error {
		return s.a.GetActiveRoll().Close(autoroll.ROLL_RESULT_DRY_RUN_FAILURE, fmt.Sprintf("Commit queue failed; closing this roll."))
	})
	b.F(F_CLOSE_DRY_RUN_OUTDATED, func() error {
		currentRoll := s.a.GetActiveRoll()
		return currentRoll.Close(autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, fmt.Sprintf("Repo has passed %s; will open a new dry run.", currentRoll.RollingTo()))
	})
	b.F(F_STOPPED_WAIT, nil)
	b.F(F_SWITCH_TO_DRY_RUN, func() error {
		return s.a.GetActiveRoll().SwitchToDryRun()
	})
	b.F(F_SWITCH_TO_NORMAL, func() error {
		return s.a.GetActiveRoll().SwitchToNormal()
	})
	b.F(F_THROTTLE_WAIT, nil)
	b.F(F_WAIT_FOR_LAND, func() error {
		sklog.Infof("Roll succeeded; syncing the repo until it lands.")
		currentRoll := s.a.GetActiveRoll()
		for {
			sklog.Infof("Syncing, looking for %s...", currentRoll.RollingTo())
			if err := s.a.UpdateRepos(); err != nil {
				return err
			}
			rolledPast, err := s.a.RolledPast(currentRoll.RollingTo())
			if err != nil {
				return err
			}
			if rolledPast {
				break
			}
			time.Sleep(10 * time.Second)
		}
		return nil
	})

	// States and transitions.

	// Stopped state.
	b.T(S_STOPPED, S_STOPPED, F_STOPPED_WAIT)
	b.T(S_STOPPED, S_NORMAL_IDLE, F_NOOP)
	b.T(S_STOPPED, S_DRY_RUN_IDLE, F_NOOP)

	// Normal states.
	b.T(S_NORMAL_IDLE, S_STOPPED, F_NOOP)
	b.T(S_NORMAL_IDLE, S_NORMAL_IDLE, F_UPDATE_REPOS)
	b.T(S_NORMAL_IDLE, S_DRY_RUN_IDLE, F_NOOP)
	b.T(S_NORMAL_IDLE, S_NORMAL_THROTTLED, F_NOOP)
	b.T(S_NORMAL_IDLE, S_NORMAL_ACTIVE, F_UPLOAD_ROLL)
	b.T(S_NORMAL_ACTIVE, S_NORMAL_ACTIVE, F_UPDATE_ROLL)
	b.T(S_NORMAL_ACTIVE, S_DRY_RUN_ACTIVE, F_SWITCH_TO_DRY_RUN)
	b.T(S_NORMAL_ACTIVE, S_NORMAL_SUCCESS, F_NOOP)
	b.T(S_NORMAL_ACTIVE, S_NORMAL_FAILURE, F_NOOP)
	b.T(S_NORMAL_ACTIVE, S_STOPPED, F_CLOSE_STOPPED)
	b.T(S_NORMAL_SUCCESS, S_NORMAL_IDLE, F_WAIT_FOR_LAND)
	b.T(S_NORMAL_FAILURE, S_NORMAL_IDLE, F_CLOSE_FAILED)
	b.T(S_NORMAL_THROTTLED, S_NORMAL_IDLE, F_NOOP)
	b.T(S_NORMAL_THROTTLED, S_NORMAL_THROTTLED, F_THROTTLE_WAIT)

	// Dry run states.
	b.T(S_DRY_RUN_IDLE, S_STOPPED, F_NOOP)
	b.T(S_DRY_RUN_IDLE, S_DRY_RUN_IDLE, F_UPDATE_REPOS)
	b.T(S_DRY_RUN_IDLE, S_NORMAL_IDLE, F_NOOP)
	b.T(S_DRY_RUN_IDLE, S_DRY_RUN_THROTTLED, F_NOOP)
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
	b.T(S_DRY_RUN_THROTTLED, S_DRY_RUN_IDLE, F_NOOP)
	b.T(S_DRY_RUN_THROTTLED, S_DRY_RUN_THROTTLED, F_THROTTLE_WAIT)

	// Build the state machine.
	b.SetInitial(S_NORMAL_IDLE)
	sm, err := b.Build(path.Join(workdir, "state_machine"))
	if err != nil {
		return nil, err
	}
	s.s = sm
	return s, nil
}

// Get the next state.
func (s *AutoRollStateMachine) GetNext() (string, error) {
	desiredMode := s.a.GetMode()
	switch state := s.s.Current(); state {
	case S_STOPPED:
		switch desiredMode {
		case autoroll_modes.MODE_RUNNING:
			return S_NORMAL_IDLE, nil
		case autoroll_modes.MODE_DRY_RUN:
			return S_DRY_RUN_IDLE, nil
		case autoroll_modes.MODE_STOPPED:
			return S_STOPPED, nil
		default:
			return "", fmt.Errorf("Invalid mode: %q", desiredMode)
		}
	case S_NORMAL_IDLE:
		switch desiredMode {
		case autoroll_modes.MODE_RUNNING:
			break
		case autoroll_modes.MODE_DRY_RUN:
			return S_DRY_RUN_IDLE, nil
		case autoroll_modes.MODE_STOPPED:
			return S_STOPPED, nil
		default:
			return "", fmt.Errorf("Invalid mode: %q", desiredMode)
		}
		current := s.a.GetCurrentRev()
		next := s.a.GetNextRollRev()
		if current == next {
			return S_NORMAL_IDLE, nil
		} else if s.c.Get() >= ROLL_ATTEMPT_THROTTLE_NUM {
			return S_NORMAL_THROTTLED, nil
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
			desiredMode := s.a.GetMode()
			if desiredMode == autoroll_modes.MODE_DRY_RUN {
				return S_DRY_RUN_ACTIVE, nil
			} else if desiredMode == autoroll_modes.MODE_STOPPED {
				return S_STOPPED, nil
			} else if desiredMode == autoroll_modes.MODE_RUNNING {
				return S_NORMAL_ACTIVE, nil
			} else {
				return "", fmt.Errorf("Invalid mode %q", desiredMode)
			}
		}
	case S_NORMAL_SUCCESS:
		return S_NORMAL_IDLE, nil
	case S_NORMAL_FAILURE:
		return S_NORMAL_IDLE, nil
	case S_NORMAL_THROTTLED:
		if s.c.Get() < ROLL_ATTEMPT_THROTTLE_NUM {
			return S_NORMAL_IDLE, nil
		} else {
			return S_NORMAL_THROTTLED, nil
		}
	case S_DRY_RUN_IDLE:
		if desiredMode == autoroll_modes.MODE_RUNNING {
			return S_NORMAL_IDLE, nil
		} else if desiredMode == autoroll_modes.MODE_STOPPED {
			return S_STOPPED, nil
		} else if desiredMode != autoroll_modes.MODE_DRY_RUN {
			return "", fmt.Errorf("Invalid mode %q", desiredMode)
		}
		current := s.a.GetCurrentRev()
		next := s.a.GetNextRollRev()
		if current == next {
			return S_DRY_RUN_IDLE, nil
		} else if s.c.Get() >= ROLL_ATTEMPT_THROTTLE_NUM {
			return S_DRY_RUN_THROTTLED, nil
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
			if desiredMode == autoroll_modes.MODE_RUNNING {
				return S_NORMAL_ACTIVE, nil
			} else if desiredMode == autoroll_modes.MODE_STOPPED {
				return S_STOPPED, nil
			} else if desiredMode == autoroll_modes.MODE_DRY_RUN {
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
		if desiredMode == autoroll_modes.MODE_RUNNING {
			return S_NORMAL_ACTIVE, nil
		} else if desiredMode == autoroll_modes.MODE_STOPPED {
			return S_STOPPED, nil
		} else if desiredMode != autoroll_modes.MODE_DRY_RUN {
			return "", fmt.Errorf("Invalid mode %q", desiredMode)
		}

		if s.a.GetNextRollRev() == s.a.GetActiveRoll().RollingTo() {
			// The current dry run is for the commit we want. Leave
			// it open so we can land it if we want.
			return S_DRY_RUN_SUCCESS_LEAVING_OPEN, nil
		}
		return S_DRY_RUN_IDLE, nil
	case S_DRY_RUN_FAILURE:
		return S_DRY_RUN_IDLE, nil
	case S_DRY_RUN_THROTTLED:
		if s.c.Get() < ROLL_ATTEMPT_THROTTLE_NUM {
			return S_DRY_RUN_IDLE, nil
		} else {
			return S_DRY_RUN_THROTTLED, nil
		}
	default:
		return "", fmt.Errorf("Invalid state %q", state)
	}
}

// Attempt to perform the given state transition.
func (s *AutoRollStateMachine) Transition(dest string) error {
	fName, err := s.s.GetTransitionName(dest)
	if err != nil {
		return err
	}
	sklog.Infof("Attempting to perform transition from %q to %q: %s", s.s.Current(), dest, fName)
	if err := s.s.Transition(dest); err != nil {
		return err
	}
	sklog.Infof("Successfully performed transition.")
	return nil
}

// Attempt to perform the next state transition.
func (s *AutoRollStateMachine) NextTransition() error {
	next, err := s.GetNext()
	if err != nil {
		return err
	}
	return s.Transition(next)
}

// Perform the next state transition, plus any subsequent transitions which are
// no-ops.
func (s *AutoRollStateMachine) NextTransitionSequence() error {
	if err := s.NextTransition(); err != nil {
		return err
	}
	// Greedily perform transitions until we reach a transition which is not
	// a no-op, or until we've performed a maximum number of transitions, to
	// keep us from accidentally looping extremely quickly.
	for i := 0; i < MAX_NOOP_TRANSITIONS; i++ {
		next, err := s.GetNext()
		if err != nil {
			return err
		}
		fName, err := s.s.GetTransitionName(next)
		if err != nil {
			return err
		} else if fName == F_NOOP {
			if err := s.Transition(next); err != nil {
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
