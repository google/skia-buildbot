package state_machine

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func TestStateMachine(t *testing.T) {
	testutils.MediumTest(t)

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
	b.SetInitial("85")
	s, err := b.Build()
	assert.EqualError(t, err, "Transition from state \"16\" to state \"17\" but state \"17\" is not defined!")
	assert.Nil(t, s)
	b.AddState("17", func() State {
		return "17"
	})
	s, err = b.Build()
	assert.EqualError(t, err, "Initial state \"85\" is not defined!")
	b.SetInitial("15")
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
	b.AddState("85", func() State {
		return "93"
	})
	b.AddState("93", func() State {
		return "93"
	})

	w, err := ioutil.TempDir("", "")
	defer testutils.RemoveAll(t, w)
	file := path.Join(w, "state_machine")
	p, err := b.BuildPersistent(file)
	assert.NoError(t, err)

	check := func(expect string) {
		assert.NoError(t, p.NextTransition())
		assert.Equal(t, State(expect), p.Current())
		p2, err := b.BuildPersistent(file)
		assert.NoError(t, err)
		assert.Equal(t, p.Current(), p2.Current())
	}
	assert.Equal(t, State("15"), p.Current())
	check("16")
	check("17")
}
