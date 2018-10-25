package db

/*
	Shared test utilities for DB implementations.
*/

import (
	"math/rand"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_driver/go/td"
)

// Test basic DB functionality.
func TestDB(t testutils.TestingT, d DB) {
	// DB should return nil with no error for missing task drivers.
	id := "fake-id-TestDB"
	r, err := d.GetTaskDriver(id)
	assert.NoError(t, err)
	assert.Nil(t, r)

	// Create a task driver in the DB via UpdateTaskDriver.
	m := &td.Message{
		TaskId:    id,
		StepId:    td.STEP_ID_ROOT,
		Timestamp: time.Now().Truncate(time.Millisecond), // BigTable truncates timestamps to milliseconds.
		Type:      td.MSG_TYPE_STEP_STARTED,
		Step: &td.StepProperties{
			Id: td.STEP_ID_ROOT,
		},
	}
	assert.NoError(t, m.Validate())
	assert.NoError(t, d.UpdateTaskDriver(id, m))
	r, err = d.GetTaskDriver(id)
	assert.NoError(t, err)
	assert.NotNil(t, r)
	expect := &TaskDriverRun{
		TaskId: id,
		Steps: map[string]*Step{
			td.STEP_ID_ROOT: &Step{
				Properties: &td.StepProperties{
					Id: td.STEP_ID_ROOT,
				},
				Started: m.Timestamp,
			},
		},
	}
	deepequal.AssertDeepEqual(t, r, expect)

	// Update the task driver with some data.
	m = &td.Message{
		TaskId:    id,
		StepId:    td.STEP_ID_ROOT,
		Timestamp: time.Now().Truncate(time.Millisecond), // BigTable truncates timestamps to milliseconds.
		Type:      td.MSG_TYPE_STEP_DATA,
		Data: td.LogData{
			Name:     "fake-log",
			Id:       "fake-log-id",
			Severity: "ERROR",
			Log:      "???",
		},
		DataType: td.DATA_TYPE_LOG,
	}
	assert.NoError(t, m.Validate())
	assert.NoError(t, d.UpdateTaskDriver(id, m))
	r, err = d.GetTaskDriver(id)
	assert.NoError(t, err)
	assert.NotNil(t, r)
	expect.Steps[td.STEP_ID_ROOT].Data = append(expect.Steps[td.STEP_ID_ROOT].Data, &StepData{
		Type: m.DataType,
		Data: m.Data,
	})
	deepequal.AssertDeepEqual(t, r, expect)
}

// Verify that messages can arrive in any order with the same result.
func TestMessageOrdering(t testutils.TestingT, d DB) {
	id := "fake-id-MessageOrdering"

	// TODO(borenet): Obtain a non-trivial set of real-world messages.
	msgs := []*td.Message{
		&td.Message{
			TaskId:    id,
			StepId:    td.STEP_ID_ROOT,
			Timestamp: time.Now().Truncate(time.Millisecond), // BigTable truncates timestamps to milliseconds.
			Type:      td.MSG_TYPE_STEP_STARTED,
			Step: &td.StepProperties{
				Id: td.STEP_ID_ROOT,
			},
		},
	}

	// Play back the messages in the order they were sent. The returned
	// instance becomes the baseline for the remaining tests.
	for _, m := range msgs {
		assert.NoError(t, d.UpdateTaskDriver(id, m))
	}
	base, err := d.GetTaskDriver(id)
	assert.NoError(t, err)
	assert.NotNil(t, base)

	// Reverse the messages and play them back.
	id2 := id + "2"
	reversed := make([]*td.Message, len(msgs))
	for i, m := range msgs {
		// Fixup the ID.
		m.TaskId = id2
		reversed[len(reversed)-1-i] = m
	}
	for _, m := range reversed {
		assert.NoError(t, d.UpdateTaskDriver(id2, m))
	}
	rev, err := d.GetTaskDriver(id2)
	assert.NoError(t, err)
	base.TaskId = id2 // The task ID will differ; switch it.
	deepequal.AssertDeepEqual(t, base, rev)

	// Shuffle the messages and play them back.
	id3 := id + "3"
	shuffled := make([]*td.Message, len(msgs))
	for i, shuffleIdx := range rand.Perm(len(msgs)) {
		m := msgs[shuffleIdx]
		// Fixup the ID.
		m.TaskId = id3
		shuffled[i] = m
	}
	for _, m := range shuffled {
		assert.NoError(t, d.UpdateTaskDriver(id3, m))
	}
	shuf, err := d.GetTaskDriver(id3)
	assert.NoError(t, err)
	base.TaskId = id3 // The task ID will differ; switch it.
	deepequal.AssertDeepEqual(t, base, shuf)

	// Ensure that we don't make a mess if messages arrive multiple times.
	id4 := id + "4"
	for _, m := range append(append(msgs, reversed...), shuffled...) {
		// Fixup the ID.
		m.TaskId = id4
		assert.NoError(t, d.UpdateTaskDriver(id4, m))
	}
	mult, err := d.GetTaskDriver(id4)
	assert.NoError(t, err)
	base.TaskId = id4 // The task ID will differ; switch it.
	deepequal.AssertDeepEqual(t, base, mult)
}
