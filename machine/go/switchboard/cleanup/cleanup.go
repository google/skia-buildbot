// Package cleanup provides a worker that cleans up stale MeetingPoints.
package cleanup

import (
	"context"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/switchboard"
)

const defaultRefreshDuration = time.Minute

// Cleanup provides a worker that cleans up stale MeetingPoints.
type Cleanup struct {
	switchboard              switchboard.Switchboard
	refreshDuration          time.Duration
	totalMeetingPoints       metrics2.Int64Metric
	listMeetingPointsFailed  metrics2.Counter
	clearMeetingPointsFailed metrics2.Counter
	liveness                 metrics2.Liveness
}

// New returns a new Cleanup instance.
func New(switchboard switchboard.Switchboard) *Cleanup {
	return &Cleanup{
		switchboard:              switchboard,
		refreshDuration:          defaultRefreshDuration,
		totalMeetingPoints:       metrics2.GetInt64Metric("machine_switchboard_cleanup_total_meetingpoints"),
		listMeetingPointsFailed:  metrics2.GetCounter("machine_switchboard_cleanup_list_meetingpoints_failed"),
		clearMeetingPointsFailed: metrics2.GetCounter("machine_switchboard_cleanup_clear_meetingpoints_failed"),
		liveness:                 metrics2.NewLiveness("machine_switchboard_cleanup"),
	}
}

// Start a process of cleaning up stale MeetingPoints.
//
// This function should never return, unless the context is cancelled.
func (c *Cleanup) Start(ctx context.Context) {
	ticker := time.NewTicker(c.refreshDuration)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mps, err := c.switchboard.ListMeetingPoints(ctx)
			if err != nil {
				c.listMeetingPointsFailed.Inc(1)
				sklog.Errorf("Failed to list MeetingPoints: %s", err)
				continue
			}
			c.totalMeetingPoints.Update(int64(len(mps)))
			cutoff := now.Now(ctx).Add(-2 * switchboard.MeetingPointKeepAliveDuration)
			numErrors := 0
			for _, mp := range mps {
				if mp.LastUpdated.Before(cutoff) {
					err := c.switchboard.ClearMeetingPoint(ctx, mp)
					if err != nil {
						c.clearMeetingPointsFailed.Inc(1)
						sklog.Errorf("Failed to delete MeetingPoint that was stale %v: %s", mp, err)
						numErrors++
					}
				}
			}
			if numErrors == 0 {
				c.liveness.Reset()
			}
		}
	}
}
