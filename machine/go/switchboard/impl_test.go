package switchboard

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machineserver/config"
)

const podName = "switch-pod-0"

// time to use by now.Now() by default.
var mockTime = time.Unix(12, 0).UTC()

func setupForTest(t *testing.T) (context.Context, *switchboardImpl) {
	unittest.RequiresFirestoreEmulator(t)
	cfg := config.InstanceConfig{
		Store: config.Store{
			Project:  "test-project",
			Instance: fmt.Sprintf("test-%s", uuid.New()),
		},
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, now.ContextKey, mockTime)
	s, err := New(ctx, true, cfg)
	require.NoError(t, err)
	return ctx, s
}

func TestAddPod_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	// Confirm it was added correctly.
	pods, err := s.ListPods(ctx)
	require.NoError(t, err)
	require.Len(t, pods, 1)
	require.Equal(t, Pod{
		Name:        podName,
		LastUpdated: mockTime,
	}, pods[0])
}

func TestAddPod_AddWhenAlreadyExisting_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	// Adding the same pod should fail.
	err = s.AddPod(ctx, podName)
	require.Error(t, err)
}

func TestRemovePod_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	// Now remove it.
	err = s.RemovePod(ctx, podName)
	require.NoError(t, err)

	// Confirm it's gone.
	pods, err := s.ListPods(ctx)
	require.NoError(t, err)
	require.Len(t, pods, 0)
}

func TestKeepAlivePod_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	newMockTime := mockTime.Add(time.Hour)
	ctx = context.WithValue(ctx, now.ContextKey, newMockTime)
	err = s.KeepAlivePod(ctx, podName)
	require.NoError(t, err)

	// Confirm the time has been updated.
	pods, err := s.ListPods(ctx)
	require.Len(t, pods, 1)
	require.Equal(t, Pod{
		Name:        podName,
		LastUpdated: newMockTime,
	}, pods[0])
}
