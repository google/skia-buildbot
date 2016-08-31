// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ui

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/golang/protobuf/proto"

	"github.com/luci/luci-go/cron/appengine/engine"
	"github.com/luci/luci-go/cron/appengine/messages"
	"github.com/luci/luci-go/cron/appengine/schedule"
	"github.com/luci/luci-go/cron/appengine/task"
)

type cronJob struct {
	ProjectID   string
	JobID       string
	Schedule    string
	Definition  string
	Revision    string
	RevisionURL string
	State       string
	Overruns    int
	NextRun     string
	Paused      bool
	LabelClass  string

	sortKey string
}

var stateToLabelClass = map[engine.StateKind]string{
	engine.JobStateDisabled:  "label-default",
	engine.JobStateScheduled: "label-primary",
	engine.JobStateSuspended: "label-default",
	engine.JobStateQueued:    "label-primary",
	engine.JobStateRunning:   "label-info",
	engine.JobStateOverrun:   "label-warning",
	engine.JobStateSlowQueue: "label-warning",
}

func makeCronJob(j *engine.CronJob, now time.Time) *cronJob {
	nextRun := ""
	switch ts := j.State.TickTime; {
	case ts == schedule.DistantFuture:
		nextRun = "-"
	case !ts.IsZero():
		nextRun = humanize.RelTime(ts, now, "ago", "from now")
	default:
		nextRun = "not scheduled yet"
	}

	// JobID has form <project>/<id>. Split it into components.
	chunks := strings.Split(j.JobID, "/")

	return &cronJob{
		ProjectID:   chunks[0],
		JobID:       chunks[1],
		Schedule:    j.Schedule,
		Definition:  taskToText(j.Task),
		Revision:    j.Revision,
		RevisionURL: j.RevisionURL,
		State:       string(j.State.State),
		Overruns:    j.State.Overruns,
		NextRun:     nextRun,
		Paused:      j.Paused,
		LabelClass:  stateToLabelClass[j.State.State],

		sortKey: j.JobID,
	}
}

func taskToText(task []byte) string {
	if len(task) == 0 {
		return ""
	}
	msg := messages.Task{}
	if err := proto.Unmarshal(task, &msg); err != nil {
		return fmt.Sprintf("Failed to unmarshal the task - %s", err)
	}
	buf := bytes.Buffer{}
	if err := proto.MarshalText(&buf, &msg); err != nil {
		return fmt.Sprintf("Failed to marshal the task - %s", err)
	}
	return buf.String()
}

type sortedCronJobs []*cronJob

func (s sortedCronJobs) Len() int           { return len(s) }
func (s sortedCronJobs) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedCronJobs) Less(i, j int) bool { return s[i].sortKey < s[j].sortKey }

func convertToSortedCronJobs(jobs []*engine.CronJob, now time.Time) sortedCronJobs {
	out := make(sortedCronJobs, len(jobs))
	for i, job := range jobs {
		out[i] = makeCronJob(job, now)
	}
	sort.Sort(out)
	return out
}

type invocation struct {
	ProjectID   string
	JobID       string
	InvID       int64
	Attempt     int64
	Revision    string
	RevisionURL string
	Definition  string
	TriggeredBy string
	Started     string
	Duration    string
	Status      string
	DebugLog    string
	RowClass    string
	LabelClass  string
	ViewURL     string
}

var statusToRowClass = map[task.Status]string{
	task.StatusStarting:  "active",
	task.StatusRunning:   "info",
	task.StatusSucceeded: "success",
	task.StatusFailed:    "danger",
	task.StatusOverrun:   "warning",
	task.StatusAborted:   "danger",
}

var statusToLabelClass = map[task.Status]string{
	task.StatusStarting:  "label-default",
	task.StatusRunning:   "label-info",
	task.StatusSucceeded: "label-success",
	task.StatusFailed:    "label-danger",
	task.StatusOverrun:   "label-warning",
	task.StatusAborted:   "label-danger",
}

func makeInvocation(projecID, jobID string, i *engine.Invocation, now time.Time) *invocation {
	triggeredBy := "-"
	if i.TriggeredBy != "" {
		triggeredBy = string(i.TriggeredBy)
		if i.TriggeredBy.Email() != "" {
			triggeredBy = i.TriggeredBy.Email() // triggered by a user (not a service)
		}
	}
	finished := i.Finished
	if finished.IsZero() {
		finished = now
	}
	duration := humanize.RelTime(i.Started, finished, "", "")
	if duration == "now" {
		duration = "1 second" // "now" looks weird for durations
	}
	return &invocation{
		ProjectID:   projecID,
		JobID:       jobID,
		InvID:       i.ID,
		Attempt:     i.RetryCount + 1,
		Revision:    i.Revision,
		RevisionURL: i.RevisionURL,
		Definition:  taskToText(i.Task),
		TriggeredBy: triggeredBy,
		Started:     humanize.RelTime(i.Started, now, "ago", "from now"),
		Duration:    duration,
		Status:      string(i.Status),
		DebugLog:    i.DebugLog,
		RowClass:    statusToRowClass[i.Status],
		LabelClass:  statusToLabelClass[i.Status],
		ViewURL:     i.ViewURL,
	}
}

func convertToInvocations(projectID, jobID string, invs []*engine.Invocation, now time.Time) []*invocation {
	out := make([]*invocation, len(invs))
	for i, inv := range invs {
		out[i] = makeInvocation(projectID, jobID, inv, now)
	}
	return out
}
