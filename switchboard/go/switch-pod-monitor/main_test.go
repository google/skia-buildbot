package main

import (
	"context"
	"errors"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/switchboard"
	"go.skia.org/infra/machine/go/switchboard/mocks"
)

var errMock = errors.New("test error")

const hostname = "skia-rpi2-rack4-shelf1-002"

func TestConnecToSwitchboardAndWait_ContextCancelled_ReturnsNil(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(nil)

	// Cancel the context so that registerPodWithSwitchboard exits.
	cancel()
	err := connectToSwitchboardAndWait(ctx, hostname, switchboardMock, switchboard.PodKeepAliveDuration, switchboard.PodMaxConsecutiveKeepAliveErrors)
	require.NoError(t, err)

	switchboardMock.AssertExpectations(t)
}

func TestConnecToSwitchboardAndWait_AddPodFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(errMock)

	err := connectToSwitchboardAndWait(ctx, hostname, switchboardMock, switchboard.PodKeepAliveDuration, switchboard.PodMaxConsecutiveKeepAliveErrors)
	require.Contains(t, err.Error(), err.Error())
	switchboardMock.AssertExpectations(t)
	cancel()
}

func TestConnecToSwitchboardAndWait_TooManyKeepAlivePodFailures_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(nil)
	switchboardMock.On("RemovePod", testutils.AnyContext, hostname).Return(nil)
	switchboardMock.On("KeepAlivePod", testutils.AnyContext, hostname).Return(errMock)

	err := connectToSwitchboardAndWait(ctx, hostname, switchboardMock, time.Nanosecond, 1)
	require.Contains(t, err.Error(), "Switchpod.KeepAlivePod failed 1 consecutive times")

	switchboardMock.AssertExpectations(t)
	cancel()
}

func TestConnecToSwitchboardAndWait_SIGTERMSent_RemovePodGetsCalled(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	var addPodCalled sync.WaitGroup
	addPodCalled.Add(1)
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(nil).Run(func(args mock.Arguments) {
		addPodCalled.Done()
	})
	var removePodCalled sync.WaitGroup
	removePodCalled.Add(1)
	switchboardMock.On("RemovePod", testutils.AnyContext, hostname).Return(nil).Run(func(args mock.Arguments) {
		removePodCalled.Done()
	})
	go func() {
		err := connectToSwitchboardAndWait(ctx, hostname, switchboardMock, switchboard.PodKeepAliveDuration, switchboard.PodMaxConsecutiveKeepAliveErrors)
		require.NoError(t, err)
	}()
	addPodCalled.Wait()
	err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	require.NoError(t, err)
	removePodCalled.Wait()
	switchboardMock.AssertExpectations(t)
	cancel()
}

func TestConnecToSwitchboardAndWait_SIGTERMSentAndNoMeetingPointsRemain_ReturnsNoError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	var addPodCalled sync.WaitGroup
	addPodCalled.Add(1)
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(nil).Run(func(args mock.Arguments) {
		addPodCalled.Done()
	})
	switchboardMock.On("RemovePod", testutils.AnyContext, hostname).Return(nil)

	var listPointsCalled sync.WaitGroup
	listPointsCalled.Add(1)
	switchboardMock.On("ListMeetingPoints", testutils.AnyContext).Return(nil, nil).Run(func(args mock.Arguments) {
		listPointsCalled.Done()
	})

	go func() {
		err := connectToSwitchboardAndWait(ctx, hostname, switchboardMock, time.Millisecond, switchboard.PodMaxConsecutiveKeepAliveErrors)
		require.NoError(t, err)
	}()
	addPodCalled.Wait()
	err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	require.NoError(t, err)
	listPointsCalled.Wait()
	switchboardMock.AssertExpectations(t)
	cancel()
}

func TestConnecToSwitchboardAndWait_SIGTERMSentAndMeetingPointsRemain_ReturnsWithNoErrorOnceNoMatchingMeetingPointsAreFound(t *testing.T) {
	unittest.SmallTest(t)

	meetingPoint := switchboard.MeetingPoint{
		PodName: hostname,
	}

	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	var addPodCalled sync.WaitGroup
	addPodCalled.Add(1)
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(nil).Run(func(args mock.Arguments) {
		addPodCalled.Done()
	})
	switchboardMock.On("RemovePod", testutils.AnyContext, hostname).Return(nil)

	var listPointsCalled sync.WaitGroup
	listPointsCalled.Add(1)
	switchboardMock.On("ListMeetingPoints", testutils.AnyContext).Return([]switchboard.MeetingPoint{meetingPoint}, nil).Times(1)
	switchboardMock.On("ListMeetingPoints", testutils.AnyContext).Return(nil, nil).Run(func(args mock.Arguments) {
		listPointsCalled.Done()
	})

	go func() {
		err := connectToSwitchboardAndWait(ctx, hostname, switchboardMock, time.Millisecond, switchboard.PodMaxConsecutiveKeepAliveErrors)
		require.NoError(t, err)
	}()
	addPodCalled.Wait()
	err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	require.NoError(t, err)
	listPointsCalled.Wait()
	switchboardMock.AssertExpectations(t)
	cancel()
}
func TestConnecToSwitchboardAndWait_TwoLoops_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(nil)
	count := 0
	switchboardMock.On("KeepAlivePod", testutils.AnyContext, hostname).Return(nil).Run(func(args mock.Arguments) {
		count++
		if count > 1 {
			cancel()
		}
	})

	err := connectToSwitchboardAndWait(ctx, hostname, switchboardMock, time.Nanosecond, 1)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	switchboardMock.AssertExpectations(t)
}
