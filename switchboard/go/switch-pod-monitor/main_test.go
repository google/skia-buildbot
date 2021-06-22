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

func TestConnectToSwitchboardAndWait_ContextCancelled_ReturnsNil(t *testing.T) {
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

func TestConnectToSwitchboardAndWait_AddPodFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(errMock)

	err := connectToSwitchboardAndWait(ctx, hostname, switchboardMock, switchboard.PodKeepAliveDuration, switchboard.PodMaxConsecutiveKeepAliveErrors)
	require.Contains(t, err.Error(), err.Error())
	switchboardMock.AssertExpectations(t)
	cancel()
}

func TestConnectToSwitchboardAndWait_TooManyKeepAlivePodFailures_ReturnsError(t *testing.T) {
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

func TestConnectToSwitchboardAndWait_SIGTERMSent_RemovePodGetsCalled(t *testing.T) {
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

func TestConnectToSwitchboardAndWait_SIGTERMSentAndNoMeetingPointsRemain_ReturnsNoError(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	var addPodCalled sync.WaitGroup
	addPodCalled.Add(1)
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(nil).Run(func(args mock.Arguments) {
		addPodCalled.Done()
	})
	switchboardMock.On("RemovePod", testutils.AnyContext, hostname).Return(nil)
	switchboardMock.On("KeepAlivePod", testutils.AnyContext, hostname).Return(nil).Maybe()

	// ListMeetingPoints will only be called once since it returns 0 matching MeetingPoints.
	var numMeetingPointsCalled sync.WaitGroup
	numMeetingPointsCalled.Add(1)
	switchboardMock.On("NumMeetingPointsForPod", testutils.AnyContext, hostname).Return(0, nil).Run(func(args mock.Arguments) {
		numMeetingPointsCalled.Done()
	})

	go func() {
		err := connectToSwitchboardAndWait(ctx, hostname, switchboardMock, time.Millisecond, switchboard.PodMaxConsecutiveKeepAliveErrors)
		require.NoError(t, err)
	}()
	addPodCalled.Wait()
	err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	require.NoError(t, err)
	numMeetingPointsCalled.Wait()
	switchboardMock.AssertExpectations(t)
	cancel()
}

func TestConnectToSwitchboardAndWait_SIGTERMSentAndMeetingPointsRemain_ReturnsWithNoErrorOnceNoMatchingMeetingPointsAreFound(t *testing.T) {
	unittest.SmallTest(t)

	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	var addPodCalled sync.WaitGroup
	addPodCalled.Add(1)
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(nil).Run(func(args mock.Arguments) {
		addPodCalled.Done()
	})
	switchboardMock.On("RemovePod", testutils.AnyContext, hostname).Return(nil)

	// NumMeetingPointsForPod will be called twice, the firs time it returns  1,
	// the second time it returns a 0.
	var numMeetingPointsCalled sync.WaitGroup
	numMeetingPointsCalled.Add(1)
	switchboardMock.On("NumMeetingPointsForPod", testutils.AnyContext, hostname).Return(1, nil).Times(1)
	switchboardMock.On("NumMeetingPointsForPod", testutils.AnyContext, hostname).Return(0, nil).Run(func(args mock.Arguments) {
		numMeetingPointsCalled.Done()
	})
	switchboardMock.On("KeepAlivePod", testutils.AnyContext, hostname).Return(nil).Maybe()

	go func() {
		err := connectToSwitchboardAndWait(ctx, hostname, switchboardMock, time.Millisecond, switchboard.PodMaxConsecutiveKeepAliveErrors)
		require.NoError(t, err)
	}()
	addPodCalled.Wait()
	err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	require.NoError(t, err)
	numMeetingPointsCalled.Wait()
	switchboardMock.AssertExpectations(t)
	cancel()
}

func TestConnectToSwitchboardAndWait_TwoLoops_Success(t *testing.T) {
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
