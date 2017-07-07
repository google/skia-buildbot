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

	// Normal operation.
	S_NORMAL_IDLE      = "idle"
	S_NORMAL_ACTIVE    = "active"
	S_NORMAL_SUCCESS   = "success"
	S_NORMAL_FAILURE   = "failure"
	S_NORMAL_THROTTLED = "throttled"

	// Dry run.
	S_DRY_RUN_IDLE                 = "dry run idle"
	S_DRY_RUN_ACTIVE               = "dry run active"
	S_DRY_RUN_SUCCESS              = "dry run success"
	S_DRY_RUN_SUCCESS_LEAVING_OPEN = "dry run success; leaving open"
	S_DRY_RUN_FAILURE              = "dry run failure"
	S_DRY_RUN_THROTTLED            = "dry run throttled"

	// Stopped.
	S_STOPPED = "stopped"
)

// Interface for interacting with a single autoroll CL.
type AutoRollImpl interface {
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
	GetActiveRoll() AutoRollImpl

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

// New returns a StateMachine for the autoroller.
func New(impl AutoRollerImpl, workdir string) (*state_machine.PersistentStateMachine, error) {
	sm := state_machine.NewBuilder()

	// Global state.
	attemptCounter, err := util.NewPersistentAutoDecrementCounter(path.Join(workdir, "attempt_counter"), ROLL_ATTEMPT_THROTTLE_TIME)
	if err != nil {
		return nil, err
	}

	// Stopped state.
	sm.AddState(S_STOPPED, func() state_machine.State {
		desiredMode := impl.GetMode()
		if desiredMode == autoroll_modes.MODE_RUNNING {
			return S_NORMAL_IDLE
		} else if desiredMode == autoroll_modes.MODE_DRY_RUN {
			return S_DRY_RUN_IDLE
		} else if desiredMode == autoroll_modes.MODE_STOPPED {
			return S_STOPPED
		} else {
			sklog.Errorf("Invalid mode %q", desiredMode)
			return S_STOPPED
		}
	})

	// Normal states.
	sm.AddState(S_NORMAL_IDLE, func() state_machine.State {
		desiredMode := impl.GetMode()
		if desiredMode == autoroll_modes.MODE_DRY_RUN {
			return S_DRY_RUN_IDLE
		} else if desiredMode == autoroll_modes.MODE_STOPPED {
			return S_STOPPED
		} else if desiredMode != autoroll_modes.MODE_RUNNING {
			sklog.Errorf("Invalid mode %q", desiredMode)
			return S_STOPPED
		}
		current := impl.GetCurrentRev()
		next := impl.GetNextRollRev()
		if current == next {
			return S_NORMAL_IDLE
		} else if attemptCounter.Get() >= ROLL_ATTEMPT_THROTTLE_NUM {
			return S_NORMAL_THROTTLED
		} else {
			return S_NORMAL_ACTIVE
		}
	})
	sm.AddState(S_NORMAL_ACTIVE, func() state_machine.State {
		currentRoll := impl.GetActiveRoll()
		if currentRoll.IsFinished() {
			if currentRoll.IsSuccess() {
				return S_NORMAL_SUCCESS
			} else {
				return S_NORMAL_FAILURE
			}
		} else {
			desiredMode := impl.GetMode()
			if desiredMode == autoroll_modes.MODE_DRY_RUN {
				return S_DRY_RUN_ACTIVE
			} else if desiredMode == autoroll_modes.MODE_STOPPED {
				return S_STOPPED
			} else if desiredMode == autoroll_modes.MODE_RUNNING {
				return S_NORMAL_ACTIVE
			} else {
				sklog.Errorf("Invalid mode %q", desiredMode)
				return S_STOPPED
			}
		}
	})
	sm.AddState(S_NORMAL_SUCCESS, func() state_machine.State {
		return S_NORMAL_IDLE
	})
	sm.AddState(S_NORMAL_FAILURE, func() state_machine.State {
		return S_NORMAL_IDLE
	})
	sm.AddState(S_NORMAL_THROTTLED, func() state_machine.State {
		if attemptCounter.Get() < ROLL_ATTEMPT_THROTTLE_NUM {
			return S_NORMAL_IDLE
		} else {
			return S_NORMAL_THROTTLED
		}
	})

	// Dry run states.
	sm.AddState(S_DRY_RUN_IDLE, func() state_machine.State {
		desiredMode := impl.GetMode()
		if desiredMode == autoroll_modes.MODE_RUNNING {
			return S_NORMAL_IDLE
		} else if desiredMode == autoroll_modes.MODE_STOPPED {
			return S_STOPPED
		} else if desiredMode != autoroll_modes.MODE_DRY_RUN {
			sklog.Errorf("Invalid mode %q", desiredMode)
			return S_STOPPED
		}
		current := impl.GetCurrentRev()
		next := impl.GetNextRollRev()
		if current == next {
			return S_DRY_RUN_IDLE
		} else if attemptCounter.Get() >= ROLL_ATTEMPT_THROTTLE_NUM {
			return S_DRY_RUN_THROTTLED
		} else {
			return S_DRY_RUN_ACTIVE
		}
	})
	sm.AddState(S_DRY_RUN_ACTIVE, func() state_machine.State {
		currentRoll := impl.GetActiveRoll()
		if currentRoll.IsDryRunFinished() {
			if currentRoll.IsDryRunSuccess() {
				return S_DRY_RUN_SUCCESS
			} else {
				return S_DRY_RUN_FAILURE
			}
		} else {
			desiredMode := impl.GetMode()
			if desiredMode == autoroll_modes.MODE_RUNNING {
				return S_NORMAL_ACTIVE
			} else if desiredMode == autoroll_modes.MODE_STOPPED {
				return S_STOPPED
			} else if desiredMode == autoroll_modes.MODE_DRY_RUN {
				return S_DRY_RUN_ACTIVE
			} else {
				sklog.Errorf("Invalid mode %q", desiredMode)
				return S_STOPPED
			}
		}
	})
	sm.AddState(S_DRY_RUN_SUCCESS, func() state_machine.State {
		if impl.GetNextRollRev() == impl.GetActiveRoll().RollingTo() {
			// The current dry run is for the commit we want. Leave
			// it open so we can land it if we want.
			return S_DRY_RUN_SUCCESS_LEAVING_OPEN
		}
		return S_DRY_RUN_IDLE
	})
	sm.AddState(S_DRY_RUN_SUCCESS_LEAVING_OPEN, func() state_machine.State {
		if impl.GetNextRollRev() == impl.GetActiveRoll().RollingTo() {
			// The current dry run is for the commit we want. Leave
			// it open so we can land it if we want.
			return S_DRY_RUN_SUCCESS_LEAVING_OPEN
		}
		return S_DRY_RUN_IDLE
	})
	sm.AddState(S_DRY_RUN_FAILURE, func() state_machine.State {
		return S_DRY_RUN_IDLE
	})
	sm.AddState(S_DRY_RUN_THROTTLED, func() state_machine.State {
		if attemptCounter.Get() < ROLL_ATTEMPT_THROTTLE_NUM {
			return S_DRY_RUN_IDLE
		} else {
			return S_DRY_RUN_THROTTLED
		}
	})

	// Set the default state.
	if err := sm.SetInitial(S_NORMAL_IDLE); err != nil {
		return nil, err
	}

	// Stopped transitions.
	sm.AddTransition(S_STOPPED, S_STOPPED, nil)
	sm.AddTransition(S_STOPPED, S_NORMAL_IDLE, nil)
	sm.AddTransition(S_STOPPED, S_DRY_RUN_IDLE, nil)

	// Normal transitions.
	sm.AddTransition(S_NORMAL_IDLE, S_NORMAL_IDLE, func() error {
		return impl.UpdateRepos()
	})
	sm.AddTransition(S_NORMAL_IDLE, S_STOPPED, nil)
	sm.AddTransition(S_NORMAL_IDLE, S_NORMAL_ACTIVE, func() error {
		attemptCounter.Inc()
		return impl.UploadNewRoll(impl.GetCurrentRev(), impl.GetNextRollRev(), false)
	})
	sm.AddTransition(S_NORMAL_ACTIVE, S_NORMAL_ACTIVE, func() error {
		return impl.GetActiveRoll().Update()
	})
	sm.AddTransition(S_NORMAL_ACTIVE, S_NORMAL_SUCCESS, nil)
	sm.AddTransition(S_NORMAL_ACTIVE, S_NORMAL_FAILURE, func() error {
		return impl.GetActiveRoll().Close(autoroll.ROLL_RESULT_FAILURE, fmt.Sprintf("Commit queue failed; closing this roll."))
	})
	sm.AddTransition(S_NORMAL_ACTIVE, S_STOPPED, func() error {
		return impl.GetActiveRoll().Close(autoroll.ROLL_RESULT_FAILURE, fmt.Sprintf("AutoRoller is stopped; closing the active roll."))
	})
	sm.AddTransition(S_NORMAL_SUCCESS, S_NORMAL_IDLE, func() error {
		sklog.Infof("Roll succeeded; syncing the repo until it lands.")
		currentRoll := impl.GetActiveRoll()
		for {
			sklog.Infof("Syncing, looking for %s...", currentRoll.RollingTo())
			if err := impl.UpdateRepos(); err != nil {
				return err
			}
			rolledPast, err := impl.RolledPast(currentRoll.RollingTo())
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
	sm.AddTransition(S_NORMAL_FAILURE, S_NORMAL_IDLE, nil)
	sm.AddTransition(S_NORMAL_IDLE, S_NORMAL_THROTTLED, nil)
	sm.AddTransition(S_NORMAL_THROTTLED, S_NORMAL_IDLE, nil)

	// Dry run transitions.
	sm.AddTransition(S_DRY_RUN_IDLE, S_DRY_RUN_IDLE, func() error {
		return impl.UpdateRepos()
	})
	sm.AddTransition(S_DRY_RUN_IDLE, S_STOPPED, nil)
	sm.AddTransition(S_DRY_RUN_IDLE, S_DRY_RUN_ACTIVE, func() error {
		attemptCounter.Inc()
		return impl.UploadNewRoll(impl.GetCurrentRev(), impl.GetNextRollRev(), true)
	})
	sm.AddTransition(S_DRY_RUN_ACTIVE, S_DRY_RUN_ACTIVE, func() error {
		return impl.GetActiveRoll().Update()
	})
	sm.AddTransition(S_DRY_RUN_ACTIVE, S_DRY_RUN_SUCCESS, nil)
	sm.AddTransition(S_DRY_RUN_ACTIVE, S_DRY_RUN_FAILURE, func() error {
		return impl.GetActiveRoll().Close(autoroll.ROLL_RESULT_DRY_RUN_FAILURE, fmt.Sprintf("Commit queue failed; closing this roll."))
	})
	sm.AddTransition(S_DRY_RUN_ACTIVE, S_STOPPED, func() error {
		return impl.GetActiveRoll().Close(autoroll.ROLL_RESULT_DRY_RUN_FAILURE, fmt.Sprintf("AutoRoller is stopped; closing the active roll."))
	})
	sm.AddTransition(S_DRY_RUN_SUCCESS, S_DRY_RUN_SUCCESS_LEAVING_OPEN, func() error {
		return impl.GetActiveRoll().AddComment("Dry run finished successfully; leaving open in case we want to land.")
	})
	sm.AddTransition(S_DRY_RUN_SUCCESS, S_DRY_RUN_IDLE, func() error {
		currentRoll := impl.GetActiveRoll()
		return currentRoll.Close(autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, fmt.Sprintf("Repo has passed %s; will open a new dry run.", currentRoll.RollingTo()))
	})
	sm.AddTransition(S_DRY_RUN_SUCCESS_LEAVING_OPEN, S_DRY_RUN_SUCCESS_LEAVING_OPEN, func() error {
		return impl.UpdateRepos()
	})
	sm.AddTransition(S_DRY_RUN_SUCCESS_LEAVING_OPEN, S_DRY_RUN_IDLE, func() error {
		currentRoll := impl.GetActiveRoll()
		return currentRoll.Close(autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, fmt.Sprintf("Repo has passed %s; will open a new dry run.", currentRoll.RollingTo()))
	})
	sm.AddTransition(S_DRY_RUN_FAILURE, S_DRY_RUN_IDLE, func() error {
		currentRoll := impl.GetActiveRoll()
		return currentRoll.Close(autoroll.ROLL_RESULT_DRY_RUN_FAILURE, "Dry run failed. Closing, will open another.")
	})
	sm.AddTransition(S_DRY_RUN_IDLE, S_DRY_RUN_THROTTLED, nil)
	sm.AddTransition(S_DRY_RUN_THROTTLED, S_DRY_RUN_IDLE, nil)

	// Transitions between normal and dry run.
	sm.AddTransition(S_NORMAL_IDLE, S_DRY_RUN_IDLE, nil)
	sm.AddTransition(S_DRY_RUN_IDLE, S_NORMAL_IDLE, nil)
	sm.AddTransition(S_NORMAL_ACTIVE, S_DRY_RUN_ACTIVE, func() error {
		return impl.GetActiveRoll().SwitchToDryRun()
	})
	sm.AddTransition(S_DRY_RUN_ACTIVE, S_NORMAL_ACTIVE, func() error {
		return impl.GetActiveRoll().SwitchToNormal()
	})
	sm.AddTransition(S_DRY_RUN_SUCCESS_LEAVING_OPEN, S_NORMAL_ACTIVE, func() error {
		return impl.GetActiveRoll().SwitchToNormal()
	})

	return state_machine.NewPersistentStateMachine(path.Join(workdir, "state_machine"), sm)
}
