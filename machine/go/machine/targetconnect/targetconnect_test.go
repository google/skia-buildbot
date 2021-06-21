// Package targetconnect initiates and maintains a connection from
// a test machine to a switchboard pod. See https://go/skia-switchboard.
package targetconnect

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
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

func TestSingleStep_FirstCallToReserveMeetingPointReturnsError_Returns(t *testing.T) {
	unittest.SmallTest(t)
	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("ReserveMeetingPoint", testutils.AnyContext, hostname, username).Return(meetingPoint, errMyMockError)

	c := New(switchboardMock, mockRevPortForwardPanics{}, hostname, username)
	c.singleStep(context.Background(), time.NewTicker(time.Microsecond), time.Microsecond)
	switchboardMock.AssertExpectations(t)
}

type mockRevPortForwardCancellable struct {
}

func (m mockRevPortForwardCancellable) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func TestSingleStep_KeepAliveMeetingPointGetsCalledMultipleTimes_Returns(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("ReserveMeetingPoint", testutils.AnyContext, hostname, username).Return(meetingPoint, nil)
	keepAliveCount := 0
	switchboardMock.On("KeepAliveMeetingPoint", testutils.AnyContext, meetingPoint).Run(func(args mock.Arguments) {
		keepAliveCount++
		if keepAliveCount > 1 {
			cancel()
		}
	}).Times(2).Return(nil)
	switchboardMock.On("ClearMeetingPoint", testutils.AnyContext, meetingPoint).Return(nil)

	c := New(switchboardMock, mockRevPortForwardCancellable{}, hostname, username)
	c.singleStep(ctx, time.NewTicker(time.Millisecond), time.Microsecond)
	switchboardMock.AssertExpectations(t)
}

func TestStart_ContextIsCancelled_ReturnsAndMeetingPointIsCleared(t *testing.T) {
	unittest.SmallTest(t)
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
	require.Error(t, err)
	clearMeetingPointCalledWG.Wait()
	require.Equal(t, int64(1), c.stepsCounter.Get())
	c.stepsCounter.Reset()
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
	unittest.SmallTest(t)
	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("ClearMeetingPoint", testutils.AnyContext, meetingPoint).Times(2).Return(nil)
	var reserveMeetingPointWG sync.WaitGroup
	reserveMeetingPointWG.Add(2)
	switchboardMock.On("ReserveMeetingPoint", testutils.AnyContext, hostname, username).Times(2).Run(func(args mock.Arguments) {
		reserveMeetingPointWG.Done()
	}).Return(meetingPoint, nil)

	c := New(switchboardMock, &mockRevPortForwardSuccessOnSecondCallToStart{}, hostname, username)
	ctx, cancel := context.WithCancel(context.Background())
	// Call Start() in a Go routine since we need to cancel the Context after
	// Start() is called, and Start() doesn't return.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := c.Start(ctx)
		require.Error(t, err)
	}()

	// Wait until ReserveMeetingPoint has been called twice.
	reserveMeetingPointWG.Wait()
	cancel()

	// Wait for Start() to return.
	wg.Wait()
	require.Equal(t, int64(2), c.stepsCounter.Get())
	c.stepsCounter.Reset()
	switchboardMock.AssertExpectations(t)
}
