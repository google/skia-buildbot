package state_machine

import (
	"context"
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
)

func TestStateMachine(t *testing.T) {
	testutils.MediumTest(t)

	ctx := context.Background()

	gcsClient := test_gcsclient.NewMemoryClient("test-bucket")
	file := "test_state_machine"
	busyFile := file + ".transitioning"

	b := NewBuilder()
	b.T("15", "16", "noop")
	b.T("16", "17", "err")

	b.F("noop", nil)
	b.F("err", func(ctx context.Context) error {
		return fmt.Errorf("nope")
	})
	b.SetInitial("85")
	s, err := b.Build(ctx, gcsClient, file)
	assert.EqualError(t, err, "Initial state \"85\" is not defined!")
	b.SetInitial("15")
	s, err = b.Build(ctx, gcsClient, file)
	assert.EqualError(t, err, "No transitions defined from state \"17\"")
	b.T("17", "17", "noop")
	b.T("18", "17", "noop")
	s, err = b.Build(ctx, gcsClient, file)
	assert.EqualError(t, err, "No transitions defined to state \"18\"")
	b.T("15", "18", "noop")
	s, err = b.Build(ctx, gcsClient, file)
	assert.NoError(t, err)
	assert.Equal(t, "15", s.Current())
	name, err := s.GetTransitionName("16")
	assert.NoError(t, err)
	assert.Equal(t, "noop", name)
	assert.NoError(t, s.Transition(ctx, "16"))
	assert.Equal(t, "16", s.Current())
	name, err = s.GetTransitionName("17")
	assert.NoError(t, err)
	assert.Equal(t, "err", name)
	assert.EqualError(t, s.Transition(ctx, "17"), "Failed to transition from \"16\" to \"17\": nope")
	assert.Equal(t, "16", s.Current())

	b.T("16", "17", "noop")
	p, err := b.Build(ctx, gcsClient, file)
	assert.EqualError(t, err, "Multiple defined transitions from \"16\" to \"17\": \"err\", \"noop\"")
	splitIdx := -1
	for i, t := range b.transitions {
		if t.from == "16" && t.to == "17" && t.fn == "err" {
			splitIdx = i
			break
		}
	}
	assert.False(t, splitIdx < 0)
	b.transitions = append(b.transitions[:splitIdx], b.transitions[splitIdx+1:]...)
	p, err = b.Build(ctx, gcsClient, file)
	assert.NoError(t, err)

	assert.Equal(t, "16", p.Current())
	name, err = p.GetTransitionName("17")
	assert.NoError(t, err)
	assert.Equal(t, "noop", name)
	assert.NoError(t, p.Transition(ctx, "17"))
	assert.Equal(t, "17", p.Current())
	p2, err := b.Build(ctx, gcsClient, file)
	assert.NoError(t, err)
	assert.Equal(t, p.Current(), p2.Current())

	// Verify that we refuse to transition when the busy file exists.
	assert.NoError(t, gcsClient.SetFileContents(ctx, busyFile, gcs.FILE_WRITE_OPTS_TEXT, []byte("anotherstate")))
	expectErr := "Transition to \"anotherstate\" already in progress; did a previous transition get interrupted?"
	_, err = b.Build(ctx, gcsClient, file)
	assert.EqualError(t, err, expectErr)
	assert.EqualError(t, p2.Transition(ctx, "17"), expectErr)
}
