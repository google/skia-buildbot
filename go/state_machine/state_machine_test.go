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
	b.AddState("15", func() (State, TransitionFn) {
		return "16", nil
	})
	b.AddState("16", func() (State, TransitionFn) {
		return "17", func() error {
			return fmt.Errorf("nope")
		}
	})
	b.SetInitial("85")
	s, err := b.Build()
	assert.EqualError(t, err, "Initial state \"85\" is not defined!")
	b.SetInitial("15")
	s, err = b.Build()
	assert.NoError(t, err)

	assert.NoError(t, err)
	assert.Equal(t, State("15"), s.Current())
	assert.NoError(t, s.NextTransition())
	assert.Equal(t, State("16"), s.Current())
	assert.EqualError(t, s.NextTransition(), "Failed to transition to state \"17\": nope")
	assert.Equal(t, State("16"), s.Current())

	b.AddState("85", func() (State, TransitionFn) {
		return "93", nil
	})
	b.AddState("93", func() (State, TransitionFn) {
		return "93", nil
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
}
