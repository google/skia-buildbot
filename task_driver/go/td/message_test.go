package td

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
)

func TestMessageValidation(t *testing.T) {

	now := time.Now()

	// Helper funcs.
	checkValid := func(m *Message) {
		require.NoError(t, m.Validate())
	}
	checkNotValid := func(fn func() *Message, errMsg string) {
		err := skerr.Unwrap(fn().Validate())
		require.EqualError(t, err, errMsg)
	}
	msgRunStarted := func() *Message {
		return &Message{
			ID:        uuid.New().String(),
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MsgType_RunStarted,
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
			ID:        uuid.New().String(),
			StepId:    StepIDRoot,
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MsgType_StepStarted,
			Step: &StepProperties{
				Id:      StepIDRoot,
				Name:    "step-name",
				IsInfra: false,
			},
		}
	}
	msgStepFinished := func() *Message {
		return &Message{
			ID:        uuid.New().String(),
			StepId:    "fake-step-id",
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MsgType_StepFinished,
		}
	}
	msgTextStepData := func() *Message {
		return &Message{
			ID:        uuid.New().String(),
			StepId:    "fake-step-id",
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MsgType_StepData,
			Data: &TextData{
				Value: "http://www.google.com",
				Label: "Google homepage",
			},
			DataType: DataType_Text,
		}
	}
	msgCommandStepData := func() *Message {
		return &Message{
			ID:        uuid.New().String(),
			StepId:    "fake-step-id",
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MsgType_StepData,
			Data: &ExecData{
				Cmd: []string{"echo", "hi"},
				Env: []string{"K=V"},
			},
			DataType: DataType_Command,
		}
	}
	msgStepFailed := func() *Message {
		return &Message{
			ID:        uuid.New().String(),
			StepId:    "fake-step-id",
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MsgType_StepFailed,
			Error:     "failed",
		}
	}
	msgStepException := func() *Message {
		return &Message{
			ID:        uuid.New().String(),
			StepId:    "fake-step-id",
			TaskId:    "fake-task-id",
			Timestamp: now,
			Type:      MsgType_StepException,
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
		m.ID = ""
		return m
	}, "ID or Index is required.")
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
	}, fmt.Sprintf("RunProperties are required for %s", MsgType_RunStarted))
	checkNotValid(func() *Message {
		m := msgRunStarted()
		m.Run.SwarmingBot = ""
		return m
	}, "SwarmingBot is required for non-local runs!")
	checkNotValid(func() *Message {
		m := msgStepStarted()
		m.StepId = ""
		return m
	}, fmt.Sprintf("StepId is required for %s", MsgType_StepStarted))
	checkNotValid(func() *Message {
		m := msgStepStarted()
		m.Step = nil
		return m
	}, fmt.Sprintf("StepProperties are required for %s", MsgType_StepStarted))
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
	}, fmt.Sprintf("StepId is required for %s", MsgType_StepFinished))
	checkNotValid(func() *Message {
		m := msgCommandStepData()
		m.StepId = ""
		return m
	}, fmt.Sprintf("StepId is required for %s", MsgType_StepData))
	checkNotValid(func() *Message {
		m := msgCommandStepData()
		m.Data = nil
		return m
	}, fmt.Sprintf("Data is required for %s", MsgType_StepData))
	checkNotValid(func() *Message {
		m := msgCommandStepData()
		m.DataType = "fake"
		return m
	}, "Invalid DataType \"fake\"")
	checkNotValid(func() *Message {
		m := msgStepFailed()
		m.StepId = ""
		return m
	}, fmt.Sprintf("StepId is required for %s", MsgType_StepFailed))
	checkNotValid(func() *Message {
		m := msgStepFailed()
		m.Error = ""
		return m
	}, fmt.Sprintf("Error is required for %s", MsgType_StepFailed))
	checkNotValid(func() *Message {
		m := msgStepException()
		m.StepId = ""
		return m
	}, fmt.Sprintf("StepId is required for %s", MsgType_StepException))
	checkNotValid(func() *Message {
		m := msgStepException()
		m.Error = ""
		return m
	}, fmt.Sprintf("Error is required for %s", MsgType_StepException))
	checkNotValid(func() *Message {
		m := msgStepException()
		m.Type = "invalid"
		return m
	}, "Invalid message Type \"invalid\"")
}
