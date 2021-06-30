// Package cleanup provides a worker that cleans up stale MeetingPoints.
package cleanup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/switchboard"
	"go.skia.org/infra/machine/go/switchboard/mocks"
)

var (
	errMyMockError = errors.New("my mock error")

	mockTime = time.Unix(0, 0).UTC()

	meetingPoint = switchboard.MeetingPoint{}
)

func TestCleanup_ExitsOnCancelledContext(t *testing.T) {
	unittest.SmallTest(t)
	s := &mocks.Switchboard{}
	c := New(s)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Start(ctx)
	// Test passes if it doesn't timeout.
}

func TestCleanup_ListMeetingPointsFails_ErrorReflectedInMetrics(t *testing.T) {
	unittest.SmallTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	s := &mocks.Switchboard{}
	s.On("ListMeetingPoints", testutils.AnyContext).Return(nil, errMyMockError).Run(func(args mock.Arguments) {
		cancel()
	})

	c := New(s)
	c.totalMeetingPoints.Update(0)
	c.listMeetingPointsFailed.Reset()
	c.refreshDuration = time.Microsecond
	c.Start(ctx)
	mock.AssertExpectationsForObjects(t, s)
	require.GreaterOrEqual(t, c.listMeetingPointsFailed.Get(), int64(1))
	require.Equal(t, int64(0), c.totalMeetingPoints.Get())
}

func TestCleanup_ClearMeetingPointsFails_ErrorReflectedInMetrics(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.WithValue(context.Background(), now.ContextKey, mockTime)
	ctx, cancel := context.WithCancel(ctx)

	s := &mocks.Switchboard{}
	// Set the MeetingPoint.LastUpdated back in time long enough to be considered stale.
	meetingPoint.LastUpdated = mockTime.Add(-3 * switchboard.MeetingPointKeepAliveDuration)
	s.On("ListMeetingPoints", testutils.AnyContext).Return([]switchboard.MeetingPoint{meetingPoint}, nil)
	s.On("ClearMeetingPoint", testutils.AnyContext, meetingPoint).Return(errMyMockError).Run(func(args mock.Arguments) {
		cancel()
	})

	c := New(s)
	c.listMeetingPointsFailed.Reset()
	c.clearMeetingPointsFailed.Reset()
	c.totalMeetingPoints.Update(0)
	c.refreshDuration = time.Microsecond
	c.Start(ctx)
	mock.AssertExpectationsForObjects(t, s)
	require.Equal(t, int64(1), c.totalMeetingPoints.Get())
	require.Equal(t, int64(0), c.listMeetingPointsFailed.Get())
	require.GreaterOrEqual(t, c.clearMeetingPointsFailed.Get(), int64(1))
}
