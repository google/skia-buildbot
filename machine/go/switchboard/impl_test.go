package switchboard

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machineserver/config"
)

// Constants and variables used by all tests.

const podName = "switch-pod-0"

var (
	// time to use by now.Now() by default.
	mockTime = time.Unix(12, 0).UTC()

	machineID = "skia-rpi2-rack4-shelf1-003"

	userName = "chrome-bot"
)

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
	for _, c := range s.counters {
		c.Reset()
	}
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
	require.Equal(t, int64(1), s.counters[switchboardAddPod].Get())
	require.Equal(t, int64(1), s.counters[switchboardListPod].Get())
	require.Equal(t, int64(0), s.counters[switchboardListPodErrors].Get())
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

	require.Equal(t, int64(2), s.counters[switchboardAddPod].Get())
}

func TestRemovePod_PodExists_Success(t *testing.T) {
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
	require.Equal(t, int64(1), s.counters[switchboardAddPod].Get())
	require.Equal(t, int64(1), s.counters[switchboardRemovePod].Get())
	require.Equal(t, int64(1), s.counters[switchboardListPod].Get())
	require.Equal(t, int64(0), s.counters[switchboardListPodErrors].Get())
}

func TestRemovePod_PodDoesNotExist_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Note we never added a pod.
	err := s.RemovePod(ctx, podName)
	require.NoError(t, err)
	require.Equal(t, int64(1), s.counters[switchboardRemovePod].Get())
}

func TestKeepAlivePod_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	newMockTime := mockTime.Add(time.Hour)
	ctx = context.WithValue(ctx, now.ContextKey, newMockTime)

	// Call KeepAlivePod so the time gets updated.
	err = s.KeepAlivePod(ctx, podName)
	require.NoError(t, err)

	// Confirm the time has been updated.
	pods, err := s.ListPods(ctx)
	require.NoError(t, err)
	require.Len(t, pods, 1)
	require.Equal(t, Pod{
		Name:        podName,
		LastUpdated: newMockTime,
	}, pods[0])

	require.Equal(t, int64(1), s.counters[switchboardAddPod].Get())
	require.Equal(t, int64(1), s.counters[switchboardKeepAlivePod].Get())
	require.Equal(t, int64(0), s.counters[switchboardKeepAlivePodErrors].Get())
	require.Equal(t, int64(1), s.counters[switchboardListPod].Get())
	require.Equal(t, int64(0), s.counters[switchboardListPodErrors].Get())
}

func TestKeepAlivePod_PodDoesNotExist_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Note we never added a pod.
	err := s.KeepAlivePod(ctx, podName)
	require.Error(t, err)

	require.Equal(t, int64(1), s.counters[switchboardKeepAlivePod].Get())
	require.Equal(t, int64(1), s.counters[switchboardKeepAlivePodErrors].Get())
}

func TestReserveMeetingPoint_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	// Reserve a MeetingPoint.
	meetingPoint, err := s.ReserveMeetingPoint(ctx, machineID, userName)
	require.NoError(t, err)

	// Confirm the MeetingPoint is in the correct pod.
	require.GreaterOrEqual(t, meetingPoint.Port, portRangeBegin)
	require.GreaterOrEqual(t, portRangeEnd, meetingPoint.Port)
	require.Equal(t, mockTime, meetingPoint.LastUpdated)
	require.Equal(t, meetingPoint.PodName, podName)
	require.Equal(t, meetingPoint.Username, userName)

	require.Equal(t, int64(1), s.counters[switchboardReserveMeetingpoint].Get())
	require.Equal(t, int64(0), s.counters[switchboardReserveMeetingpointErrors].Get())
}

func TestReserveMeetingPoint_TriesAgainOnCollision_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	// Get a reservation for machineID.
	rand.Seed(1)
	meetingPoint, err := s.ReserveMeetingPoint(ctx, machineID, userName)
	require.NoError(t, err)

	// Reset rand so we guess the same port the first time, which should result
	// in a single collision, which means it tries again and then succeeds.
	rand.Seed(1)
	meetingPoint, err = s.ReserveMeetingPoint(ctx, machineID, userName)
	require.NoError(t, err)

	require.GreaterOrEqual(t, meetingPoint.Port, portRangeBegin)
	require.GreaterOrEqual(t, portRangeEnd, meetingPoint.Port)
	require.Equal(t, mockTime, meetingPoint.LastUpdated)
	require.Equal(t, meetingPoint.PodName, podName)
	require.Equal(t, meetingPoint.Username, userName)

	require.Equal(t, int64(2), s.counters[switchboardReserveMeetingpoint].Get())
	require.Equal(t, int64(1), s.counters[switchboardReserveMeetingpointErrors].Get()) // Confirm we failed at least once.
}

func TestReserveMeetingPoint_NoPodsAvailable_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// We haven't added any pods, so this should fail after 'reserveRetries' retries.
	_, err := s.ReserveMeetingPoint(ctx, machineID, userName)
	require.Error(t, err)
	require.Contains(t, err.Error(), ErrNoPodsFound.Error())

	require.Equal(t, int64(1), s.counters[switchboardReserveMeetingpoint].Get())
	require.Equal(t, int64(reserveRetries), s.counters[switchboardReserveMeetingpointErrors].Get())
}

func TestGetMeetingPoint_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	meetingPoint, err := s.ReserveMeetingPoint(ctx, machineID, userName)
	require.NoError(t, err)

	meetingPoint, err = s.GetMeetingPoint(ctx, machineID)
	require.NoError(t, err)

	require.GreaterOrEqual(t, meetingPoint.Port, portRangeBegin)
	require.GreaterOrEqual(t, portRangeEnd, meetingPoint.Port)
	require.Equal(t, mockTime, meetingPoint.LastUpdated)
	require.Equal(t, meetingPoint.PodName, podName)
	require.Equal(t, meetingPoint.Username, userName)

	require.Equal(t, int64(1), s.counters[switchboardReserveMeetingpoint].Get())
	require.Equal(t, int64(0), s.counters[switchboardReserveMeetingpointErrors].Get())
	require.Equal(t, int64(1), s.counters[switchboardGetMeetingpoint].Get())
	require.Equal(t, int64(0), s.counters[switchboardGetMeetingpointErrors].Get())
}

func TestGetMeetingPoint_NoSuchMachine_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	// Note that we don't call ReserveMeetingPoint before calling GetMeetingPoint.
	_, err = s.GetMeetingPoint(ctx, machineID)
	require.Error(t, err)
	require.Contains(t, err.Error(), ErrMachineNotFound.Error())

	require.Equal(t, int64(1), s.counters[switchboardGetMeetingpoint].Get())
	require.Equal(t, int64(1), s.counters[switchboardGetMeetingpointErrors].Get())
}

func TestKeepAliveMeetingPoint_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	meetingPoint, err := s.ReserveMeetingPoint(ctx, machineID, userName)
	require.NoError(t, err)

	// Advance the mock time.
	newMockTime := mockTime.Add(time.Hour)
	ctx = context.WithValue(ctx, now.ContextKey, newMockTime)

	// newMockTime should be used for LastUpdated.
	err = s.KeepAliveMeetingPoint(ctx, meetingPoint)
	require.NoError(t, err)

	meetingPoint, err = s.GetMeetingPoint(ctx, machineID)
	require.NoError(t, err)

	require.GreaterOrEqual(t, meetingPoint.Port, portRangeBegin)
	require.GreaterOrEqual(t, portRangeEnd, meetingPoint.Port)
	require.Equal(t, newMockTime, meetingPoint.LastUpdated) // LastUpdated has changed.
	require.Equal(t, meetingPoint.PodName, podName)
	require.Equal(t, meetingPoint.Username, userName)

	require.Equal(t, int64(1), s.counters[switchboardReserveMeetingpoint].Get())
	require.Equal(t, int64(0), s.counters[switchboardReserveMeetingpointErrors].Get())
	require.Equal(t, int64(1), s.counters[switchboardKeepAliveMeetingpoint].Get())
	require.Equal(t, int64(0), s.counters[switchboardKeepAliveMeetingpointErrors].Get())
}

func TestKeepAliveMeetingPoint_MeetingPointDoesNotExist_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	err := s.KeepAliveMeetingPoint(ctx, MeetingPoint{
		PodName: podName,
		Port:    portRangeBegin,
	})
	require.Error(t, err)

	require.Equal(t, int64(1), s.counters[switchboardKeepAliveMeetingpoint].Get())
	require.Equal(t, int64(1), s.counters[switchboardKeepAliveMeetingpointErrors].Get())
}

func TestClearMeetingPoint_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	meetingPoint, err := s.ReserveMeetingPoint(ctx, machineID, userName)
	require.NoError(t, err)

	meetingPoint, err = s.GetMeetingPoint(ctx, machineID)
	require.NoError(t, err)

	err = s.ClearMeetingPoint(ctx, meetingPoint)
	require.NoError(t, err)

	meetingPoint, err = s.GetMeetingPoint(ctx, machineID)
	require.Error(t, err)
	require.Contains(t, err.Error(), ErrMachineNotFound.Error())

	require.Equal(t, int64(1), s.counters[switchboardClearMeetingpoint].Get())
	require.Equal(t, int64(2), s.counters[switchboardGetMeetingpoint].Get())
	require.Equal(t, int64(1), s.counters[switchboardGetMeetingpointErrors].Get())
}

func TestClearMeetingPoint_NoSuchMeetingPoint_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	err := s.ClearMeetingPoint(ctx, MeetingPoint{
		PodName: podName,
		Port:    portRangeBegin,
	})
	require.NoError(t, err)
}

func TestListMeetingPoints_NoMeetingPointsExist_ReturnsEmptySlice(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Note we haven't added any meeting points, so this should return an empty list.
	meetingPoints, err := s.ListMeetingPoints(ctx)
	require.NoError(t, err)
	require.Empty(t, meetingPoints)
	require.Equal(t, int64(1), s.counters[switchboardListMeetingpoint].Get())
	require.Equal(t, int64(0), s.counters[switchboardListMeetingPointErrors].Get())
}

func TestListMeetingPoints_MeetingPointsExist_ReturnsMeetingPoints(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	meetingPoint, err := s.ReserveMeetingPoint(ctx, machineID, userName)
	require.NoError(t, err)

	meetingPoints, err := s.ListMeetingPoints(ctx)
	require.NoError(t, err)
	require.Len(t, meetingPoints, 1)
	require.Equal(t, meetingPoint, meetingPoints[0])
	require.Equal(t, int64(1), s.counters[switchboardListMeetingpoint].Get())
	require.Equal(t, int64(0), s.counters[switchboardListMeetingPointErrors].Get())
}

func TestNumMeetingPointsForPod_NoMeetingPointsExist_ReturnsZero(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Note we haven't added any meeting points, so this should return 0.
	num, err := s.NumMeetingPointsForPod(ctx, podName)
	require.NoError(t, err)
	require.Equal(t, 0, num)
	require.Equal(t, int64(1), s.counters[switchboardNumMeetingpointsForPod].Get())
	require.Equal(t, int64(0), s.counters[switchboardNumMeetingPointsForPodErrors].Get())
}

func TestNumMeetingPointsForPod_MeetingPointExists_ReturnsOne(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	_, err = s.ReserveMeetingPoint(ctx, machineID, userName)
	require.NoError(t, err)

	num, err := s.NumMeetingPointsForPod(ctx, podName)
	require.NoError(t, err)
	require.Equal(t, num, 1)
	require.Equal(t, int64(1), s.counters[switchboardNumMeetingpointsForPod].Get())
	require.Equal(t, int64(0), s.counters[switchboardNumMeetingPointsForPodErrors].Get())
}

func TestNumMeetingPointsForPod_MeetingPointsExistButNoneMatchThePodName_ReturnZero(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	_, err = s.ReserveMeetingPoint(ctx, machineID, userName)
	require.NoError(t, err)

	num, err := s.NumMeetingPointsForPod(ctx, "not a known pod name")
	require.NoError(t, err)
	require.Equal(t, num, 0)
	require.Equal(t, int64(1), s.counters[switchboardNumMeetingpointsForPod].Get())
	require.Equal(t, int64(0), s.counters[switchboardNumMeetingPointsForPodErrors].Get())

}

func TestIsValidPod(t *testing.T) {
	unittest.LargeTest(t)
	ctx, s := setupForTest(t)

	// Add a pod.
	err := s.AddPod(ctx, podName)
	require.NoError(t, err)

	require.True(t, s.IsValidPod(ctx, podName))
	require.False(t, s.IsValidPod(ctx, "this is not a valid pod name"))
	require.Equal(t, int64(2), s.counters[switchboardIsValidPod].Get())
}
