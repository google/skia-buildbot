package swarming

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/util"

	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/satori/go.uuid"
)

type TestClient struct {
	botList    []*swarming.SwarmingRpcsBotInfo
	botListMtx sync.RWMutex

	taskList    []*swarming.SwarmingRpcsTaskRequestMetadata
	taskListMtx sync.RWMutex
}

func NewTestClient() *TestClient {
	return &TestClient{
		botList:  []*swarming.SwarmingRpcsBotInfo{},
		taskList: []*swarming.SwarmingRpcsTaskRequestMetadata{},
	}
}

func (c *TestClient) SwarmingService() *swarming.Service {
	return nil
}

func (c *TestClient) ListBots(dimensions map[string]string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	c.botListMtx.RLock()
	defer c.botListMtx.RUnlock()
	rv := make([]*swarming.SwarmingRpcsBotInfo, 0, len(c.botList))
	for _, b := range c.botList {
		match := true
		for k, v := range dimensions {
			dMatch := false
			for _, dim := range b.Dimensions {
				if dim.Key == k && util.In(v, dim.Value) {
					dMatch = true
					break
				}
			}
			if !dMatch {
				match = false
				break
			}
		}
		if match {
			rv = append(rv, b)
		}
	}
	return rv, nil
}

func (c *TestClient) ListSkiaBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_SKIA,
	})
}

func (c *TestClient) ListSkiaTriggerBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_SKIA_TRIGGERS,
	})
}

func (c *TestClient) ListCTBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_CT,
	})
}

func (c *TestClient) GetStdoutOfTask(id string) (*swarming.SwarmingRpcsTaskOutput, error) {
	return nil, nil
}

func (c *TestClient) GracefullyShutdownBot(id string) (*swarming.SwarmingRpcsTerminateResponse, error) {
	return nil, nil
}

func (c *TestClient) ListTasks(start, end time.Time, tags []string, state string) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	rv := make([]*swarming.SwarmingRpcsTaskRequestMetadata, 0, len(c.taskList))
	tagSet := util.NewStringSet(tags)
	for _, t := range c.taskList {
		if len(tagSet.Intersect(util.NewStringSet(t.Request.Tags))) == len(tags) {
			if state == "" || t.TaskResult.State == state {
				rv = append(rv, t)
			}
		}
	}
	return rv, nil
}

func (c *TestClient) ListSkiaTasks(start, end time.Time) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.ListTasks(start, end, []string{"pool:Skia"}, "")
}

func (c *TestClient) CancelTask(id string) error {
	return nil
}

// TriggerTask automatically appends its result to the mocked tasks set by
// MockTasks.
func (c *TestClient) TriggerTask(t *swarming.SwarmingRpcsNewTaskRequest) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	createdTs := time.Now().UTC().Format(TIMESTAMP_FORMAT)
	id := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	rv := &swarming.SwarmingRpcsTaskRequestMetadata{
		Request: &swarming.SwarmingRpcsTaskRequest{
			CreatedTs:  createdTs,
			Name:       t.Name,
			Priority:   t.Priority,
			Properties: t.Properties,
			Tags:       t.Tags,
		},
		TaskId: id,
		TaskResult: &swarming.SwarmingRpcsTaskResult{
			CreatedTs: createdTs,
			Name:      t.Name,
			State:     "PENDING",
			TaskId:    id,
			Tags:      t.Tags,
		},
	}
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	c.taskList = append(c.taskList, rv)
	return rv, nil
}

func (c *TestClient) RetryTask(t *swarming.SwarmingRpcsTaskRequestMetadata) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.TriggerTask(&swarming.SwarmingRpcsNewTaskRequest{
		Name:     t.Request.Name,
		Priority: t.Request.Priority,
		Tags:     t.Request.Tags,
		User:     t.Request.User,
	})
}

func (c *TestClient) GetTask(id string) (*swarming.SwarmingRpcsTaskResult, error) {
	m, err := c.GetTaskMetadata(id)
	if err != nil {
		return nil, err
	}
	return m.TaskResult, nil
}

func (c *TestClient) GetTaskMetadata(id string) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	for _, t := range c.taskList {
		if t.TaskId == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("No such task: %s", id)
}

func (c *TestClient) MockBots(bots []*swarming.SwarmingRpcsBotInfo) {
	c.botListMtx.Lock()
	defer c.botListMtx.Unlock()
	c.botList = bots
}

// MockTasks sets the tasks that can be returned from ListTasks, ListSkiaTasks,
// GetTaskMetadata, and GetTask. Replaces any previous tasks, including those
// automatically added by TriggerTask.
func (c *TestClient) MockTasks(tasks []*swarming.SwarmingRpcsTaskRequestMetadata) {
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	c.taskList = tasks
}

// DoMockTasks calls f for each mocked task, allowing goroutine-safe updates. f
// must not call any other method on c.
func (c *TestClient) DoMockTasks(f func(*swarming.SwarmingRpcsTaskRequestMetadata)) {
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	for _, task := range c.taskList {
		f(task)
	}
}
