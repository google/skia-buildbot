package state_machine

import (
	"testing"

	"go.skia.org/infra/go/state_machine"

	"github.com/stretchr/testify/assert"
)

type testRollTypeImpl struct {
	result string
}

func (r *testRollTypeImpl) Close() error {
	return nil
}

func (r *testRollTypeImpl) IsFinished() bool {
	return r.result != ""
}

func (r *testRollTypeImpl) IsSuccess() bool {
	return r.result == "SUCCESS"
}

func (r *testRollTypeImpl) SwitchToDryRun() error {
	return nil
}

func (r *testRollTypeImpl) SwitchToNormal() error {
	return nil
}

type testAutoRollerImpl struct {
	currentRoll *testRollTypeImpl
	currentRev  string
	nextRollRev string
	mode        string
}

func (a *testAutoRollerImpl) CreateNewRoll(from, to string) (rollType, error) {
	a.currentRoll = &testRollTypeImpl{
		result: "",
	}
	return a.currentRoll, nil
}

func (a *testAutoRollerImpl) GetActiveRoll() (rollType, error) {
	return a.currentRoll, nil
}

func (a *testAutoRollerImpl) GetCurrentRev() (string, error) {
	return a.currentRev, nil
}

func (a *testAutoRollerImpl) GetNextRollRev() (string, error) {
	return a.nextRollRev, nil
}

func (a *testAutoRollerImpl) GetMode() string {
	return a.mode
}

func assertEqual(t *testing.T, a, b state_machine.State) {
	assert.Equal(t, a, b)
}

func TestAutoRollStateMachine(t *testing.T) {
	// Create the roller.
	roller := &testAutoRollerImpl{
		currentRoll: nil,
		currentRev:  "HEAD",
		nextRollRev: "HEAD",
		mode:        MODE_NORMAL,
	}
	sm := New(roller)
	assert.NotNil(t, sm)
	assertEqual(t, S_NORMAL_IDLE, sm.Current())

	// Ensure that we stay idle.
	assert.NoError(t, sm.NextTransition())
	assertEqual(t, S_NORMAL_IDLE, sm.Current())

	// Create a new roll.
	roller.nextRollRev = "HEAD+1"
	assert.NoError(t, sm.NextTransition())
	assertEqual(t, S_NORMAL_ACTIVE, sm.Current())

	// Still active.
	assert.NoError(t, sm.NextTransition())
	assertEqual(t, S_NORMAL_ACTIVE, sm.Current())

	// Roll finished successfully.
	roller.currentRoll.result = "SUCCESS"
	roller.currentRev = roller.nextRollRev
	assert.NoError(t, sm.NextTransition())
	assertEqual(t, S_NORMAL_SUCCESS, sm.Current())
	assert.NoError(t, sm.NextTransition())
	assertEqual(t, S_NORMAL_IDLE, sm.Current())
}
