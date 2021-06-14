package main

import (
	"context"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/switchboard"
	"go.skia.org/infra/machine/go/switchboard/mocks"
)

func TestRegisterPodWithSwitchboard_SIGTERMSent_RemovePodGetsCalled(t *testing.T) {
	unittest.ManualTest(t)
	ctx := context.Background()

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
		err := registerPodWithSwitchboard(ctx, hostname, switchboardMock, switchboard.PodKeepAliveDuration, switchboard.PodMaxConsecutiveKeepAliveErrors)
		require.NoError(t, err)
	}()
	addPodCalled.Wait()
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	removePodCalled.Wait()

	switchboardMock.AssertExpectations(t)
}
