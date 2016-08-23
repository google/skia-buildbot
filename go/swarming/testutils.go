package swarming

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/util"

	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
)

type testClient struct {
	botList    []*swarming.SwarmingRpcsBotInfo
	botListMtx sync.RWMutex

	taskList    []*swarming.SwarmingRpcsTaskRequestMetadata
	taskListMtx sync.RWMutex
}

func NewTestClient() ApiClient {
	return &testClient{
		botList: []*swarming.SwarmingRpcsBotInfo{},

		taskList: []*swarming.SwarmingRpcsTaskRequestMetadata{},
	}
}

func (c *testClient) SwarmingService() *swarming.Service {
	return nil
}

func (c *testClient) ListBots(dimensions map[string]string) ([]*swarming.SwarmingRpcsBotInfo, error) {
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

func (c *testClient) ListSkiaBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_SKIA,
	})
}

func (c *testClient) ListSkiaTriggerBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_SKIA_TRIGGERS,
	})
}

func (c *testClient) ListCTBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_CT,
	})
}

func (c *testClient) ListTasks(start, end time.Time, tags []string, state string) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
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

func (c *testClient) ListSkiaTasks(start, end time.Time) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.ListTasks(start, end, []string{"pool:Skia"}, "")
}

func (c *testClient) CancelTask(id string) error {
	return nil
}

func (c *testClient) TriggerTask(t *swarming.SwarmingRpcsNewTaskRequest) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return nil, nil
}

func (c *testClient) RetryTask(t *swarming.SwarmingRpcsTaskRequestMetadata) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return nil, nil
}

func (c *testClient) GetTask(id string) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	for _, t := range c.taskList {
		if t.TaskId == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("No such task: %s", id)
}

func (c *testClient) MockBots(bots []*swarming.SwarmingRpcsBotInfo) {
	c.botListMtx.Lock()
	defer c.botListMtx.Unlock()
	c.botList = bots
}

func (c *testClient) MockTasks(tasks []*swarming.SwarmingRpcsTaskRequestMetadata) {
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	c.taskList = tasks
}
