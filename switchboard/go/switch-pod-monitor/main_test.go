package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/switchboard"
	"go.skia.org/infra/machine/go/switchboard/mocks"
)

var errMock = errors.New("test error")

const hostname = "skia-rpi2-rack4-shelf1-002"

func TestRegisterPodWithSwitchboard_ContextCancelled_ReturnsNil(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(nil)
	switchboardMock.On("RemovePod", testutils.AnyContext, hostname).Return(nil)

	// Cancel the context so that registerPodWithSwitchboard exits.
	cancel()
	err := registerPodWithSwitchboard(ctx, hostname, switchboardMock, switchboard.PodKeepAliveDuration, switchboard.PodMaxConsecutiveKeepAliveErrors)
	require.NoError(t, err)
	switchboardMock.AssertExpectations(t)
}

func TestRegisterPodWithSwitchboard_AddPodFailes_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(errMock)

	err := registerPodWithSwitchboard(ctx, hostname, switchboardMock, switchboard.PodKeepAliveDuration, switchboard.PodMaxConsecutiveKeepAliveErrors)
	require.Contains(t, err.Error(), err.Error())
	switchboardMock.AssertExpectations(t)
}

func TestRegisterPodWithSwitchboard_TooManyKeepAlivePodFailures_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	switchboardMock := &mocks.Switchboard{}
	switchboardMock.On("AddPod", testutils.AnyContext, hostname).Return(nil)
	switchboardMock.On("RemovePod", testutils.AnyContext, hostname).Return(nil)
	switchboardMock.On("KeepAlivePod", testutils.AnyContext, hostname).Return(errMock)

	err := registerPodWithSwitchboard(ctx, hostname, switchboardMock, time.Nanosecond, 1)
	require.Contains(t, err.Error(), "Switchpod.KeepAlivePod failed 1 consecutive times")

	switchboardMock.AssertExpectations(t)
}
