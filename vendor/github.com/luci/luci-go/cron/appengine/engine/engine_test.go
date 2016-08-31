// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package engine

import (
	"encoding/json"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/api/pubsub/v1"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/taskqueue"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/rand/mathrand"
	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/server/secrets/testsecrets"

	"github.com/luci/luci-go/cron/appengine/catalog"
	"github.com/luci/luci-go/cron/appengine/messages"
	"github.com/luci/luci-go/cron/appengine/task"
	"github.com/luci/luci-go/cron/appengine/task/noop"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetAllProjects(t *testing.T) {
	Convey("works", t, func() {
		c := newTestContext(epoch)
		e, _ := newTestEngine()
		ds := datastore.Get(c)

		// Empty.
		projects, err := e.GetAllProjects(c)
		So(err, ShouldBeNil)
		So(len(projects), ShouldEqual, 0)

		// Non empty.
		So(ds.Put(
			&CronJob{JobID: "abc/1", ProjectID: "abc", Enabled: true},
			&CronJob{JobID: "abc/2", ProjectID: "abc", Enabled: true},
			&CronJob{JobID: "def/1", ProjectID: "def", Enabled: true},
			&CronJob{JobID: "xyz/1", ProjectID: "xyz", Enabled: false},
		), ShouldBeNil)
		ds.Testable().CatchupIndexes()
		projects, err = e.GetAllProjects(c)
		So(err, ShouldBeNil)
		So(projects, ShouldResemble, []string{"abc", "def"})
	})
}

func TestUpdateProjectJobs(t *testing.T) {
	Convey("works", t, func() {
		c := newTestContext(epoch)
		e, _ := newTestEngine()

		// Doing nothing.
		So(e.UpdateProjectJobs(c, "abc", []catalog.Definition{}), ShouldBeNil)
		So(allJobs(c), ShouldResemble, []CronJob{})

		// Adding a new job (ticks every 5 sec).
		So(e.UpdateProjectJobs(c, "abc", []catalog.Definition{
			{
				JobID:    "abc/1",
				Revision: "rev1",
				Schedule: "*/5 * * * * * *",
			}}), ShouldBeNil)
		So(allJobs(c), ShouldResemble, []CronJob{
			{
				JobID:     "abc/1",
				ProjectID: "abc",
				Revision:  "rev1",
				Enabled:   true,
				Schedule:  "*/5 * * * * * *",
				State: JobState{
					State:     "SCHEDULED",
					TickNonce: 6278013164014963328,
					TickTime:  epoch.Add(5 * time.Second),
				},
			},
		})
		// Enqueued timer task to launch it.
		task := ensureOneTask(c, "timers-q")
		So(task.Path, ShouldEqual, "/timers")
		So(task.ETA, ShouldResemble, epoch.Add(5*time.Second))
		taskqueue.Get(c).Testable().ResetTasks()

		// Readding same job in with exact same config revision -> noop.
		So(e.UpdateProjectJobs(c, "abc", []catalog.Definition{
			{
				JobID:    "abc/1",
				Revision: "rev1",
				Schedule: "*/5 * * * * * *",
			}}), ShouldBeNil)
		ensureZeroTasks(c, "timers-q")
		ensureZeroTasks(c, "invs-q")

		// Changing schedule to tick earlier -> rescheduled.
		So(e.UpdateProjectJobs(c, "abc", []catalog.Definition{
			{
				JobID:    "abc/1",
				Revision: "rev2",
				Schedule: "*/1 * * * * * *",
			}}), ShouldBeNil)
		So(allJobs(c), ShouldResemble, []CronJob{
			{
				JobID:     "abc/1",
				ProjectID: "abc",
				Revision:  "rev2",
				Enabled:   true,
				Schedule:  "*/1 * * * * * *",
				State: JobState{
					State:     "SCHEDULED",
					TickNonce: 9111178027324032851,
					TickTime:  epoch.Add(1 * time.Second),
				},
			},
		})
		// Enqueued timer task to launch it.
		task = ensureOneTask(c, "timers-q")
		So(task.Path, ShouldEqual, "/timers")
		So(task.ETA, ShouldResemble, epoch.Add(1*time.Second))
		taskqueue.Get(c).Testable().ResetTasks()

		// Removed -> goes to disabled state.
		So(e.UpdateProjectJobs(c, "abc", []catalog.Definition{}), ShouldBeNil)
		So(allJobs(c), ShouldResemble, []CronJob{
			{
				JobID:     "abc/1",
				ProjectID: "abc",
				Revision:  "rev2",
				Enabled:   false,
				Schedule:  "*/1 * * * * * *",
				State: JobState{
					State: "DISABLED",
				},
			},
		})
		ensureZeroTasks(c, "timers-q")
		ensureZeroTasks(c, "invs-q")
	})
}

func TestTransactionRetries(t *testing.T) {
	Convey("retry works", t, func() {
		c := newTestContext(epoch)
		e, _ := newTestEngine()

		// Adding a new job with transaction retry, should enqueue one task.
		datastore.Get(c).Testable().SetTransactionRetryCount(2)
		So(e.UpdateProjectJobs(c, "abc", []catalog.Definition{
			{
				JobID:    "abc/1",
				Revision: "rev1",
				Schedule: "*/5 * * * * * *",
			}}), ShouldBeNil)
		So(allJobs(c), ShouldResemble, []CronJob{
			{
				JobID:     "abc/1",
				ProjectID: "abc",
				Revision:  "rev1",
				Enabled:   true,
				Schedule:  "*/5 * * * * * *",
				State: JobState{
					State:     "SCHEDULED",
					TickNonce: 1907242367099883828,
					TickTime:  epoch.Add(5 * time.Second),
				},
			},
		})
		// Enqueued timer task to launch it.
		task := ensureOneTask(c, "timers-q")
		So(task.Path, ShouldEqual, "/timers")
		So(task.ETA, ShouldResemble, epoch.Add(5*time.Second))
		taskqueue.Get(c).Testable().ResetTasks()
	})

	Convey("collision is handled", t, func() {
		c := newTestContext(epoch)
		e, _ := newTestEngine()

		// Pretend collision happened in all retries.
		datastore.Get(c).Testable().SetTransactionRetryCount(15)
		err := e.UpdateProjectJobs(c, "abc", []catalog.Definition{
			{
				JobID:    "abc/1",
				Revision: "rev1",
				Schedule: "*/5 * * * * * *",
			}})
		So(errors.IsTransient(err), ShouldBeTrue)
		So(allJobs(c), ShouldResemble, []CronJob{})
		ensureZeroTasks(c, "timers-q")
		ensureZeroTasks(c, "invs-q")
	})
}

func TestResetAllJobsOnDevServer(t *testing.T) {
	Convey("works", t, func() {
		c := newTestContext(epoch)
		e, _ := newTestEngine()

		So(e.UpdateProjectJobs(c, "abc", []catalog.Definition{
			{
				JobID:    "abc/1",
				Revision: "rev1",
				Schedule: "*/5 * * * * * *",
			}}), ShouldBeNil)
		So(allJobs(c), ShouldResemble, []CronJob{
			{
				JobID:     "abc/1",
				ProjectID: "abc",
				Revision:  "rev1",
				Enabled:   true,
				Schedule:  "*/5 * * * * * *",
				State: JobState{
					State:     "SCHEDULED",
					TickNonce: 6278013164014963328,
					TickTime:  epoch.Add(5 * time.Second),
				},
			},
		})

		clock.Get(c).(testclock.TestClock).Add(1 * time.Minute)

		// ResetAllJobsOnDevServer should reschedule the job.
		So(e.ResetAllJobsOnDevServer(c), ShouldBeNil)
		So(allJobs(c), ShouldResemble, []CronJob{
			{
				JobID:     "abc/1",
				ProjectID: "abc",
				Revision:  "rev1",
				Enabled:   true,
				Schedule:  "*/5 * * * * * *",
				State: JobState{
					State:     "SCHEDULED",
					TickNonce: 9111178027324032851,
					TickTime:  epoch.Add(65 * time.Second),
				},
			},
		})
	})
}

func TestFullFlow(t *testing.T) {
	Convey("full flow", t, func() {
		c := newTestContext(epoch)
		e, mgr := newTestEngine()
		taskBytes := noopTaskBytes()

		// Adding a new job (ticks every 5 sec).
		So(e.UpdateProjectJobs(c, "abc", []catalog.Definition{
			{
				JobID:    "abc/1",
				Revision: "rev1",
				Schedule: "*/5 * * * * * *",
				Task:     taskBytes,
			}}), ShouldBeNil)
		So(allJobs(c), ShouldResemble, []CronJob{
			{
				JobID:     "abc/1",
				ProjectID: "abc",
				Revision:  "rev1",
				Enabled:   true,
				Schedule:  "*/5 * * * * * *",
				Task:      taskBytes,
				State: JobState{
					State:     "SCHEDULED",
					TickNonce: 6278013164014963328,
					TickTime:  epoch.Add(5 * time.Second),
				},
			},
		})
		// Enqueued timer task to launch it.
		tsk := ensureOneTask(c, "timers-q")
		So(tsk.Path, ShouldEqual, "/timers")
		So(tsk.ETA, ShouldResemble, epoch.Add(5*time.Second))
		taskqueue.Get(c).Testable().ResetTasks()

		// Tick time comes, the tick task is executed, job is added to queue.
		clock.Get(c).(testclock.TestClock).Add(5 * time.Second)
		So(e.ExecuteSerializedAction(c, tsk.Payload, 0), ShouldBeNil)

		// Job is in queued state now.
		So(allJobs(c), ShouldResemble, []CronJob{
			{
				JobID:     "abc/1",
				ProjectID: "abc",
				Revision:  "rev1",
				Enabled:   true,
				Schedule:  "*/5 * * * * * *",
				Task:      taskBytes,
				State: JobState{
					State:           "QUEUED",
					TickNonce:       9111178027324032851,
					TickTime:        epoch.Add(10 * time.Second),
					InvocationNonce: 631000787647335445,
					InvocationTime:  epoch.Add(5 * time.Second),
				},
			},
		})

		// Next tick task is added.
		tickTask := ensureOneTask(c, "timers-q")
		So(tickTask.Path, ShouldEqual, "/timers")
		So(tickTask.ETA, ShouldResemble, epoch.Add(10*time.Second))

		// Invocation task (ETA is 1 sec in the future).
		invTask := ensureOneTask(c, "invs-q")
		So(invTask.Path, ShouldEqual, "/invs")
		So(invTask.ETA, ShouldResemble, epoch.Add(6*time.Second))
		taskqueue.Get(c).Testable().ResetTasks()

		// Time to run the job and it fails to launch with a transient error.
		mgr.launchTask = func(ctl task.Controller) error {
			// Check data provided via the controller.
			So(ctl.JobID(), ShouldEqual, "abc/1")
			So(ctl.InvocationID(), ShouldEqual, int64(9200093518582666224))
			So(ctl.InvocationNonce(), ShouldEqual, int64(631000787647335445))
			So(ctl.Task(), ShouldResemble, &messages.NoopTask{})

			ctl.DebugLog("oops, fail")
			return errors.WrapTransient(errors.New("oops"))
		}
		So(errors.IsTransient(e.ExecuteSerializedAction(c, invTask.Payload, 0)), ShouldBeTrue)

		// Still in QUEUED state, but with InvocatioID assigned.
		jobs := allJobs(c)
		So(jobs, ShouldResemble, []CronJob{
			{
				JobID:     "abc/1",
				ProjectID: "abc",
				Revision:  "rev1",
				Enabled:   true,
				Schedule:  "*/5 * * * * * *",
				Task:      taskBytes,
				State: JobState{
					State:           "QUEUED",
					TickNonce:       9111178027324032851,
					TickTime:        epoch.Add(10 * time.Second),
					InvocationNonce: 631000787647335445,
					InvocationTime:  epoch.Add(5 * time.Second),
					InvocationID:    9200093518582666224,
				},
			},
		})
		jobKey := datastore.Get(c).KeyForObj(&jobs[0])

		// Check Invocation fields.
		inv := Invocation{ID: 9200093518582666224, JobKey: jobKey}
		So(datastore.Get(c).Get(&inv), ShouldBeNil)
		inv.JobKey = nil // for easier ShouldResemble below
		So(inv, ShouldResemble, Invocation{
			ID:              9200093518582666224,
			InvocationNonce: 631000787647335445,
			Revision:        "rev1",
			Started:         epoch.Add(5 * time.Second),
			Finished:        epoch.Add(5 * time.Second),
			Task:            taskBytes,
			DebugLog: "[22:42:05.000] Invocation initiated (attempt 1)\n" +
				"[22:42:05.000] oops, fail\n" +
				"[22:42:05.000] Invocation finished in 0 with status FAILED\n" +
				"[22:42:05.000] It will probably be retried\n",
			Status:         task.StatusFailed,
			MutationsCount: 1,
		})

		// Second attempt. Now starts, hangs midway, they finishes.
		mgr.launchTask = func(ctl task.Controller) error {
			// Make sure Save() checkpoints the progress.
			ctl.DebugLog("Starting")
			ctl.State().Status = task.StatusRunning
			So(ctl.Save(), ShouldBeNil)

			// After first Save the job and the invocation are in running state.
			So(allJobs(c), ShouldResemble, []CronJob{
				{
					JobID:     "abc/1",
					ProjectID: "abc",
					Revision:  "rev1",
					Enabled:   true,
					Schedule:  "*/5 * * * * * *",
					Task:      taskBytes,
					State: JobState{
						State:           "RUNNING",
						TickNonce:       9111178027324032851,
						TickTime:        epoch.Add(10 * time.Second),
						InvocationNonce: 631000787647335445,
						InvocationTime:  epoch.Add(5 * time.Second),
						InvocationID:    9200093518581789696,
					},
				},
			})
			inv := Invocation{ID: 9200093518581789696, JobKey: jobKey}
			So(datastore.Get(c).Get(&inv), ShouldBeNil)
			inv.JobKey = nil // for easier ShouldResemble below
			So(inv, ShouldResemble, Invocation{
				ID:              9200093518581789696,
				InvocationNonce: 631000787647335445,
				Revision:        "rev1",
				Started:         epoch.Add(5 * time.Second),
				Task:            taskBytes,
				DebugLog:        "[22:42:05.000] Invocation initiated (attempt 2)\n[22:42:05.000] Starting\n",
				RetryCount:      1,
				Status:          task.StatusRunning,
				MutationsCount:  1,
			})

			// Noop save, just for the code coverage.
			So(ctl.Save(), ShouldBeNil)

			// Change state to the final one.
			ctl.State().Status = task.StatusSucceeded
			ctl.State().ViewURL = "http://view_url"
			ctl.State().TaskData = []byte("blah")
			return nil
		}
		So(e.ExecuteSerializedAction(c, invTask.Payload, 1), ShouldBeNil)

		// After final save.
		inv = Invocation{ID: 9200093518581789696, JobKey: jobKey}
		So(datastore.Get(c).Get(&inv), ShouldBeNil)
		inv.JobKey = nil // for easier ShouldResemble below
		So(inv, ShouldResemble, Invocation{
			ID:              9200093518581789696,
			InvocationNonce: 631000787647335445,
			Revision:        "rev1",
			Started:         epoch.Add(5 * time.Second),
			Finished:        epoch.Add(5 * time.Second),
			Task:            taskBytes,
			DebugLog: "[22:42:05.000] Invocation initiated (attempt 2)\n" +
				"[22:42:05.000] Starting\n" +
				"[22:42:05.000] Invocation finished in 0 with status SUCCEEDED\n",
			RetryCount:     1,
			Status:         task.StatusSucceeded,
			ViewURL:        "http://view_url",
			TaskData:       []byte("blah"),
			MutationsCount: 2,
		})

		// Previous invocation is canceled.
		inv = Invocation{ID: 9200093518582666224, JobKey: jobKey}
		So(datastore.Get(c).Get(&inv), ShouldBeNil)
		inv.JobKey = nil // for easier ShouldResemble below
		So(inv, ShouldResemble, Invocation{
			ID:              9200093518582666224,
			InvocationNonce: 631000787647335445,
			Revision:        "rev1",
			Started:         epoch.Add(5 * time.Second),
			Finished:        epoch.Add(5 * time.Second),
			Task:            taskBytes,
			DebugLog: "[22:42:05.000] Invocation initiated (attempt 1)\n" +
				"[22:42:05.000] oops, fail\n" +
				"[22:42:05.000] Invocation finished in 0 with status FAILED\n" +
				"[22:42:05.000] It will probably be retried\n",
			Status:         task.StatusFailed,
			MutationsCount: 1,
		})

		// Job is in scheduled state again.
		So(allJobs(c), ShouldResemble, []CronJob{
			{
				JobID:     "abc/1",
				ProjectID: "abc",
				Revision:  "rev1",
				Enabled:   true,
				Schedule:  "*/5 * * * * * *",
				Task:      taskBytes,
				State: JobState{
					State:     "SCHEDULED",
					TickNonce: 9111178027324032851,
					TickTime:  epoch.Add(10 * time.Second),
					PrevTime:  epoch.Add(5 * time.Second),
				},
			},
		})
	})
}

func TestGenerateInvocationID(t *testing.T) {
	Convey("generateInvocationID does not collide", t, func() {
		c := newTestContext(epoch)
		k := datastore.Get(c).NewKey("CronJob", "", 123, nil)

		// Bunch of ids generated at the exact same moment in time do not collide.
		ids := map[int64]struct{}{}
		for i := 0; i < 20; i++ {
			id, err := generateInvocationID(c, k)
			So(err, ShouldBeNil)
			ids[id] = struct{}{}
		}
		So(len(ids), ShouldEqual, 20)
	})

	Convey("generateInvocationID gen IDs with most recent first", t, func() {
		c := newTestContext(epoch)
		k := datastore.Get(c).NewKey("CronJob", "", 123, nil)

		older, err := generateInvocationID(c, k)
		So(err, ShouldBeNil)

		clock.Get(c).(testclock.TestClock).Add(5 * time.Second)

		newer, err := generateInvocationID(c, k)
		So(err, ShouldBeNil)

		So(newer, ShouldBeLessThan, older)
	})
}

func TestQueries(t *testing.T) {
	Convey("with mock data", t, func() {
		c := newTestContext(epoch)
		e, _ := newTestEngine()
		ds := datastore.Get(c)

		So(ds.Put(
			&CronJob{JobID: "abc/1", ProjectID: "abc", Enabled: true},
			&CronJob{JobID: "abc/2", ProjectID: "abc", Enabled: true},
			&CronJob{JobID: "def/1", ProjectID: "def", Enabled: true},
			&CronJob{JobID: "def/2", ProjectID: "def", Enabled: false},
		), ShouldBeNil)

		job1 := ds.NewKey("CronJob", "abc/1", 0, nil)
		job2 := ds.NewKey("CronJob", "abc/2", 0, nil)
		So(ds.Put(
			&Invocation{ID: 1, JobKey: job1, InvocationNonce: 123},
			&Invocation{ID: 2, JobKey: job1, InvocationNonce: 123},
			&Invocation{ID: 3, JobKey: job1},
			&Invocation{ID: 1, JobKey: job2},
			&Invocation{ID: 2, JobKey: job2},
			&Invocation{ID: 3, JobKey: job2},
		), ShouldBeNil)

		ds.Testable().CatchupIndexes()

		Convey("GetAllCronJobs works", func() {
			jobs, err := e.GetAllCronJobs(c)
			So(err, ShouldBeNil)
			ids := stringset.New(0)
			for _, j := range jobs {
				ids.Add(j.JobID)
			}
			asSlice := ids.ToSlice()
			sort.Strings(asSlice)
			So(asSlice, ShouldResemble, []string{"abc/1", "abc/2", "def/1"}) // only enabled
		})

		Convey("GetProjectCronJobs works", func() {
			jobs, err := e.GetProjectCronJobs(c, "def")
			So(err, ShouldBeNil)
			So(len(jobs), ShouldEqual, 1)
			So(jobs[0].JobID, ShouldEqual, "def/1")
		})

		Convey("GetCronJob works", func() {
			job, err := e.GetCronJob(c, "missing/job")
			So(job, ShouldBeNil)
			So(err, ShouldBeNil)

			job, err = e.GetCronJob(c, "abc/1")
			So(job, ShouldNotBeNil)
			So(err, ShouldBeNil)
		})

		Convey("ListInvocations works", func() {
			invs, cursor, err := e.ListInvocations(c, "abc/1", 2, "")
			So(err, ShouldBeNil)
			So(len(invs), ShouldEqual, 2)
			So(invs[0].ID, ShouldEqual, 1)
			So(invs[1].ID, ShouldEqual, 2)
			So(cursor, ShouldNotEqual, "")

			invs, cursor, err = e.ListInvocations(c, "abc/1", 2, cursor)
			So(err, ShouldBeNil)
			So(len(invs), ShouldEqual, 1)
			So(invs[0].ID, ShouldEqual, 3)
			So(cursor, ShouldEqual, "")
		})

		Convey("GetInvocation works", func() {
			inv, err := e.GetInvocation(c, "missing/job", 1)
			So(inv, ShouldBeNil)
			So(err, ShouldBeNil)

			inv, err = e.GetInvocation(c, "abc/1", 1)
			So(inv, ShouldNotBeNil)
			So(err, ShouldBeNil)
		})

		Convey("GetInvocationsByNonce works", func() {
			inv, err := e.GetInvocationsByNonce(c, 11111) // unknown
			So(len(inv), ShouldEqual, 0)
			So(err, ShouldBeNil)

			inv, err = e.GetInvocationsByNonce(c, 123)
			So(len(inv), ShouldEqual, 2)
			So(err, ShouldBeNil)
		})
	})
}

func TestPrepareTopic(t *testing.T) {
	Convey("PrepareTopic works", t, func(ctx C) {
		c := newTestContext(epoch)

		e, _ := newTestEngine()

		pubSubCalls := 0
		e.configureTopic = func(c context.Context, topic, sub, pushURL, publisher string) error {
			pubSubCalls++
			ctx.So(topic, ShouldEqual, "projects/app/topics/dev-cron+noop+some~publisher.com")
			ctx.So(sub, ShouldEqual, "projects/app/subscriptions/dev-cron+noop+some~publisher.com")
			ctx.So(pushURL, ShouldEqual, "") // pull on dev server
			ctx.So(publisher, ShouldEqual, "some@publisher.com")
			return nil
		}

		ctl := &taskController{
			ctx:     c,
			eng:     e,
			manager: &noop.TaskManager{},
			saved: Invocation{
				ID:     123456,
				JobKey: datastore.Get(c).NewKey("CronJob", "job_id", 0, nil),
			},
		}
		ctl.populateState()

		// Once.
		topic, token, err := ctl.PrepareTopic("some@publisher.com")
		So(err, ShouldBeNil)
		So(topic, ShouldEqual, "projects/app/topics/dev-cron+noop+some~publisher.com")
		So(token, ShouldNotEqual, "")
		So(pubSubCalls, ShouldEqual, 1)

		// Again. 'configureTopic' should not be called anymore.
		_, _, err = ctl.PrepareTopic("some@publisher.com")
		So(err, ShouldBeNil)
		So(pubSubCalls, ShouldEqual, 1)

		// Make sure memcache-based deduplication also works.
		e.doneFlags = make(map[string]bool)
		_, _, err = ctl.PrepareTopic("some@publisher.com")
		So(err, ShouldBeNil)
		So(pubSubCalls, ShouldEqual, 1)
	})
}

func TestProcessPubSubPush(t *testing.T) {
	Convey("with mock invocation", t, func() {
		c := newTestContext(epoch)
		e, mgr := newTestEngine()
		ds := datastore.Get(c)

		So(ds.Put(&CronJob{
			JobID:     "abc/1",
			ProjectID: "abc",
			Enabled:   true,
		}), ShouldBeNil)

		task, err := proto.Marshal(&messages.Task{
			Noop: &messages.NoopTask{},
		})
		So(err, ShouldBeNil)

		inv := Invocation{
			ID:     1,
			JobKey: ds.NewKey("CronJob", "abc/1", 0, nil),
			Task:   task,
		}
		So(ds.Put(&inv), ShouldBeNil)

		// Skip talking to PubSub for real.
		e.configureTopic = func(c context.Context, topic, sub, pushURL, publisher string) error {
			return nil
		}

		ctl, err := e.controllerForInvocation(c, &inv)
		So(err, ShouldBeNil)

		// Grab the working auth token.
		_, token, err := ctl.PrepareTopic("some@publisher.com")
		So(err, ShouldBeNil)
		So(token, ShouldNotEqual, "")

		Convey("ProcessPubSubPush works", func() {
			msg := struct {
				Message pubsub.PubsubMessage `json:"message"`
			}{
				Message: pubsub.PubsubMessage{
					Attributes: map[string]string{"auth_token": token},
					Data:       "blah",
				},
			}
			blob, err := json.Marshal(&msg)
			So(err, ShouldBeNil)

			handled := false
			mgr.handleNotification = func(msg *pubsub.PubsubMessage) error {
				So(msg.Data, ShouldEqual, "blah")
				handled = true
				return nil
			}
			So(e.ProcessPubSubPush(c, blob), ShouldBeNil)
			So(handled, ShouldBeTrue)
		})

		Convey("ProcessPubSubPush handles bad token", func() {
			msg := struct {
				Message pubsub.PubsubMessage `json:"message"`
			}{
				Message: pubsub.PubsubMessage{
					Attributes: map[string]string{"auth_token": token + "blah"},
					Data:       "blah",
				},
			}
			blob, err := json.Marshal(&msg)
			So(err, ShouldBeNil)
			So(e.ProcessPubSubPush(c, blob), ShouldErrLike, "bad token")
		})

		Convey("ProcessPubSubPush handles missing invocation", func() {
			ds.Delete(ds.KeyForObj(&inv))
			msg := pubsub.PubsubMessage{
				Attributes: map[string]string{"auth_token": token},
			}
			blob, err := json.Marshal(&msg)
			So(err, ShouldBeNil)
			So(errors.IsTransient(e.ProcessPubSubPush(c, blob)), ShouldBeFalse)
		})
	})
}

func TestAborts(t *testing.T) {
	Convey("with mock invocation", t, func() {
		c := newTestContext(epoch)
		e, mgr := newTestEngine()
		ds := datastore.Get(c)

		taskBlob, err := proto.Marshal(&messages.Task{
			Noop: &messages.NoopTask{},
		})
		So(err, ShouldBeNil)

		// A job in "QUEUED" state (about to run an invocation).
		const jobID = "abc/1"
		const invNonce = int64(12345)
		So(ds.Put(&CronJob{
			JobID:     jobID,
			ProjectID: "abc",
			Enabled:   true,
			Task:      taskBlob,
			Schedule:  "manual",
			State: JobState{
				State:           JobStateQueued,
				InvocationNonce: invNonce,
			},
		}), ShouldBeNil)

		// Launch new invocation.
		var invID int64
		mgr.launchTask = func(ctl task.Controller) error {
			invID = ctl.InvocationID()
			ctl.State().Status = task.StatusRunning
			So(ctl.Save(), ShouldBeNil)
			return nil
		}
		So(e.startInvocation(c, jobID, invNonce, "", 0), ShouldBeNil)

		// It is alive and cron job entity tracks it.
		inv, err := e.GetInvocation(c, jobID, invID)
		So(err, ShouldBeNil)
		So(inv.Status, ShouldEqual, task.StatusRunning)
		job, err := e.GetCronJob(c, jobID)
		So(err, ShouldBeNil)
		So(job.State.State, ShouldEqual, JobStateRunning)
		So(job.State.InvocationID, ShouldEqual, invID)

		Convey("AbortInvocation works", func() {
			// Kill it.
			So(e.AbortInvocation(c, jobID, invID, ""), ShouldBeNil)

			// It is dead.
			inv, err = e.GetInvocation(c, jobID, invID)
			So(err, ShouldBeNil)
			So(inv.Status, ShouldEqual, task.StatusAborted)

			// The cron job moved on with its life.
			job, err = e.GetCronJob(c, jobID)
			So(err, ShouldBeNil)
			So(job.State.State, ShouldEqual, JobStateSuspended)
			So(job.State.InvocationID, ShouldEqual, 0)
		})

		Convey("AbortJob works", func() {
			// Kill it.
			So(e.AbortJob(c, jobID, ""), ShouldBeNil)

			// It is dead.
			inv, err = e.GetInvocation(c, jobID, invID)
			So(err, ShouldBeNil)
			So(inv.Status, ShouldEqual, task.StatusAborted)

			// The cron job moved on with its life.
			job, err = e.GetCronJob(c, jobID)
			So(err, ShouldBeNil)
			So(job.State.State, ShouldEqual, JobStateSuspended)
			So(job.State.InvocationID, ShouldEqual, 0)
		})
	})
}

////

func newTestContext(now time.Time) context.Context {
	c := memory.Use(context.Background())
	c = clock.Set(c, testclock.New(now))
	c = mathrand.Set(c, rand.New(rand.NewSource(1000)))
	c = testsecrets.Use(c)

	ds := datastore.Get(c)
	ds.Testable().AddIndexes(&datastore.IndexDefinition{
		Kind: "CronJob",
		SortBy: []datastore.IndexColumn{
			{Property: "Enabled"},
			{Property: "ProjectID"},
		},
	})
	ds.Testable().CatchupIndexes()

	tq := taskqueue.Get(c)
	tq.Testable().CreateQueue("timers-q")
	tq.Testable().CreateQueue("invs-q")
	return c
}

func newTestEngine() (*engineImpl, *fakeTaskManager) {
	mgr := &fakeTaskManager{}
	cat := catalog.New("cron.cfg")
	cat.RegisterTaskManager(mgr)
	return NewEngine(Config{
		Catalog:              cat,
		TimersQueuePath:      "/timers",
		TimersQueueName:      "timers-q",
		InvocationsQueuePath: "/invs",
		InvocationsQueueName: "invs-q",
		PubSubPushPath:       "/push-url",
	}).(*engineImpl), mgr
}

////

// fakeTaskManager implement task.Manager interface.
type fakeTaskManager struct {
	launchTask         func(ctl task.Controller) error
	handleNotification func(msg *pubsub.PubsubMessage) error
}

func (m *fakeTaskManager) Name() string {
	return "fake"
}

func (m *fakeTaskManager) ProtoMessageType() proto.Message {
	return (*messages.NoopTask)(nil)
}

func (m *fakeTaskManager) ValidateProtoMessage(msg proto.Message) error {
	return nil
}

func (m *fakeTaskManager) LaunchTask(c context.Context, ctl task.Controller) error {
	return m.launchTask(ctl)
}

func (m *fakeTaskManager) AbortTask(c context.Context, ctl task.Controller) error {
	return nil
}

func (m *fakeTaskManager) HandleNotification(c context.Context, ctl task.Controller, msg *pubsub.PubsubMessage) error {
	return m.handleNotification(msg)
}

////

func noopTaskBytes() []byte {
	buf, _ := proto.Marshal(&messages.Task{Noop: &messages.NoopTask{}})
	return buf
}

func allJobs(c context.Context) []CronJob {
	ds := datastore.Get(c)
	ds.Testable().CatchupIndexes()
	entities := []CronJob{}
	if err := ds.GetAll(datastore.NewQuery("CronJob"), &entities); err != nil {
		panic(err)
	}
	// Strip UTC location pointers from zero time.Time{} so that ShouldResemble
	// can compare it to default time.Time{}. nil location is UTC too.
	for i := range entities {
		ent := &entities[i]
		if ent.State.InvocationTime.IsZero() {
			ent.State.InvocationTime = time.Time{}
		}
		if ent.State.TickTime.IsZero() {
			ent.State.TickTime = time.Time{}
		}
	}
	return entities
}

func ensureZeroTasks(c context.Context, q string) {
	tqt := taskqueue.Get(c).Testable()
	tasks := tqt.GetScheduledTasks()[q]
	So(tasks == nil || len(tasks) == 0, ShouldBeTrue)
}

func ensureOneTask(c context.Context, q string) *taskqueue.Task {
	tqt := taskqueue.Get(c).Testable()
	tasks := tqt.GetScheduledTasks()[q]
	So(len(tasks), ShouldEqual, 1)
	for _, t := range tasks {
		return t
	}
	return nil
}
