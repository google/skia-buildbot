/*
	Used by the Leasing Server to poll swarming.
*/

package main

import (
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

func populateRunningTask(newState, botId string, t *Task, k *datastore.Key) error {
	// Update the state and add the bot name.
	t.SwarmingTaskState = newState
	t.SwarmingBotId = botId

	// Add the start and end lease times.
	durationHrs, err := strconv.Atoi(t.InitialDurationHrs)
	if err != nil {
		return fmt.Errorf("Failed to parse %s", t.InitialDurationHrs)
	}
	t.LeaseStartTime = time.Now()
	t.LeaseEndTime = time.Now().Add(time.Minute * time.Duration(durationHrs)) // rmistry: Change to mins only for testing

	if _, err := UpdateDSTask(k, t); err != nil {
		return fmt.Errorf("Error updating task in datastore: %v", err)
	}

	// Inform the requester that the task has been picked up
	if err := SendStartEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId, t.SwarmingBotId); err != nil {
		return fmt.Errorf("Error sending start email: %s", err)
	}

	return nil
}

func expireTask(t *Task, k *datastore.Key) error {
	t.Done = true
	if _, err := UpdateDSTask(k, t); err != nil {
		return fmt.Errorf("Error updating task in datastore: %v", err)
	}
	sklog.Infof("Marked as expired task %v in the datastore with key %d", t, k.ID)
	// Inform the requester that the task has completed.
	if err := SendCompletionEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId); err != nil {
		return fmt.Errorf("Error sending completion email: %s", err)
	}
	return nil
}

func taskExpiringSoon(t *Task, k *datastore.Key) error {
	if err := SendWarningEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId); err != nil {
		return fmt.Errorf("Error sending 15m warning email: %s", err)
	}
	t.WarningSent = true
	if _, err := UpdateDSTask(k, t); err != nil {
		return fmt.Errorf("Error updating task in datastore: %v", err)
	}
	return nil
}

func checkForUnexpectedStates(newState string, failure bool, t *Task, k *datastore.Key) error {
	// TODO(rmistry): Uncomment the below!
	unexpectedStates := []string{
		swarming.TASK_STATE_BOT_DIED,
		swarming.TASK_STATE_CANCELED,
		//swarming.TASK_STATE_COMPLETED,
		swarming.TASK_STATE_EXPIRED,
		swarming.TASK_STATE_TIMED_OUT,
	}
	for _, unexpectedState := range unexpectedStates {
		if newState == unexpectedState {
			// Update the state.
			if newState == swarming.TASK_STATE_COMPLETED {
				// If completed state then append success or failure as well.
				if failure {
					newState += " (FAILURE)"
				} else {
					newState += " (SUCCESS)"
				}
			}
			t.SwarmingTaskState = newState
			// Something unexpected happened so mark the leasing task as done and end the lease.
			t.Done = true
			t.LeaseEndTime = time.Now()

			if _, err := UpdateDSTask(k, t); err != nil {
				return fmt.Errorf("Error updating task in datastore: %v", err)
			}

			// Inform the requester that something went wrong.
			if err := SendFailureEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId, t.SwarmingTaskState); err != nil {
				return fmt.Errorf("Error sending failure email: %s", err)
			}
			break
		}
	}
	return nil
}

func pollSwarmingTasks() error {

	it := GetRunningDSTasks()
	for {
		t := &Task{}
		k, err := it.Next(t)
		if err == iterator.Done {
			fmt.Println("Iterator is done")
			break
		} else if err != nil {
			return fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}

		// Get the swarming task from swarming server.
		swarmingTask, err := GetSwarmingTask(t.SwarmingTaskId)
		if err != nil {
			return fmt.Errorf("Failed to retrieve swarming task %s: %s", t.SwarmingTaskId, err)
		}

		if swarmingTask.State == swarming.TASK_STATE_PENDING {
			// If the swarming task is still pending then there is nothing to do here.
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

		if t.SwarmingTaskState == swarming.TASK_STATE_PENDING /* Check 1 */ {
			// The previous task state was pending but this has now changed.
			// Populate the datastore with running task values.
			if err := populateRunningTask(swarmingTask.State, swarmingTask.BotId, t, k); err != nil {
				return fmt.Errorf("Error populating running task: %s", err)
			}
		} else if t.LeaseEndTime.Before(time.Now()) /* Check 2*/ {
			// The task has expired.
			if err := expireTask(t, k); err != nil {
				return fmt.Errorf("Error when expiring task: %s", err)
			}
		} else if !t.WarningSent && t.LeaseEndTime.Before(time.Now().Add(time.Minute*15)) /* Check 3 */ {
			if err := taskExpiringSoon(t, k); err != nil {
				return fmt.Errorf("Error when warning task expiring soon: %s", err)
			}
			// TODO(rmistry): Add if swarmingTask.State == swarming.TASK_STATE_RUNNING IMPORTANT!!!!!!!
		} else /* Check 4 */ {

			if err := checkForUnexpectedStates(swarmingTask.State, swarmingTask.Failure, t, k); err != nil {
				return fmt.Errorf("Error when warning task expiring soon: %s", err)
			}
		}
	}

	return nil
}
