package state_machine

import (
	"fmt"

	"go.skia.org/infra/go/state_machine"
)

/*
	State machine for the autoroller.
*/

const (
	MODE_NORMAL  = "normal"
	MODE_DRY_RUN = "dry run"
	MODE_STOPPED = "stopped"

	// Normal operation.
	S_NORMAL_IDLE    = "idle"
	S_NORMAL_ACTIVE  = "active"
	S_NORMAL_SUCCESS = "success"
	S_NORMAL_FAILURE = "failure"

	// Dry run.
	S_DRY_RUN_IDLE    = "dry run idle"
	S_DRY_RUN_ACTIVE  = "dry run active"
	S_DRY_RUN_SUCCESS = "dry run success"
	S_DRY_RUN_FAILURE = "dry run failure"

	// Stopped.
	S_STOPPED = "stopped"
)

type rollType interface {
	Close() error
	IsFinished() bool
	IsSuccess() bool
	SwitchToDryRun() error
	SwitchToNormal() error
}

type autoRollerImpl interface {
	CreateNewRoll(from, to string) (rollType, error)
	GetActiveRoll() (rollType, error)
	GetCurrentRev() (string, error)
	GetNextRollRev() (string, error)
	GetMode() string
}

func New(impl autoRollerImpl) *state_machine.StateMachine {
	sm := state_machine.New(S_NORMAL_IDLE)

	sm.AddState(S_STOPPED, func() (state_machine.State, error) {
		desiredMode := impl.GetMode()
		if desiredMode == MODE_NORMAL {
			return S_NORMAL_IDLE, nil
		} else if desiredMode == MODE_DRY_RUN {
			return S_DRY_RUN_IDLE, nil
		} else if desiredMode == MODE_STOPPED {
			return S_STOPPED, nil
		} else {
			return "", fmt.Errorf("Invalid mode %q", desiredMode)
		}
	})

	sm.AddState(S_NORMAL_IDLE, func() (state_machine.State, error) {
		desiredMode := impl.GetMode()
		if desiredMode == MODE_DRY_RUN {
			return S_DRY_RUN_IDLE, nil
		} else if desiredMode == MODE_STOPPED {
			return S_STOPPED, nil
		} else if desiredMode != MODE_NORMAL {
			return "", fmt.Errorf("Invalid mode %q", desiredMode)
		}
		current, err := impl.GetCurrentRev()
		if err != nil {
			return "", err
		}
		next, err := impl.GetNextRollRev()
		if err != nil {
			return "", err
		}
		if current == next {
			return S_NORMAL_IDLE, nil
		} else {
			_, err := impl.CreateNewRoll(current, next)
			if err != nil {
				return "", err
			}
			return S_NORMAL_ACTIVE, nil
		}
	})
	sm.AddState(S_NORMAL_ACTIVE, func() (state_machine.State, error) {
		roll, err := impl.GetActiveRoll()
		if err != nil {
			return "", err
		}
		// TODO(borenet): Update roll status in DB.
		if roll.IsFinished() {
			if roll.IsSuccess() {
				return S_NORMAL_SUCCESS, nil
			} else {
				return S_NORMAL_FAILURE, nil
			}
		} else {
			desiredMode := impl.GetMode()
			if desiredMode == MODE_DRY_RUN {
				if err := roll.SwitchToDryRun(); err != nil {
					return "", err
				}
				return S_DRY_RUN_ACTIVE, nil
			} else if desiredMode == MODE_STOPPED {
				if err := roll.Close(); err != nil {
					return "", err
				}
				return S_STOPPED, nil
			} else if desiredMode == MODE_NORMAL {
				return S_NORMAL_ACTIVE, nil
			} else {
				return "", fmt.Errorf("Invalid mode %q", desiredMode)
			}
		}
	})
	sm.AddState(S_NORMAL_SUCCESS, func() (state_machine.State, error) {
		// TODO(borenet): Maybe we don't need this state?
		return S_NORMAL_IDLE, nil
	})
	sm.AddState(S_NORMAL_FAILURE, func() (state_machine.State, error) {
		// TODO(borenet): Maybe we don't need this state?
		return S_NORMAL_IDLE, nil
	})

	sm.AddState(S_DRY_RUN_IDLE, func() (state_machine.State, error) {
		desiredMode := impl.GetMode()
		if desiredMode == MODE_NORMAL {
			return S_NORMAL_IDLE, nil
		} else if desiredMode == MODE_STOPPED {
			return S_STOPPED, nil
		} else if desiredMode != MODE_DRY_RUN {
			return "", fmt.Errorf("Invalid mode %q", desiredMode)
		}
		current, err := impl.GetCurrentRev()
		if err != nil {
			return "", err
		}
		next, err := impl.GetNextRollRev()
		if err != nil {
			return "", err
		}
		if current == next {
			return S_DRY_RUN_IDLE, nil
		} else {
			_, err := impl.CreateNewRoll(current, next)
			if err != nil {
				return "", err
			}
			return S_DRY_RUN_ACTIVE, nil
		}
	})
	sm.AddState(S_DRY_RUN_ACTIVE, func() (state_machine.State, error) {
		roll, err := impl.GetActiveRoll()
		if err != nil {
			return "", err
		}
		// TODO(borenet): Update roll status in DB.
		if roll.IsFinished() {
			if roll.IsSuccess() {
				return S_DRY_RUN_SUCCESS, nil
			} else {
				return S_DRY_RUN_FAILURE, nil
			}
		} else {
			desiredMode := impl.GetMode()
			if desiredMode == MODE_NORMAL {
				if err := roll.SwitchToNormal(); err != nil {
					return "", err
				}
				return S_DRY_RUN_ACTIVE, nil
			} else if desiredMode == MODE_STOPPED {
				if err := roll.Close(); err != nil {
					return "", err
				}
				return S_STOPPED, nil
			} else if desiredMode == MODE_DRY_RUN {
				return S_DRY_RUN_ACTIVE, nil
			} else {
				return "", fmt.Errorf("Invalid mode %q", desiredMode)
			}
		}
	})
	sm.AddState(S_NORMAL_SUCCESS, func() (state_machine.State, error) {
		// TODO(borenet): Maybe we don't need this state?
		return S_NORMAL_IDLE, nil
	})
	sm.AddState(S_NORMAL_FAILURE, func() (state_machine.State, error) {
		// TODO(borenet): Maybe we don't need this state?
		return S_NORMAL_IDLE, nil
	})

	// Normal operation.
	sm.AddTransition(S_NORMAL_IDLE, S_NORMAL_IDLE, nil)
	sm.AddTransition(S_NORMAL_IDLE, S_NORMAL_ACTIVE, nil)
	sm.AddTransition(S_NORMAL_ACTIVE, S_NORMAL_ACTIVE, nil)
	sm.AddTransition(S_NORMAL_ACTIVE, S_NORMAL_SUCCESS, nil)
	sm.AddTransition(S_NORMAL_ACTIVE, S_NORMAL_FAILURE, nil)
	sm.AddTransition(S_NORMAL_SUCCESS, S_NORMAL_IDLE, nil)
	sm.AddTransition(S_NORMAL_FAILURE, S_NORMAL_IDLE, nil)

	// Dry run.
	sm.AddTransition(S_DRY_RUN_IDLE, S_DRY_RUN_IDLE, nil)
	sm.AddTransition(S_DRY_RUN_IDLE, S_DRY_RUN_ACTIVE, nil)
	sm.AddTransition(S_DRY_RUN_ACTIVE, S_DRY_RUN_ACTIVE, nil)
	sm.AddTransition(S_DRY_RUN_ACTIVE, S_DRY_RUN_SUCCESS, nil)
	sm.AddTransition(S_DRY_RUN_ACTIVE, S_DRY_RUN_FAILURE, nil)
	sm.AddTransition(S_DRY_RUN_SUCCESS, S_DRY_RUN_IDLE, nil)
	sm.AddTransition(S_DRY_RUN_FAILURE, S_DRY_RUN_IDLE, nil)

	// Transitions between normal and dry run.
	sm.AddTransition(S_NORMAL_IDLE, S_DRY_RUN_IDLE, nil)
	sm.AddTransition(S_DRY_RUN_IDLE, S_NORMAL_IDLE, nil)
	sm.AddTransition(S_NORMAL_ACTIVE, S_DRY_RUN_ACTIVE, nil)
	sm.AddTransition(S_DRY_RUN_ACTIVE, S_NORMAL_ACTIVE, nil)

	return sm
}
