// Package targetconnect initiates and maintains a connection from
// a test machine to a switchboard pod. See https://go/skia-switchboard.
package targetconnect

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
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
	errMyMockError = errors.New("my mock error")
	meetingPoint   = switchboard.MeetingPoint{}
)

type mockRevPortForwardPanics struct{}

func (mockRevPortForwardPanics) Start(context.Context) error {
	panic("Start should never get called.")
}

func TestStart_FirstCallToReserveMeetingPointReturnsError_ReturnsError(t *testing.T) {
	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("ReserveMeetingPoint", testutils.AnyContext, hostname, username).Return(meetingPoint, errMyMockError)

	c := New(switchboardMock, mockRevPortForwardPanics{}, hostname, username)
	err := c.Start(context.Background())
	require.Contains(t, err.Error(), errMyMockError.Error())
	switchboardMock.AssertExpectations(t)
}

type mockRevPortForwardCancellable struct {
}

func (m mockRevPortForwardCancellable) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func TestStart_ContextIsCancelled_ReturnsNilAndMeetingPointIsCleared(t *testing.T) {
	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("ReserveMeetingPoint", testutils.AnyContext, hostname, username).Return(meetingPoint, nil)
	var clearMeetingPointCalledWG sync.WaitGroup
	clearMeetingPointCalledWG.Add(1)
	switchboardMock.On("ClearMeetingPoint", testutils.AnyContext, meetingPoint).Run(func(mock.Arguments) {
		clearMeetingPointCalledWG.Done()
	}).Return(nil)

	c := New(switchboardMock, mockRevPortForwardCancellable{}, hostname, username)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.Start(ctx)
	require.NoError(t, err)
	clearMeetingPointCalledWG.Wait()
	switchboardMock.AssertExpectations(t)
}

// Mock revPortForward so that Start fails the first time it's called and then
// succeeds the second time.
type mockRevPortForwardSuccessOnSecondCallToStart struct {
	calls int
}

func (m *mockRevPortForwardSuccessOnSecondCallToStart) Start(ctx context.Context) error {
	m.calls++
	// Fail out the first time Start is called.
	if m.calls == 1 {
		return errMyMockError
	}
	<-ctx.Done()
	return nil
}

func TestStart_FirstCallToRevPortForwardFails_CausesASecondCalltoConnectToPod(t *testing.T) {
	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("ReserveMeetingPoint", testutils.AnyContext, hostname, username).Times(2).Return(meetingPoint, nil)
	// Mock ClearMeetingPoint so that it fails the first time it's called and
	// then succeeds the second time. Also have a sync.WaitGroup for each call
	// so that we can be assured they have been called as the test progresses.
	clearMeetingPointWG := [2]sync.WaitGroup{}
	clearMeetingPointCalls := 0
	clearMeetingPointWG[0].Add(1)
	clearMeetingPointWG[1].Add(1)
	switchboardMock.On("ClearMeetingPoint", testutils.AnyContext, meetingPoint).Times(2).Run(func(args mock.Arguments) {
		clearMeetingPointWG[clearMeetingPointCalls].Done()
		clearMeetingPointCalls++
	}).Return(nil)

	c := New(switchboardMock, &mockRevPortForwardSuccessOnSecondCallToStart{}, hostname, username)
	ctx, cancel := context.WithCancel(context.Background())
	// Call Start() in a Go routine since we need to cancel the Context after
	// Start() is called, and Start() doesn't return.
	go func() {
		err := c.Start(ctx)
		require.NoError(t, err)
	}()
	// Wait unitl we know that the first ClearMeetingPoint has been called.
	clearMeetingPointWG[0].Wait()
	cancel()
	// Wait until the second ClearMeetingPoint call has been done.
	clearMeetingPointWG[1].Wait()
	switchboardMock.AssertExpectations(t)
}
