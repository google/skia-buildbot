package state_machine

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
)

func TestStateMachine(t *testing.T) {

	ctx := context.Background()

	gcsClient := mem_gcsclient.New("test-bucket")
	file := "test_state_machine"

	b := NewBuilder()
	b.T("15", "16", "noop")
	b.T("16", "17", "err")

	b.F("noop", nil)
	b.F("err", func(ctx context.Context) error {
		return fmt.Errorf("nope")
	})
	b.SetInitial("85")
	s, err := b.Build(ctx, gcsClient, file)
	require.EqualError(t, err, "Initial state \"85\" is not defined!")
	b.SetInitial("15")
	s, err = b.Build(ctx, gcsClient, file)
	require.EqualError(t, err, "No transitions defined from state \"17\"")
	b.T("17", "17", "noop")
	b.T("18", "17", "noop")
	s, err = b.Build(ctx, gcsClient, file)
	require.EqualError(t, err, "No transitions defined to state \"18\"")
	b.T("15", "18", "noop")
	s, err = b.Build(ctx, gcsClient, file)
	require.NoError(t, err)
	require.Equal(t, "15", s.Current())
	name, err := s.GetTransitionName("16")
	require.NoError(t, err)
	require.Equal(t, "noop", name)
	require.NoError(t, s.Transition(ctx, "16"))
	require.Equal(t, "16", s.Current())
	name, err = s.GetTransitionName("17")
	require.NoError(t, err)
	require.Equal(t, "err", name)
	require.EqualError(t, s.Transition(ctx, "17"), "Failed to transition from \"16\" to \"17\": nope")
	require.Equal(t, "16", s.Current())

	b.T("16", "17", "noop")
	p, err := b.Build(ctx, gcsClient, file)
	require.EqualError(t, err, "Multiple defined transitions from \"16\" to \"17\": \"err\", \"noop\"")
	splitIdx := -1
	for i, t := range b.transitions {
		if t.from == "16" && t.to == "17" && t.fn == "err" {
			splitIdx = i
			break
		}
	}
	require.False(t, splitIdx < 0)
	b.transitions = append(b.transitions[:splitIdx], b.transitions[splitIdx+1:]...)
	p, err = b.Build(ctx, gcsClient, file)
	require.NoError(t, err)

	require.Equal(t, "16", p.Current())
	name, err = p.GetTransitionName("17")
	require.NoError(t, err)
	require.Equal(t, "noop", name)
	require.NoError(t, p.Transition(ctx, "17"))
	require.Equal(t, "17", p.Current())
	p2, err := b.Build(ctx, gcsClient, file)
	require.NoError(t, err)
	require.Equal(t, p.Current(), p2.Current())
}
