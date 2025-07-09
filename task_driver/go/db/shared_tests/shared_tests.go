package shared_tests

/*
	Shared test utilities for DB implementations.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/td"
)

// Test basic DB functionality.
func TestDB(t sktest.TestingT, d db.DB) {
	// DB should return nil with no error for missing task drivers.
	id := "fake-id-TestDB"
	ctx := context.Background()
	r, err := d.GetTaskDriver(ctx, id)
	require.NoError(t, err)
	require.Nil(t, r)

	// Create a task driver in the DB via UpdateTaskDriver.
	m := &td.Message{
		ID:        uuid.New().String(),
		TaskId:    id,
		StepId:    td.StepIDRoot,
		Timestamp: time.Now().Truncate(time.Millisecond), // BigTable truncates timestamps to milliseconds.
		Type:      td.MsgType_StepStarted,
		Step: &td.StepProperties{
			Id: td.StepIDRoot,
		},
	}
	require.NoError(t, m.Validate())
	require.NoError(t, d.UpdateTaskDriver(ctx, id, m))
	r, err = d.GetTaskDriver(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, r)
	expect := &db.TaskDriverRun{
		TaskId: id,
		Steps: map[string]*db.Step{
			td.StepIDRoot: {
				Properties: &td.StepProperties{
					Id: td.StepIDRoot,
				},
				Started: m.Timestamp,
			},
		},
	}
	assertdeep.Equal(t, r, expect)

	// Update the task driver with some data.
	m = &td.Message{
		ID:        uuid.New().String(),
		TaskId:    id,
		StepId:    td.StepIDRoot,
		Timestamp: time.Now().Truncate(time.Millisecond), // BigTable truncates timestamps to milliseconds.
		Type:      td.MsgType_StepData,
		Data: td.LogData{
			Name:     "fake-log",
			Id:       "fake-log-id",
			Severity: "ERROR",
			Log:      "???",
		},
		DataType: td.DataType_Log,
	}
	require.NoError(t, m.Validate())
	require.NoError(t, d.UpdateTaskDriver(ctx, id, m))
	r, err = d.GetTaskDriver(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, r)
	expect.Steps[td.StepIDRoot].Data = append(expect.Steps[td.StepIDRoot].Data, &db.StepData{
		Type:      m.DataType,
		Data:      m.Data,
		Timestamp: m.Timestamp,
	})
	assertdeep.Equal(t, r, expect)
}

// Verify that messages can arrive in any order with the same result.
func TestMessageOrdering(t sktest.TestingT, d db.DB) {
	ctx := context.Background()

	var msgs []*td.Message
	require.NoError(t, json.NewDecoder(bytes.NewReader([]byte(fakeMessageData))).Decode(&msgs))
	id := "fake-id-MessageOrdering"
	for _, msg := range msgs {
		msg.TaskId = id
	}

	// Play back the messages in the order they were sent. The returned
	// instance becomes the baseline for the remaining tests.
	for _, m := range msgs {
		require.NoError(t, d.UpdateTaskDriver(ctx, id, m))
	}
	base, err := d.GetTaskDriver(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, base)

	// Reverse the messages and play them back.
	id2 := id + "2"
	reversed := make([]*td.Message, len(msgs))
	for i, m := range msgs {
		// Fixup the ID.
		m.TaskId = id2
		reversed[len(reversed)-1-i] = m
	}
	for _, m := range reversed {
		require.NoError(t, d.UpdateTaskDriver(ctx, id2, m))
	}
	rev, err := d.GetTaskDriver(ctx, id2)
	require.NoError(t, err)
	base.TaskId = id2 // The task ID will differ; switch it.
	assertdeep.Equal(t, base, rev)

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
		require.NoError(t, d.UpdateTaskDriver(ctx, id3, m))
	}
	shuf, err := d.GetTaskDriver(ctx, id3)
	require.NoError(t, err)
	base.TaskId = id3 // The task ID will differ; switch it.
	assertdeep.Equal(t, base, shuf)

	// Ensure that we don't make a mess if messages arrive multiple times.
	id4 := id + "4"
	for _, m := range append(append(msgs, reversed...), shuffled...) {
		// Fixup the ID.
		m.TaskId = id4
		require.NoError(t, d.UpdateTaskDriver(ctx, id4, m))
	}
	mult, err := d.GetTaskDriver(ctx, id4)
	require.NoError(t, err)
	base.TaskId = id4 // The task ID will differ; switch it.
	assertdeep.Equal(t, base, mult)
}

const fakeMessageData = `[
  {
    "index": 0,
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.65411663Z",
    "type": "RUN_STARTED",
    "run": {
      "local": false,
      "swarmingBot": "skia-gce-211",
      "swarmingServer": "https://chromium-swarm.appspot.com",
      "swarmingTask": "4242b32a7c174311"
    }
  },
  {
    "index": 1,
    "stepId": "root",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.65436748Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "root",
      "name": "Infra-Experimental-Small",
      "isInfra": false
    }
  },
  {
    "index": 2,
    "stepId": "13109a79-a94a-4591-8fc4-49da0ca70ea4",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.654496253Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "13109a79-a94a-4591-8fc4-49da0ca70ea4",
      "name": "Abs .",
      "isInfra": true,
      "parent": "root"
    }
  },
  {
    "index": 3,
    "stepId": "13109a79-a94a-4591-8fc4-49da0ca70ea4",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.654608588Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 4,
    "stepId": "95d97b45-42f3-4b37-8e76-32ba80ca3cc4",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.654718592Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "95d97b45-42f3-4b37-8e76-32ba80ca3cc4",
      "name": "MkdirAll /mnt/pd0/s/w/ir/go_deps/src",
      "isInfra": true,
      "parent": "root"
    }
  },
  {
    "index": 5,
    "stepId": "95d97b45-42f3-4b37-8e76-32ba80ca3cc4",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.654814424Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 6,
    "stepId": "897a165e-ae4f-45c4-aa31-3a8d515dad4b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.654948258Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "897a165e-ae4f-45c4-aa31-3a8d515dad4b",
      "name": "Ensure Git Checkout",
      "isInfra": true,
      "parent": "root"
    }
  },
  {
    "index": 7,
    "stepId": "2b777a32-cf8b-4d21-82dd-cd77172a2395",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.655061021Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "2b777a32-cf8b-4d21-82dd-cd77172a2395",
      "name": "Stat /mnt/pd0/s/w/ir/go_deps/src/go.skia.org/infra",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 8,
    "stepId": "2b777a32-cf8b-4d21-82dd-cd77172a2395",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.655143451Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 9,
    "stepId": "ec2b619d-c951-430c-805c-821e161c7be7",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.655201875Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "ec2b619d-c951-430c-805c-821e161c7be7",
      "name": "Stat /mnt/pd0/s/w/ir/go_deps/src/go.skia.org/infra/.git",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 10,
    "stepId": "ec2b619d-c951-430c-805c-821e161c7be7",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.655269531Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 11,
    "stepId": "4778094f-f411-4d00-bd75-fc2b69e6a91d",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.65535022Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "4778094f-f411-4d00-bd75-fc2b69e6a91d",
      "name": "not-actually-git status",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 12,
    "stepId": "4778094f-f411-4d00-bd75-fc2b69e6a91d",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.655476163Z",
    "type": "STEP_DATA",
    "data": {
      "id": "90b72cec-3d18-4fa1-8810-f66ac060820b",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 13,
    "stepId": "4778094f-f411-4d00-bd75-fc2b69e6a91d",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.655715894Z",
    "type": "STEP_DATA",
    "data": {
      "id": "70092493-1f4f-40ad-ad66-69a84c61449c",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 14,
    "stepId": "4778094f-f411-4d00-bd75-fc2b69e6a91d",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.655809789Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "not-actually-git",
        "status"
      ]
    },
    "dataType": "command"
  },
  {
    "index": 15,
    "stepId": "4778094f-f411-4d00-bd75-fc2b69e6a91d",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.969139887Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 16,
    "stepId": "7894e9f4-b892-4e9f-8232-6265ac2e1b81",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.969413087Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "7894e9f4-b892-4e9f-8232-6265ac2e1b81",
      "name": "not-actually-git remote -v",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 17,
    "stepId": "7894e9f4-b892-4e9f-8232-6265ac2e1b81",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.969548114Z",
    "type": "STEP_DATA",
    "data": {
      "id": "264a95ac-2b41-4ddb-b9ee-2bbb82a82071",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 18,
    "stepId": "7894e9f4-b892-4e9f-8232-6265ac2e1b81",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.969628591Z",
    "type": "STEP_DATA",
    "data": {
      "id": "c9ac8eda-8dea-4fbe-9555-a3ed61f5c3ce",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 19,
    "stepId": "7894e9f4-b892-4e9f-8232-6265ac2e1b81",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.969698636Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "not-actually-git",
        "remote",
        "-v"
      ]
    },
    "dataType": "command"
  },
  {
    "index": 20,
    "stepId": "7894e9f4-b892-4e9f-8232-6265ac2e1b81",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.976733932Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 21,
    "stepId": "6418edca-6a0e-485c-a83f-df6029fd5ef9",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.976942217Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "6418edca-6a0e-485c-a83f-df6029fd5ef9",
      "name": "not-actually-git rev-parse HEAD",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 22,
    "stepId": "6418edca-6a0e-485c-a83f-df6029fd5ef9",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.9770855Z",
    "type": "STEP_DATA",
    "data": {
      "id": "35e1878a-ce41-4566-9acd-feb73e2a2bc9",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 23,
    "stepId": "6418edca-6a0e-485c-a83f-df6029fd5ef9",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.977237561Z",
    "type": "STEP_DATA",
    "data": {
      "id": "42d75467-ad79-4274-89bb-97acc6aca0c7",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 24,
    "stepId": "6418edca-6a0e-485c-a83f-df6029fd5ef9",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.977301821Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "not-actually-git",
        "rev-parse",
        "HEAD"
      ]
    },
    "dataType": "command"
  },
  {
    "index": 25,
    "stepId": "6418edca-6a0e-485c-a83f-df6029fd5ef9",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.982529469Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 26,
    "stepId": "fce61d42-e978-4a96-a0e1-dedfa22cd929",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.982664076Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "fce61d42-e978-4a96-a0e1-dedfa22cd929",
      "name": "Stat /mnt/pd0/s/w/ir/go_deps/src/go.skia.org/infra",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 27,
    "stepId": "fce61d42-e978-4a96-a0e1-dedfa22cd929",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.982812968Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 28,
    "stepId": "acd599c0-cfab-4f5e-be8c-b8061c360ff4",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.983038236Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "acd599c0-cfab-4f5e-be8c-b8061c360ff4",
      "name": "not-actually-git fetch --prune origin",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 29,
    "stepId": "acd599c0-cfab-4f5e-be8c-b8061c360ff4",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.983139146Z",
    "type": "STEP_DATA",
    "data": {
      "id": "80979bbf-2acd-4513-9e14-e944b1402b4b",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 30,
    "stepId": "acd599c0-cfab-4f5e-be8c-b8061c360ff4",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.983233946Z",
    "type": "STEP_DATA",
    "data": {
      "id": "e274939a-2046-437a-9986-e6382949c683",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 31,
    "stepId": "acd599c0-cfab-4f5e-be8c-b8061c360ff4",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:18.983308585Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "not-actually-git",
        "fetch",
        "--prune",
        "origin"
      ]
    },
    "dataType": "command"
  },
  {
    "index": 32,
    "stepId": "acd599c0-cfab-4f5e-be8c-b8061c360ff4",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.190338928Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 33,
    "stepId": "a79b6dcb-e555-4c48-8b82-200c8fb5c117",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.190471939Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "a79b6dcb-e555-4c48-8b82-200c8fb5c117",
      "name": "not-actually-git reset --hard HEAD",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 34,
    "stepId": "a79b6dcb-e555-4c48-8b82-200c8fb5c117",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.190544949Z",
    "type": "STEP_DATA",
    "data": {
      "id": "39a1722e-38d3-43e6-8aaa-c6f2990c4ace",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 35,
    "stepId": "a79b6dcb-e555-4c48-8b82-200c8fb5c117",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.190626057Z",
    "type": "STEP_DATA",
    "data": {
      "id": "fe0bb75f-c29d-4d0f-9224-f139050a0065",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 36,
    "stepId": "a79b6dcb-e555-4c48-8b82-200c8fb5c117",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.190773216Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "not-actually-git",
        "reset",
        "--hard",
        "HEAD"
      ]
    },
    "dataType": "command"
  },
  {
    "index": 37,
    "stepId": "a79b6dcb-e555-4c48-8b82-200c8fb5c117",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.226138678Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 38,
    "stepId": "de44473c-4b9f-489c-89cf-8d9e5717900a",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.226309285Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "de44473c-4b9f-489c-89cf-8d9e5717900a",
      "name": "not-actually-git clean -d -f",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 39,
    "stepId": "de44473c-4b9f-489c-89cf-8d9e5717900a",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.226547439Z",
    "type": "STEP_DATA",
    "data": {
      "id": "40c08549-3cfc-4b59-b319-8332d8efeb48",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 40,
    "stepId": "de44473c-4b9f-489c-89cf-8d9e5717900a",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.226636207Z",
    "type": "STEP_DATA",
    "data": {
      "id": "43f5d6d6-f4fe-4613-9e71-8de2d1ecef9e",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 41,
    "stepId": "de44473c-4b9f-489c-89cf-8d9e5717900a",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.22675083Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "not-actually-git",
        "clean",
        "-d",
        "-f"
      ]
    },
    "dataType": "command"
  },
  {
    "index": 42,
    "stepId": "de44473c-4b9f-489c-89cf-8d9e5717900a",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.247576799Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 43,
    "stepId": "428b12d7-6a9a-48a8-a27d-20fefb1de130",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.247780617Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "428b12d7-6a9a-48a8-a27d-20fefb1de130",
      "name": "not-actually-git checkout master -f",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 44,
    "stepId": "428b12d7-6a9a-48a8-a27d-20fefb1de130",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.247921585Z",
    "type": "STEP_DATA",
    "data": {
      "id": "efa76053-8db5-4618-ae9d-992bcefeac99",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 45,
    "stepId": "428b12d7-6a9a-48a8-a27d-20fefb1de130",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.24811287Z",
    "type": "STEP_DATA",
    "data": {
      "id": "68976005-0010-4b05-aa9b-a286f1fc567a",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 46,
    "stepId": "428b12d7-6a9a-48a8-a27d-20fefb1de130",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.248207762Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "not-actually-git",
        "checkout",
        "master",
        "-f"
      ]
    },
    "dataType": "command"
  },
  {
    "index": 47,
    "stepId": "428b12d7-6a9a-48a8-a27d-20fefb1de130",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.279240064Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 48,
    "stepId": "6f554f46-04d5-4a57-b4e6-0f3a0c28a912",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.279453231Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "6f554f46-04d5-4a57-b4e6-0f3a0c28a912",
      "name": "not-actually-git reset --hard origin/master",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 49,
    "stepId": "6f554f46-04d5-4a57-b4e6-0f3a0c28a912",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.2795854Z",
    "type": "STEP_DATA",
    "data": {
      "id": "d540eaf3-cc38-49b0-887d-d0a2db4e6f82",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 50,
    "stepId": "6f554f46-04d5-4a57-b4e6-0f3a0c28a912",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.279713155Z",
    "type": "STEP_DATA",
    "data": {
      "id": "1e7c47a4-097a-4d96-abb8-902709d8a76e",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 51,
    "stepId": "6f554f46-04d5-4a57-b4e6-0f3a0c28a912",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.279847368Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "not-actually-git",
        "reset",
        "--hard",
        "origin/master"
      ]
    },
    "dataType": "command"
  },
  {
    "index": 52,
    "stepId": "6f554f46-04d5-4a57-b4e6-0f3a0c28a912",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.334199251Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 53,
    "stepId": "9984209c-3132-4b37-95b9-aaefbd4ab077",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.334442972Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "9984209c-3132-4b37-95b9-aaefbd4ab077",
      "name": "not-actually-git reset --hard 1ca75add9814643b74fee7b2272796f440fd1d03",
      "isInfra": true,
      "parent": "897a165e-ae4f-45c4-aa31-3a8d515dad4b"
    }
  },
  {
    "index": 54,
    "stepId": "9984209c-3132-4b37-95b9-aaefbd4ab077",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.334552082Z",
    "type": "STEP_DATA",
    "data": {
      "id": "9b33df31-556f-4c07-a080-49bc561e8f40",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 55,
    "stepId": "9984209c-3132-4b37-95b9-aaefbd4ab077",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.334658681Z",
    "type": "STEP_DATA",
    "data": {
      "id": "7576242f-ab4b-42eb-9707-54bf80ceeb19",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 56,
    "stepId": "9984209c-3132-4b37-95b9-aaefbd4ab077",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.334734197Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "not-actually-git",
        "reset",
        "--hard",
        "1ca75add9814643b74fee7b2272796f440fd1d03"
      ]
    },
    "dataType": "command"
  },
  {
    "index": 57,
    "stepId": "9984209c-3132-4b37-95b9-aaefbd4ab077",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.374688259Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 58,
    "stepId": "897a165e-ae4f-45c4-aa31-3a8d515dad4b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.374868755Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 59,
    "stepId": "def2c8b9-77c3-4067-aec4-83919210c544",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.37499883Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "def2c8b9-77c3-4067-aec4-83919210c544",
      "name": "Set Go Environment",
      "isInfra": false,
      "environment": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ],
      "parent": "root"
    }
  },
  {
    "index": 60,
    "stepId": "a3778271-8e7f-45cf-9ede-70f4350a866b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.375111318Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "a3778271-8e7f-45cf-9ede-70f4350a866b",
      "name": "which go",
      "isInfra": false,
      "environment": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ],
      "parent": "def2c8b9-77c3-4067-aec4-83919210c544"
    }
  },
  {
    "index": 61,
    "stepId": "a3778271-8e7f-45cf-9ede-70f4350a866b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.37534656Z",
    "type": "STEP_DATA",
    "data": {
      "id": "64dbf995-0ee3-40d7-895d-5a72f88275a4",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 62,
    "stepId": "a3778271-8e7f-45cf-9ede-70f4350a866b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.375469936Z",
    "type": "STEP_DATA",
    "data": {
      "id": "7a0e3b88-ad1c-410b-9cd4-5d512a14ea18",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 63,
    "stepId": "a3778271-8e7f-45cf-9ede-70f4350a866b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.375542488Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "which",
        "go"
      ],
      "env": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ]
    },
    "dataType": "command"
  },
  {
    "index": 64,
    "stepId": "a3778271-8e7f-45cf-9ede-70f4350a866b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.376835099Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 65,
    "stepId": "1a152940-9e5a-4fe4-877b-38b42b2c872b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.377481341Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "1a152940-9e5a-4fe4-877b-38b42b2c872b",
      "name": "go version",
      "isInfra": false,
      "environment": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ],
      "parent": "def2c8b9-77c3-4067-aec4-83919210c544"
    }
  },
  {
    "index": 66,
    "stepId": "1a152940-9e5a-4fe4-877b-38b42b2c872b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.377627273Z",
    "type": "STEP_DATA",
    "data": {
      "id": "b20445d7-ed10-4176-9e7d-9133fed4f988",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 67,
    "stepId": "1a152940-9e5a-4fe4-877b-38b42b2c872b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.377792604Z",
    "type": "STEP_DATA",
    "data": {
      "id": "81e08427-45bc-497b-8edf-e3c0e1c28333",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 68,
    "stepId": "1a152940-9e5a-4fe4-877b-38b42b2c872b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.377914237Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "go",
        "version"
      ],
      "env": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ]
    },
    "dataType": "command"
  },
  {
    "index": 69,
    "stepId": "1a152940-9e5a-4fe4-877b-38b42b2c872b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.388812904Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 70,
    "stepId": "2a13263f-af43-4f23-a790-c79ecdd8c145",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.389025762Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "2a13263f-af43-4f23-a790-c79ecdd8c145",
      "name": "sudo npm i -g bower@1.8.2",
      "isInfra": false,
      "environment": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ],
      "parent": "def2c8b9-77c3-4067-aec4-83919210c544"
    }
  },
  {
    "index": 71,
    "stepId": "2a13263f-af43-4f23-a790-c79ecdd8c145",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.389178194Z",
    "type": "STEP_DATA",
    "data": {
      "id": "69832f77-e449-4242-96d9-9bb6d82c9b90",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 72,
    "stepId": "2a13263f-af43-4f23-a790-c79ecdd8c145",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.389309221Z",
    "type": "STEP_DATA",
    "data": {
      "id": "8ab039c3-3288-4d95-89ee-e9dd97b09578",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 73,
    "stepId": "2a13263f-af43-4f23-a790-c79ecdd8c145",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:21.389390041Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "sudo",
        "npm",
        "i",
        "-g",
        "bower@1.8.2"
      ],
      "env": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ]
    },
    "dataType": "command"
  },
  {
    "index": 74,
    "stepId": "2a13263f-af43-4f23-a790-c79ecdd8c145",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.322193956Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 75,
    "stepId": "cb396dfd-f442-4251-9bc2-fa9c8ddd4e0b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.322405933Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "cb396dfd-f442-4251-9bc2-fa9c8ddd4e0b",
      "name": "./setup_test_db",
      "isInfra": false,
      "environment": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ],
      "parent": "def2c8b9-77c3-4067-aec4-83919210c544"
    }
  },
  {
    "index": 76,
    "stepId": "cb396dfd-f442-4251-9bc2-fa9c8ddd4e0b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.322578892Z",
    "type": "STEP_DATA",
    "data": {
      "id": "1614311f-2070-4140-8e13-416c8a5beb53",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 77,
    "stepId": "cb396dfd-f442-4251-9bc2-fa9c8ddd4e0b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.322688616Z",
    "type": "STEP_DATA",
    "data": {
      "id": "4e62ca76-dc03-4ba7-bf7e-85ecf8c1bdd6",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 78,
    "stepId": "cb396dfd-f442-4251-9bc2-fa9c8ddd4e0b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.322761468Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "./setup_test_db"
      ],
      "env": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ]
    },
    "dataType": "command"
  },
  {
    "index": 79,
    "stepId": "cb396dfd-f442-4251-9bc2-fa9c8ddd4e0b",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.673649588Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 80,
    "stepId": "881b2d19-6b63-4211-9a08-b21a5de0adc7",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.673900337Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "881b2d19-6b63-4211-9a08-b21a5de0adc7",
      "name": "Sync missing Go DEPS",
      "isInfra": false,
      "environment": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ],
      "parent": "def2c8b9-77c3-4067-aec4-83919210c544"
    }
  },
  {
    "index": 81,
    "stepId": "fbe2518b-8363-4c7d-b1b8-866046af7f38",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.674052845Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "fbe2518b-8363-4c7d-b1b8-866046af7f38",
      "name": "python /mnt/pd0/s/w/ir/go_deps/src/go.skia.org/infra/scripts/find_missing_go_deps.py --json",
      "isInfra": false,
      "environment": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ],
      "parent": "881b2d19-6b63-4211-9a08-b21a5de0adc7"
    }
  },
  {
    "index": 82,
    "stepId": "fbe2518b-8363-4c7d-b1b8-866046af7f38",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.674235218Z",
    "type": "STEP_DATA",
    "data": {
      "id": "f45e5f3c-557f-4487-b183-d32675325811",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 83,
    "stepId": "fbe2518b-8363-4c7d-b1b8-866046af7f38",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.674376112Z",
    "type": "STEP_DATA",
    "data": {
      "id": "c995dbf6-9d17-47be-a84f-5505ff6c0194",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 84,
    "stepId": "fbe2518b-8363-4c7d-b1b8-866046af7f38",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:34.674464953Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "python",
        "/mnt/pd0/s/w/ir/go_deps/src/go.skia.org/infra/scripts/find_missing_go_deps.py",
        "--json"
      ],
      "env": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ]
    },
    "dataType": "command"
  },
  {
    "index": 85,
    "stepId": "fbe2518b-8363-4c7d-b1b8-866046af7f38",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:38.695186118Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 86,
    "stepId": "881b2d19-6b63-4211-9a08-b21a5de0adc7",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:38.69535963Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 87,
    "stepId": "36e603c5-0c66-4c7c-bfc3-c401f22352cf",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:38.695554162Z",
    "type": "STEP_STARTED",
    "step": {
      "id": "36e603c5-0c66-4c7c-bfc3-c401f22352cf",
      "name": "go run ./run_unittests.go --alsologtostderr --small",
      "isInfra": false,
      "environment": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ],
      "parent": "def2c8b9-77c3-4067-aec4-83919210c544"
    }
  },
  {
    "index": 88,
    "stepId": "36e603c5-0c66-4c7c-bfc3-c401f22352cf",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:38.695724672Z",
    "type": "STEP_DATA",
    "data": {
      "id": "16cb8d22-7458-4f28-ad01-dd13bb913e65",
      "name": "stdout",
      "severity": "INFO"
    },
    "dataType": "log"
  },
  {
    "index": 89,
    "stepId": "36e603c5-0c66-4c7c-bfc3-c401f22352cf",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:38.695853625Z",
    "type": "STEP_DATA",
    "data": {
      "id": "eb6833c8-2a50-4cb3-bb78-6b2a2c209b6c",
      "name": "stderr",
      "severity": "ERROR"
    },
    "dataType": "log"
  },
  {
    "index": 90,
    "stepId": "36e603c5-0c66-4c7c-bfc3-c401f22352cf",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T19:58:38.695921619Z",
    "type": "STEP_DATA",
    "data": {
      "command": [
        "go",
        "run",
        "./run_unittests.go",
        "--alsologtostderr",
        "--small"
      ],
      "env": [
        "CHROME_HEADLESS=1",
        "GOCACHE=/mnt/pd0/s/w/ir/cache/go_cache",
        "GOROOT=/mnt/pd0/s/w/ir/go/go",
        "GOPATH=/mnt/pd0/s/w/ir/go_deps",
        "GIT_USER_AGENT=git/1.9.1",
        "PATH=/mnt/pd0/s/w/ir/go/go/bin:/mnt/pd0/s/w/ir/go_deps/bin:/mnt/pd0/s/w/ir/gcloud_linux/bin:/mnt/pd0/s/w/ir/protoc/bin:/mnt/pd0/s/w/ir/node/node/bin:/b/s/w/ir/cipd_bin_packages:/b/s/w/ir/cipd_bin_packages/bin:/b/s/w/ir/go/go/bin:/b/s/cipd_cache/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/mnt/pd0/s/w/ir/depot_tools",
        "SKIABOT_TEST_DEPOT_TOOLS=/mnt/pd0/s/w/ir/depot_tools",
        "TMPDIR="
      ]
    },
    "dataType": "command"
  },
  {
    "index": 91,
    "stepId": "36e603c5-0c66-4c7c-bfc3-c401f22352cf",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T20:00:53.105756143Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 92,
    "stepId": "def2c8b9-77c3-4067-aec4-83919210c544",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T20:00:53.106064996Z",
    "type": "STEP_FINISHED"
  },
  {
    "index": 93,
    "stepId": "root",
    "taskId": "20190107T195439.551193060Z_0000000000ae5bf3",
    "timestamp": "2019-01-07T20:00:53.106179289Z",
    "type": "STEP_FINISHED"
  }
]`
