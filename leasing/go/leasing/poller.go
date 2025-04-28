/*
	Used by the Leasing Server to poll swarming.
*/

package main

import (
	"context"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/leasing/go/types"
	"google.golang.org/api/iterator"
)

// populateRunningTask updates the provided Task struct with a new state, botId, lease start/end times,
// and sends a start email.
func populateRunningTask(newState, botId string, k *datastore.Key, t *types.Task) error {
	// Update the state and add the bot name.
	t.SwarmingTaskState = newState
	t.SwarmingBotId = botId

	// Add the start and end lease times.
	durationHrs, err := strconv.Atoi(t.InitialDurationHrs)
	if err != nil {
		return skerr.Wrapf(err, "Failed to parse %s", t.InitialDurationHrs)
	}
	t.LeaseStartTime = time.Now()
	t.LeaseEndTime = time.Now().Add(time.Hour * time.Duration(durationHrs))

	// Inform the requester that the task has been picked up
	threadingReference, err := SendStartEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId, t.SwarmingBotId, t.TaskIdForIsolates)
	if err != nil {
		sklog.Errorf("Error sending start email: %s", err)
	} else {
		// Store threadingReference in datastore for threading followup emails.
		t.EmailThreadingReference = threadingReference
	}

	if _, err := UpdateDSTask(k, t); err != nil {
		return skerr.Wrapf(err, "Error updating task in datastore")
	}

	return nil
}

// expireTask marks the provided Task struct as Done and sends a completion email.
func expireTask(k *datastore.Key, t *types.Task) error {
	t.Done = true
	t.SwarmingTaskState = getCompletedStateStr(false)
	if _, err := UpdateDSTask(k, t); err != nil {
		return skerr.Wrapf(err, "Error updating task in datastore")
	}
	sklog.Infof("Marked as expired task %v in the datastore with key %d", t, k.ID)
	// Inform the requester that the task has completed.
	if err := SendCompletionEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId, t.SwarmingBotId, t.EmailThreadingReference); err != nil {
		return skerr.Wrapf(err, "Error sending completion email")
	}
	return nil
}

// taskExpiringSoon sends a warning email and updates the WarningSent field in the Task struct.
func taskExpiringSoon(k *datastore.Key, t *types.Task) error {
	if err := SendWarningEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId, t.SwarmingBotId, t.EmailThreadingReference); err != nil {
		return skerr.Wrapf(err, "Error sending 15m warning email")
	}
	t.WarningSent = true
	if _, err := UpdateDSTask(k, t); err != nil {
		return skerr.Wrapf(err, "Error updating task in datastore")
	}
	return nil
}

// taskCancelled marks the provided Task struct as cancelled.
func taskCancelled(k *datastore.Key, t *types.Task) error {
	t.Done = true
	t.SwarmingTaskState = getCompletedStateStr(true)
	t.LeaseStartTime = time.Now()
	t.LeaseEndTime = time.Now()
	if _, err := UpdateDSTask(k, t); err != nil {
		return skerr.Wrapf(err, "Error updating task in datastore")
	}
	return nil
}

func getCompletedStateStr(failure bool) string {
	if failure {
		return apipb.TaskState_COMPLETED.String() + " (FAILURE)"
	}
	return apipb.TaskState_COMPLETED.String() + " (SUCCESS)"
}

// checkForUnexpectedStates checks to see if the new state falls in a list of unexpected states.
// If it does then the Task is marked as Done, the lease ended and a failure email it sent.
func checkForUnexpectedStates(newState apipb.TaskState, failure bool, k *datastore.Key, t *types.Task) error {
	unexpectedStates := []apipb.TaskState{
		apipb.TaskState_BOT_DIED,
		apipb.TaskState_CANCELED,
		apipb.TaskState_CLIENT_ERROR,
		apipb.TaskState_COMPLETED,
		apipb.TaskState_EXPIRED,
		apipb.TaskState_NO_RESOURCE,
		apipb.TaskState_TIMED_OUT,
	}
	for _, unexpectedState := range unexpectedStates {
		if newState == unexpectedState {
			// Update the state.
			t.SwarmingTaskState = newState.String()
			if newState == apipb.TaskState_COMPLETED {
				t.SwarmingTaskState = getCompletedStateStr(failure)
			}
			// Something unexpected happened so mark the leasing task as done and end the lease.
			t.Done = true
			t.LeaseEndTime = time.Now()

			if _, err := UpdateDSTask(k, t); err != nil {
				return skerr.Wrapf(err, "Error updating task in datastore")
			}

			// Inform the requester that something went wrong.
			if err := SendFailureEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId, t.SwarmingBotId, t.SwarmingTaskState, t.EmailThreadingReference); err != nil {
				return skerr.Wrapf(err, "Error sending failure email")
			}
			break
		}
	}
	return nil
}

// pollSwarmingTasks gets all running tasks from the Datastore, polls the equivalent
// tasks in swarming, and updates the tasks in the Datastore accordingly.
func pollSwarmingTasks(ctx context.Context) error {

	it := GetRunningDSTasks()
	for {
		t := &types.Task{}
		k, err := it.Next(t)
		if err == iterator.Done {
			break
		} else if err != nil {
			return skerr.Wrapf(err, "Failed to retrieve list of tasks")
		}

		if t.SwarmingTaskId == "" {
			// This task is not ready to be looked at yet.
			continue
		}

		// Get the swarming task from swarming server.
		swarmingTask, err := GetSwarmingTask(ctx, t.SwarmingPool, t.SwarmingTaskId)
		if err != nil {
			return skerr.Wrapf(err, "Failed to retrieve swarming task %s: %s", t.SwarmingTaskId, err)
		}

		if swarmingTask.State == apipb.TaskState_PENDING {
			// If the swarming task is still pending then there is nothing to do here.
			continue
		} else if swarmingTask.State == apipb.TaskState_CANCELED {
			// If the swarming task has been cancelled then mark it as such in the DS.
			if err := taskCancelled(k, t); err != nil {
				return skerr.Wrapf(err, "Failed to mark task as cancelled")
			}
			continue
		}

		// Check for 4 things-
		//
		// Check 1: If the previous state was pending and the new state is not then
		//          update the state, add botId and populate the lease start/end time.
		// Check 2: If lease has already expired then set Done=true.
		// Check 3: If lease is expiring in 15 mins then send a warning email if we
		//          have not done so already.
		// Check 4: If new state is unexpected then set Done=true and send failure email
		//          to the requester.

		if t.SwarmingTaskState == apipb.TaskState_PENDING.String() /* Check 1 */ {
			// The previous task state was pending but this has now changed.
			// Populate the datastore with running task values.
			if err := populateRunningTask(swarmingTask.State.String(), swarmingTask.BotId, k, t); err != nil {
				return skerr.Wrapf(err, "Error populating running task")
			}
		} else if t.LeaseEndTime.Before(time.Now()) /* Check 2*/ {
			// The task has expired.
			if err := expireTask(k, t); err != nil {
				return skerr.Wrapf(err, "Error when expiring task")
			}
		} else if swarmingTask.State == apipb.TaskState_RUNNING && !t.WarningSent && t.LeaseEndTime.Before(time.Now().Add(time.Minute*15)) /* Check 3 */ {
			if err := taskExpiringSoon(k, t); err != nil {
				return skerr.Wrapf(err, "Error when warning task expiring soon")
			}
		} else /* Check 4 */ {

			if err := checkForUnexpectedStates(swarmingTask.State, swarmingTask.Failure, k, t); err != nil {
				return skerr.Wrapf(err, "Error when warning task expiring soon")
			}
		}
	}

	return nil
}
