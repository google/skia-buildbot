// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package engine

import (
	"testing"
	"time"

	"github.com/luci/luci-go/cron/appengine/schedule"
	. "github.com/smartystreets/goconvey/convey"
)

var epoch = time.Unix(1442270520, 0).UTC()

func TestStateMachine(t *testing.T) {
	Convey("Normal flow on abs schedule", t, func() {
		m := newTestStateMachine("*/5 * * * * * *")

		// Noop when in disabled state.
		So(m.roll(func(sm *StateMachine) error { return sm.OnTimerTick(1) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateDisabled)
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarting(1, 1) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateDisabled)
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarted(1) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateDisabled)
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationDone(1) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateDisabled)
		So(m.roll(func(sm *StateMachine) error { return sm.OnScheduleChange() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateDisabled)

		// Enabling schedules a tick.
		So(m.roll(func(sm *StateMachine) error { return sm.OnJobEnabled() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(5 * time.Second), 1},
		})
		m.actions = nil

		// Re enabling does nothing.
		So(m.roll(func(sm *StateMachine) error { return sm.OnJobEnabled() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.actions, ShouldBeNil)

		// Wrong tick ID is skipped.
		So(m.roll(func(sm *StateMachine) error { return sm.OnTimerTick(2) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.actions, ShouldBeNil)

		// Early tick causes retry.
		So(m.roll(func(sm *StateMachine) error { return sm.OnTimerTick(1) }), ShouldNotBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.actions, ShouldBeNil)

		// Tick on time moves to JobStateQueued, schedules next tick.
		m.now = m.now.Add(5 * time.Second)
		So(m.roll(func(sm *StateMachine) error { return sm.OnTimerTick(1) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(10 * time.Second), 2},
			StartInvocationAction{InvocationNonce: 3},
		})
		m.actions = nil

		// Wrong invocation nonce is skipped.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarting(333, 1000) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)
		So(m.actions, ShouldBeNil)

		// Time to run.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarting(3, 1000) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)
		So(m.state.InvocationID, ShouldEqual, 1000)

		// Skip wrong invocation ID.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarted(1001) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)

		// Started.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarted(1000) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateRunning)

		// Skip wrong invocation ID.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationDone(1001) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateRunning)

		// End of the cycle. Ends up in scheduled state, waiting for the tick added
		// when StartInvocationAction was issued.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationDone(1000) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.state.TickNonce, ShouldEqual, 2)

		// Disable cancels the timer.
		So(m.roll(func(sm *StateMachine) error { return sm.OnJobDisabled() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateDisabled)
		So(m.state.InvocationNonce, ShouldEqual, 0)
	})

	Convey("Overrun when queued", t, func() {
		m := newTestStateMachine("*/5 * * * * * *")

		// Enabling schedules a tick.
		So(m.roll(func(sm *StateMachine) error { return sm.OnJobEnabled() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(5 * time.Second), 1},
		})
		m.actions = nil

		// Tick on time moves to JobStateQueued
		m.now = m.now.Add(5 * time.Second)
		So(m.roll(func(sm *StateMachine) error { return sm.OnTimerTick(1) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(10 * time.Second), 2},
			StartInvocationAction{InvocationNonce: 3},
		})
		m.actions = nil

		// Overrun when queued.
		m.now = m.now.Add(5 * time.Second)
		So(m.roll(func(sm *StateMachine) error { return sm.OnTimerTick(2) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateSlowQueue)
		So(m.state.Overruns, ShouldEqual, 1)
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(15 * time.Second), 4},
			RecordOverrunAction{Overruns: 1},
		})
		m.actions = nil

		// Time to run. Moves to JobStateOverrun because was stuck in queue.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarting(3, 100) }), ShouldBeNil)
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarted(100) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateOverrun)
		So(m.state.Overruns, ShouldEqual, 1)

		// End of the cycle.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationDone(100) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
	})

	Convey("Overrun when running", t, func() {
		m := newTestStateMachine("*/5 * * * * * *")

		// Enabling schedules a tick.
		So(m.roll(func(sm *StateMachine) error { return sm.OnJobEnabled() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(5 * time.Second), 1},
		})
		m.actions = nil

		// Tick on time moves to JobStateQueued
		m.now = m.now.Add(5 * time.Second)
		So(m.roll(func(sm *StateMachine) error { return sm.OnTimerTick(1) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(10 * time.Second), 2},
			StartInvocationAction{InvocationNonce: 3},
		})
		m.actions = nil

		// Time to run.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarting(3, 100) }), ShouldBeNil)
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarted(100) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateRunning)

		// Next tick comes while job is running.
		m.now = m.now.Add(5 * time.Second)
		So(m.roll(func(sm *StateMachine) error { return sm.OnTimerTick(2) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateOverrun)
		So(m.state.Overruns, ShouldEqual, 1)
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(15 * time.Second), 4},
			RecordOverrunAction{Overruns: 1, RunningInvocationID: 100},
		})
		m.actions = nil

		// End of the cycle.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationDone(100) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
	})

	Convey("Normal flow on rel schedule", t, func() {
		m := newTestStateMachine("with 10s interval")

		// Enabling schedules the first tick at some random moment in time.
		So(m.roll(func(sm *StateMachine) error { return sm.OnJobEnabled() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(9*time.Second + 451961492*time.Nanosecond), 1},
		})
		m.actions = nil

		// Tick on time moves to JobStateQueued and does NOT schedule a tick.
		m.now = m.now.Add(9*time.Second + 451961492*time.Nanosecond)
		So(m.roll(func(sm *StateMachine) error { return sm.OnTimerTick(1) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)
		So(m.state.TickNonce, ShouldEqual, 0)
		So(m.actions, ShouldResemble, []Action{
			StartInvocationAction{InvocationNonce: 2},
		})
		m.actions = nil

		// Time to run.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarting(2, 1000) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)
		So(m.state.InvocationID, ShouldEqual, 1000)

		// Started.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarted(1000) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateRunning)

		// Let it run for some time.
		m.now = epoch.Add(20 * time.Second)

		// End of the cycle. New tick is scheduled, 10s from current time.
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationDone(1000) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.state.TickNonce, ShouldEqual, 3)
		So(m.state.TickTime, ShouldResemble, m.now.Add(10*time.Second))
	})

	Convey("Schedule changes", t, func() {
		m := newTestStateMachine("*/5 * * * * * *")

		// Enabling schedules a tick after 5 sec.
		So(m.roll(func(sm *StateMachine) error { return sm.OnJobEnabled() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.state.TickNonce, ShouldEqual, 1)
		So(m.state.TickTime, ShouldResemble, epoch.Add(5*time.Second))
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(5 * time.Second), 1},
		})
		m.actions = nil

		// Rescheduling event with same next tick time is noop.
		So(m.roll(func(sm *StateMachine) error { return sm.OnScheduleChange() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.state.TickNonce, ShouldEqual, 1)
		So(m.state.TickTime, ShouldResemble, epoch.Add(5*time.Second))
		So(m.actions, ShouldBeNil)

		// Suddenly ticking each second.
		sched, _ := schedule.Parse("*/1 * * * * * *", 0)
		m.schedule = sched

		// Should be rescheduled to run earlier
		So(m.roll(func(sm *StateMachine) error { return sm.OnScheduleChange() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.state.TickNonce, ShouldEqual, 2)
		So(m.state.TickTime, ShouldResemble, epoch.Add(1*time.Second))
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(1 * time.Second), 2},
		})
		m.actions = nil

		// Starts running.
		m.now = m.now.Add(time.Second)
		So(m.roll(func(sm *StateMachine) error { return sm.OnTimerTick(2) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)
		So(m.state.InvocationNonce, ShouldEqual, 4)
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarting(4, 123) }), ShouldBeNil)
		So(m.roll(func(sm *StateMachine) error { return sm.OnInvocationStarted(123) }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateRunning)

		// Next tick is scheduled based on absolute schedule.
		So(m.state.TickNonce, ShouldEqual, 3)
		So(m.state.TickTime, ShouldResemble, epoch.Add(2*time.Second))
		m.actions = nil

		// Switching to use relative schedule (while job is still running).
		sched, _ = schedule.Parse("with 20s interval", 0)
		m.schedule = sched

		// Cancels pending tick.
		So(m.roll(func(sm *StateMachine) error { return sm.OnScheduleChange() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateRunning)
		So(m.state.TickNonce, ShouldEqual, 0)
		So(m.actions, ShouldBeNil)

		// Switching back to absolute schedule.
		sched, _ = schedule.Parse("*/1 * * * * * *", 0)
		m.schedule = sched

		// Reschedules the tick again.
		So(m.roll(func(sm *StateMachine) error { return sm.OnScheduleChange() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateRunning)
		So(m.state.TickNonce, ShouldEqual, 5)
		So(m.state.TickTime, ShouldResemble, epoch.Add(2*time.Second))
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(2 * time.Second), 5},
		})
		m.actions = nil
	})

	Convey("OnManualInvocation works with abs schedule", t, func() {
		m := newTestStateMachine("*/5 * * * * * *")

		// Enabling schedules a tick after 5 sec.
		So(m.roll(func(sm *StateMachine) error { return sm.OnJobEnabled() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.actions, ShouldResemble, []Action{
			TickLaterAction{epoch.Add(5 * time.Second), 1},
		})
		m.actions = nil

		// Asking to run the job works. It doesn't touch job's schedule.
		So(m.roll(func(sm *StateMachine) error { return sm.OnManualInvocation("user:abc") }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)
		So(m.actions, ShouldResemble, []Action{
			StartInvocationAction{
				InvocationNonce: 2,
				TriggeredBy:     "user:abc",
			},
		})
		m.actions = nil

		// Second call doesn't work. The job is queued already.
		err := m.roll(func(sm *StateMachine) error { return sm.OnManualInvocation("user:abc") })
		So(err, ShouldNotBeNil)
	})

	Convey("OnManualInvocation works with rel schedule", t, func() {
		m := newTestStateMachine("with 5s interval")

		// Enabling schedules a tick after random amount of seconds.
		So(m.roll(func(sm *StateMachine) error { return sm.OnJobEnabled() }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateScheduled)
		So(m.state.TickNonce, ShouldEqual, 1)
		m.actions = nil

		// Asking to run the job works. It resets the tick (to be enabled again when
		// job finishes).
		So(m.roll(func(sm *StateMachine) error { return sm.OnManualInvocation("user:abc") }), ShouldBeNil)
		So(m.state.State, ShouldEqual, JobStateQueued)
		So(m.state.TickNonce, ShouldEqual, 0) // reset
		So(m.actions, ShouldResemble, []Action{
			StartInvocationAction{
				InvocationNonce: 2,
				TriggeredBy:     "user:abc",
			},
		})

		m.actions = nil

		// Second call doesn't work. The job is queued already.
		err := m.roll(func(sm *StateMachine) error { return sm.OnManualInvocation("user:abc") })
		So(err, ShouldNotBeNil)
	})
}

type testStateMachine struct {
	state    JobState
	now      time.Time
	nonce    int64
	schedule *schedule.Schedule
	actions  []Action
}

func newTestStateMachine(scheduleExpr string) *testStateMachine {
	// Each 5 sec.
	sched, _ := schedule.Parse(scheduleExpr, 0)
	return &testStateMachine{
		state: JobState{
			State: JobStateDisabled,
		},
		now:      epoch,
		schedule: sched,
	}
}

func (t *testStateMachine) roll(cb func(sm *StateMachine) error) error {
	nonce := t.nonce
	sm := StateMachine{
		State:    t.state,
		Now:      t.now,
		Schedule: t.schedule,
		Nonce: func() int64 {
			nonce++
			return nonce
		},
	}
	if err := cb(&sm); err != nil {
		return err
	}
	t.state = sm.State
	t.nonce = nonce
	t.actions = append(t.actions, sm.Actions...)
	return nil
}
