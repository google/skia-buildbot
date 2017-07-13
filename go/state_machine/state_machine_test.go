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

	w, err := ioutil.TempDir("", "")
	defer testutils.RemoveAll(t, w)
	file := path.Join(w, "state_machine")

	b := NewBuilder()
	b.T("15", "16", "noop")
	b.T("16", "17", "err")

	b.F("noop", nil)
	b.F("err", func() error {
		return fmt.Errorf("nope")
	})
	b.SetInitial("85")
	s, err := b.Build(file)
	assert.EqualError(t, err, "Initial state \"85\" is not defined!")
	b.SetInitial("15")
	s, err = b.Build(file)
	assert.EqualError(t, err, "No transitions defined from state \"17\"")
	b.T("17", "17", "noop")
	s, err = b.Build(file)
	assert.EqualError(t, err, "No transitions defined to state \"15\"")
	b.T("15", "15", "noop")
	s, err = b.Build(file)
	assert.NoError(t, err)
	assert.Equal(t, "15", s.Current())
	name, err := s.GetTransitionName("16")
	assert.NoError(t, err)
	assert.Equal(t, "noop", name)
	assert.NoError(t, s.Transition("16"))
	assert.Equal(t, "16", s.Current())
	name, err = s.GetTransitionName("17")
	assert.NoError(t, err)
	assert.Equal(t, "err", name)
	assert.EqualError(t, s.Transition("17"), "Failed to transition from \"16\" to \"17\": nope")
	assert.Equal(t, "16", s.Current())

	b.T("16", "17", "noop")
	p, err := b.Build(file)
	assert.NoError(t, err)

	assert.Equal(t, "16", p.Current())
	name, err = s.GetTransitionName("17")
	assert.NoError(t, err)
	assert.Equal(t, "noop", name)
	assert.NoError(t, p.Transition("17"))
	assert.Equal(t, "17", p.Current())
	p2, err := b.Build(file)
	assert.NoError(t, err)
	assert.Equal(t, p.Current(), p2.Current())
}
