// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package distributor contains all the adaptors for the various supported
// distributor protocols. At a high level, it works like this:
//   * Quests specify a distributor configuration by name as part of their
//     identity.
//   * When an Execution for that Quest NeedsExecution, DM reads configuration
//     (distributor.proto) from luci-config. This configuration is stored
//     as part of the Execution so that for the duration of a given Exectuion,
//     DM always interacts with the same distributor in the same way (barring
//     code changes in DM's adapter logic itself).
//   * DM uses the selected distributor implementation to start a task and
//     record its Token. Additionally, the distributor MUST subscribe to publish
//     on DM's pubsub topic for updates. When publishing updates, the
//     distributor MUST include 2 attributes (execution_id, pubsub_key), which
//     are provided as part of TaskDescription.
//   * When DM gets a hit on pubsub, it will load the Execution, load its cached
//     distributor configuration, and then call HandleNotification for the
//     adapter to parse the notification body and return the state of the task.
//
// Adding a new distributor requires:
//   * Add a new subdir of protos with the configuration proto for the new
//     distributor. Each distributor implementation must have its own unique
//     Config message.
//   * Add a matching subdir of this package for the implementation of the
//     distributor.
//   * In the implementation, add a Register method that registers the
//     implementation with this package appropriately.
//   * In the DM frontend, import your new package implementation and run its
//     Register method.
package distributor

import (
	"net/http"
	"time"

	dm "github.com/luci/luci-go/dm/api/service/v1"

	"golang.org/x/net/context"
)

// Token is an opaque token that a distributor should use to
// uniquely identify a single DM execution.
type Token string

// Notification represents a notification from the distributor to DM that
// a particular execution has a status update. Data and Attrs are interpreted
// purely by the distributor implementation.
type Notification struct {
	ID    *dm.Execution_ID
	Data  []byte
	Attrs map[string]string
}

// D is the interface for all distributor implementations.
//
// Retries
//
// Unless otherwise noted, DM will retry methods here if they return an error
// marked as Transient, up to some internal limit. If they return
// a non-Transient error (or nil) DM will make a best effort not to duplicate
// calls, but it can't guarantee that.
type D interface {
	// Run prepares and runs a new Task from the given TaskDescription.
	//
	// Scheduling the same TaskDescription multiple times SHOULD return the same
	// Token. It's OK if this doesn't happen, but only one of the scheduled tasks
	// will be able to invoke ActivateExecution; the other one(s) will
	// early-abort and/or timeout.
	//
	// If this returns a non-Transient error, the Execution will be marked as
	// Rejected with the returned error message as the 'Reason'.
	//
	// The various time durations, if non-zero, will be used verbatim for DM to
	// timeout that phase of the task's execution. If the task's execution times
	// out in the 'STOPPING' phase, DM will poll the distributor's GetStatus
	// method up to 3 times with a 30-second gap to attempt to retrieve the final
	// information. After more than 3 times, DM will give up and mark the task as
	// expired.
	//
	// If the distributor doesn't intend to use Pubsub for notifying DM about the
	// final status of the job, set pollTimeout to the amount of time you want DM
	// to wait before polling GetStatus. e.g. if after calling FinishAttempt or
	// EnsureGraphData your distributor needs 10 seconds before it can correctly
	// respond to a GetStatus request, you should set pollTimeout to >= 10s.
	// Otherwise pollTimeout should be set fairly high (e.g. 12 hours) as a hedge
	// against a broken pubsub notification pipeline.
	//
	// If you have the choice between pubsub or not, prefer to use pubsub as it
	// allows DM to more proactively update the graph state (and unblock waiting
	// Attempts, etc.)
	Run(*TaskDescription) (tok Token, pollTimeout time.Duration, err error)

	// Cancel attempts to cancel a running task. If a task is canceled more than
	// once, this should return nil.
	Cancel(Token) error

	// GetStatus retrieves the current state of the task from the distributor.
	//
	// If this returns a non-Transient error more than 30 seconds after the task
	// was Run(), the execution will be marked Missing with the returned error
	// message as the 'Reason'. If it returns a non-Transient error within 30
	// seconds of being run, DM will automatically treat that as Transient.
	GetStatus(Token) (*dm.Result, error)

	// InfoURL calculates a user-presentable information url for the task
	// identified by Token. This should be a local operation, so it is not the
	// implementation's responsibility to validate the token in this method (e.g.
	// it could point to a non-existent job, etc.)
	InfoURL(Token) string

	// HandleNotification is called whenever DM receives a PubSub message sent to
	// a topic created with TaskDescription.PrepareTopic. The Attrs map will omit
	// the 'auth_token' field.
	//
	// Returning (nil, nil) will indicate that DM should ignore this notification.
	//
	// DM will convert pubsub Messages to a delayed GetStatus if a pubsub message
	// is delivered which refers to an Attempt whose status is NeedsExecution,
	// which could happen in the event of a not-fully-settled transacion.
	//
	// DM will ignore any notifications for executions which it doesn't know
	// about.
	HandleNotification(notification *Notification) (*dm.Result, error)

	// HandleTaskQueueTask is called if the distributor used Config.EnqueueTask.
	//
	// It may return zero or more Notifications for DM about arbitrary Executions.
	// These notifications will be handled 'later' by the HandleNotification
	// implementation.
	HandleTaskQueueTask(r *http.Request) ([]*Notification, error)

	// Validate should return a non-nil error if the given distributor parameters
	// are not appropriate for this Distributor. Payload is guaranteed to be
	// a valid JSON object. This should validate that the content of that JSON
	// object is what the distributor expects.
	Validate(parameters string) error
}

// Factory is a function which produces new distributor instance with the
// provided configuration proto.
//
// c is guaranteed to be non-transactional.
type Factory func(c context.Context, dist *Config) (D, error)
