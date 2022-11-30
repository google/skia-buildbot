// Package sink is for sending machine.Events that are eventually picked up by
// 'source'.
package sink

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/machine/go/machine"
)

var (
	errMyFake = errors.New("my fake error")

	events []machine.Event = nil
)

type FakeSink struct{}

func (f FakeSink) Send(ctx context.Context, ev machine.Event) error {
	events = append(events, ev)
	return nil
}

func TestNewMultiSink_HappyPath(t *testing.T) {
	f1 := FakeSink{}
	f2 := FakeSink{}

	event := machine.NewEvent()
	event.Host = machine.Host{
		Name: "skia-rpi2-rack4-shelf1-020",
	}

	ms := NewMultiSink(f1, f2)
	err := ms.Send(context.Background(), event)
	require.NoError(t, err)
	require.Len(t, events, 2)
	for _, e := range events {
		assertdeep.Equal(t, event, e)
	}
}

type ErrSink struct{}

func (f ErrSink) Send(ctx context.Context, ev machine.Event) error {
	return errMyFake
}

func TestNewMultiSink_SinkReturnsError_MultiSinkReturnsError(t *testing.T) {
	ms := NewMultiSink(ErrSink{})
	err := ms.Send(context.Background(), machine.NewEvent())
	require.Contains(t, err.Error(), errMyFake.Error())
}
