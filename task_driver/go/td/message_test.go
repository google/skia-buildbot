package td

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMessageValidation(t *testing.T) {

	now := time.Now()

	// Helper funcs.
	checkValid := func(m *Message) {
		require.NoError(t, m.Validate())
	}
	checkNotValid := func(fn func() *Message, errMsg string) {
		require.EqualError(t, fn().Validate(), errMsg)
	}
	msgIndex := int32(0)
	msgRunStarted := func() *Message {
		return &Message{
			Index:     int(atomic.AddInt32(&msgIndex, 1)),
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MSG_TYPE_RUN_STARTED,
			Run: &RunProperties{
				Local:          false,
				SwarmingBot:    "bot",
				SwarmingServer: "server",
				SwarmingTask:   "task",
			},
		}
	}
	msgStepStarted := func() *Message {
		return &Message{
			Index:     int(atomic.AddInt32(&msgIndex, 1)),
			StepId:    StepIDRoot,
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MSG_TYPE_STEP_STARTED,
			Step: &StepProperties{
				Id:      StepIDRoot,
				Name:    "step-name",
				IsInfra: false,
			},
		}
	}
	msgStepFinished := func() *Message {
		return &Message{
			Index:     int(atomic.AddInt32(&msgIndex, 1)),
			StepId:    "fake-step-id",
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MSG_TYPE_STEP_FINISHED,
		}
	}
	msgTextStepData := func() *Message {
		return &Message{
			Index:     int(atomic.AddInt32(&msgIndex, 1)),
			StepId:    "fake-step-id",
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MSG_TYPE_STEP_DATA,
			Data: &TextData{
				Value: "http://www.google.com",
				Label: "Google homepage",
			},
			DataType: DATA_TYPE_TEXT,
		}
	}
	msgCommandStepData := func() *Message {
		return &Message{
			Index:     int(atomic.AddInt32(&msgIndex, 1)),
			StepId:    "fake-step-id",
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MSG_TYPE_STEP_DATA,
			Data: &ExecData{
				Cmd: []string{"echo", "hi"},
				Env: []string{"K=V"},
			},
			DataType: DATA_TYPE_COMMAND,
		}
	}
	msgStepFailed := func() *Message {
		return &Message{
			Index:     int(atomic.AddInt32(&msgIndex, 1)),
			StepId:    "fake-step-id",
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MSG_TYPE_STEP_FAILED,
			Error:     "failed",
		}
	}
	msgStepException := func() *Message {
		return &Message{
			Index:     int(atomic.AddInt32(&msgIndex, 1)),
			StepId:    "fake-step-id",
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MSG_TYPE_STEP_EXCEPTION,
			Error:     "exception",
		}
	}

	// Validate a few messages.
	checkValid(msgRunStarted())
	checkValid(msgStepStarted())
	nonRootStarted := msgStepStarted()
	nonRootStarted.StepId = "fake-step-id"
	nonRootStarted.Step.Id = "fake-step-id"
	nonRootStarted.Step.Parent = StepIDRoot
	checkValid(nonRootStarted)
	checkValid(msgStepFinished())
	checkValid(msgTextStepData())
	checkValid(msgCommandStepData())
	checkValid(msgStepFailed())
	checkValid(msgStepException())

	// Check that we catch missing fields.
	checkNotValid(func() *Message {
		m := msgRunStarted()
		m.TaskId = ""
		return m
	}, "TaskId is required.")
	checkNotValid(func() *Message {
		m := msgRunStarted()
		m.Timestamp = time.Time{}
		return m
	}, "Timestamp is required.")
	checkNotValid(func() *Message {
		m := msgRunStarted()
		m.Run = nil
		return m
	}, fmt.Sprintf("RunProperties are required for %s", MSG_TYPE_RUN_STARTED))
	checkNotValid(func() *Message {
		m := msgRunStarted()
		m.Run.SwarmingBot = ""
		return m
	}, "SwarmingBot is required for non-local runs!")
	checkNotValid(func() *Message {
		m := msgStepStarted()
		m.StepId = ""
		return m
	}, fmt.Sprintf("StepId is required for %s", MSG_TYPE_STEP_STARTED))
	checkNotValid(func() *Message {
		m := msgStepStarted()
		m.Step = nil
		return m
	}, fmt.Sprintf("StepProperties are required for %s", MSG_TYPE_STEP_STARTED))
	checkNotValid(func() *Message {
		m := msgStepStarted()
		m.Step.Id = ""
		return m
	}, "Id is required.")
	checkNotValid(func() *Message {
		m := msgStepStarted()
		m.Step.Id = "non-root-step-with-no-parent"
		return m
	}, "Non-root steps must have a parent.")
	checkNotValid(func() *Message {
		m := msgStepStarted()
		m.Step.Id = "mismatch"
		m.Step.Parent = StepIDRoot
		return m
	}, "StepId must equal Step.Id (root vs mismatch)")
	checkNotValid(func() *Message {
		m := msgStepFinished()
		m.StepId = ""
		return m
	}, fmt.Sprintf("StepId is required for %s", MSG_TYPE_STEP_FINISHED))
	checkNotValid(func() *Message {
		m := msgCommandStepData()
		m.StepId = ""
		return m
	}, fmt.Sprintf("StepId is required for %s", MSG_TYPE_STEP_DATA))
	checkNotValid(func() *Message {
		m := msgCommandStepData()
		m.Data = nil
		return m
	}, fmt.Sprintf("Data is required for %s", MSG_TYPE_STEP_DATA))
	checkNotValid(func() *Message {
		m := msgCommandStepData()
		m.DataType = "fake"
		return m
	}, "Invalid DataType \"fake\"")
	checkNotValid(func() *Message {
		m := msgStepFailed()
		m.StepId = ""
		return m
	}, fmt.Sprintf("StepId is required for %s", MSG_TYPE_STEP_FAILED))
	checkNotValid(func() *Message {
		m := msgStepFailed()
		m.Error = ""
		return m
	}, fmt.Sprintf("Error is required for %s", MSG_TYPE_STEP_FAILED))
	checkNotValid(func() *Message {
		m := msgStepException()
		m.StepId = ""
		return m
	}, fmt.Sprintf("StepId is required for %s", MSG_TYPE_STEP_EXCEPTION))
	checkNotValid(func() *Message {
		m := msgStepException()
		m.Error = ""
		return m
	}, fmt.Sprintf("Error is required for %s", MSG_TYPE_STEP_EXCEPTION))
	checkNotValid(func() *Message {
		m := msgStepException()
		m.Type = "invalid"
		return m
	}, "Invalid message Type \"invalid\"")
	checkNotValid(func() *Message {
		m := msgStepStarted()
		m.Index = 0
		return m
	}, "A non-zero index is required.")
}
