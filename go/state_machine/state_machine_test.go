package state_machine

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestStateMachine(t *testing.T) {
	b := NewBuilder()
	b.AddState("15", func() State {
		return "16"
	})
	b.AddState("16", func() State {
		return "17"
	})
	b.AddTransition("15", "16", nil)
	b.AddTransition("16", "15", func() error {
		return fmt.Errorf("nope")
	})
	b.AddTransition("16", "17", func() error {
		return nil
	})
	assert.EqualError(t, b.SetInitial("85"), "Undefined state \"85\"")
	assert.NoError(t, b.SetInitial("15"))
	s, err := b.Build()
	assert.EqualError(t, err, "Transition from state \"16\" to state \"17\" but state \"17\" is not defined!")
	assert.Nil(t, s)
	b.AddState("17", func() State {
		return "17"
	})
	s, err = b.Build()
	assert.NoError(t, err)

	assert.NoError(t, err)
	assert.Equal(t, State("15"), s.Current())
	assert.EqualError(t, s.Transition("17"), "No transition defined from state \"15\" to \"17\"")
	assert.Equal(t, State("15"), s.Current())
	assert.NoError(t, s.Transition("16"))
	assert.Equal(t, State("16"), s.Current())
	assert.EqualError(t, s.Transition("15"), "Failed to transition to state \"15\": nope")
	assert.Equal(t, State("16"), s.Current())
	assert.NoError(t, s.Transition("17"))
	assert.Equal(t, State("17"), s.Current())
	assert.EqualError(t, s.Transition("17"), "No transitions defined from state \"17\"")

	b.AddTransition("85", "93", nil)
	_, err = b.Build()
	assert.EqualError(t, err, "Transition from state \"85\" but state \"85\" is not defined!")
}
