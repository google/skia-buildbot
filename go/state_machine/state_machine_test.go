package state_machine

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestStateMachine(t *testing.T) {
	s := New(15)
	assert.Equal(t, State(15), s.Current())
	s.AddTransition(15, func() (State, error) {
		return 16, nil
	})
	assert.NoError(t, s.Transition())
	assert.Equal(t, State(16), s.Current())
	s.AddTransition(16, func() (State, error) {
		return -1, fmt.Errorf("nope")
	})
	assert.Error(t, s.Transition())
	assert.Equal(t, State(16), s.Current())
	s.AddTransition(16, func() (State, error) {
		return 17, nil
	})
	assert.NoError(t, s.Transition())
	assert.Equal(t, State(17), s.Current())
}
