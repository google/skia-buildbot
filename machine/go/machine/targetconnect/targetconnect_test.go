// Package targetconnect initiates and maintains a connection from
// a test machine to a switchboard pod. See https://go/skia-switchboard.
package targetconnect

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/machine/go/switchboard"
	"go.skia.org/infra/machine/go/switchboard/mocks"
)

const (
	hostname = "skia-rpi2-rack4-shelf1-002"
	username = "root"
)

var (
	errMyMockError  = errors.New("my mock error")
	errMeetingPoint = switchboard.MeetingPoint{}
)

type mockRevPortForwardSuccess struct{}

func (mockRevPortForwardSuccess) Start(context.Context) error { return nil }

type mockRevPortForwardFail struct{}

func (mockRevPortForwardFail) Start(context.Context) error { return errMyMockError }

type mockRevPortForwardCancellable struct{}

func (mockRevPortForwardCancellable) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func TestStart_FirstCallToReserveMeetingPointReturnsError_ReturnsError(t *testing.T) {
	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("ReserveMeetingPoint", testutils.AnyContext, hostname, username).Return(errMeetingPoint, errMyMockError)

	c := New(switchboardMock, mockRevPortForwardSuccess{}, hostname, username)
	err := c.Start(context.Background())
	require.Error(t, err)
}
