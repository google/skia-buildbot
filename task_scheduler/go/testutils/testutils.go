package testutils

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

type TestClient struct {
	botList    []*swarming_api.SwarmingRpcsBotInfo
	botListMtx sync.RWMutex

	taskList    []*swarming_api.SwarmingRpcsTaskRequestMetadata
	taskListMtx sync.RWMutex

	triggerDedupe     map[string][]string
	triggerFailure    map[string]bool
	triggerNoResource map[string]bool
	triggerMtx        sync.Mutex
}

func NewTestClient() *TestClient {
	return &TestClient{
		botList:           []*swarming_api.SwarmingRpcsBotInfo{},
		taskList:          []*swarming_api.SwarmingRpcsTaskRequestMetadata{},
		triggerDedupe:     map[string][]string{},
		triggerFailure:    map[string]bool{},
		triggerNoResource: map[string]bool{},
	}
}

func (c *TestClient) SwarmingService() *swarming_api.Service {
	return nil
}

func (c *TestClient) GetStates(ctx context.Context, ids []string) ([]string, error) {
	rv := make([]string, 0, len(ids))
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	for _, id := range ids {
		found := false
		for _, t := range c.taskList {
			if t.TaskId == id {
				rv = append(rv, t.TaskResult.State)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("Unknown task %q", id)
		}
	}
	return rv, nil
}

func (c *TestClient) ListBots(ctx context.Context, dimensions map[string]string) ([]*swarming_api.SwarmingRpcsBotInfo, error) {
	c.botListMtx.RLock()
	defer c.botListMtx.RUnlock()
	rv := make([]*swarming_api.SwarmingRpcsBotInfo, 0, len(c.botList))
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

func (c *TestClient) ListDownBots(ctx context.Context, pool string) ([]*swarming_api.SwarmingRpcsBotInfo, error) {
	return nil, nil
}

func (c *TestClient) ListFreeBots(ctx context.Context, pool string) ([]*swarming_api.SwarmingRpcsBotInfo, error) {
	bots, err := c.ListBots(ctx, map[string]string{
		swarming.DIMENSION_POOL_KEY: pool,
	})
	if err != nil {
		return nil, err
	}
	rv := make([]*swarming_api.SwarmingRpcsBotInfo, 0, len(bots))
	for _, b := range bots {
		if !b.Quarantined && !b.IsDead && b.TaskId == "" {
			rv = append(rv, b)
		}
	}
	return rv, nil
}

func (c *TestClient) ListBotsForPool(ctx context.Context, pool string) ([]*swarming_api.SwarmingRpcsBotInfo, error) {
	return c.ListBots(ctx, map[string]string{
		swarming.DIMENSION_POOL_KEY: pool,
	})
}

func (c *TestClient) GetStdoutOfTask(ctx context.Context, id string) (*swarming_api.SwarmingRpcsTaskOutput, error) {
	return nil, nil
}

func (c *TestClient) GracefullyShutdownBot(ctx context.Context, id string) (*swarming_api.SwarmingRpcsTerminateResponse, error) {
	return nil, nil
}

func (c *TestClient) ListTasks(ctx context.Context, start, end time.Time, tags []string, state string) ([]*swarming_api.SwarmingRpcsTaskRequestMetadata, error) {
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	rv := make([]*swarming_api.SwarmingRpcsTaskRequestMetadata, 0, len(c.taskList))
	tagSet := util.NewStringSet(tags)
	for _, t := range c.taskList {
		created, err := time.Parse(swarming.TIMESTAMP_FORMAT, t.TaskResult.CreatedTs)
		if err != nil {
			return nil, err
		}
		if !util.TimeIsZero(start) && start.After(created) {
			continue
		}
		if !util.TimeIsZero(end) && end.Before(created) {
			continue
		}
		if len(tagSet.Intersect(util.NewStringSet(t.Request.Tags))) == len(tags) {
			if state == "" || t.TaskResult.State == state {
				rv = append(rv, t)
			}
		}
	}
	return rv, nil
}

func (c *TestClient) ListBotTasks(ctx context.Context, botID string, limit int) ([]*swarming_api.SwarmingRpcsTaskResult, error) {
	// For now, just return all tasks in the list.  This could probably be better.
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	rv := make([]*swarming_api.SwarmingRpcsTaskResult, 0, len(c.taskList))
	for _, t := range c.taskList {
		rv = append(rv, t.TaskResult)
	}
	return rv, nil
}

func (c *TestClient) ListSkiaTasks(ctx context.Context, start, end time.Time) ([]*swarming_api.SwarmingRpcsTaskRequestMetadata, error) {
	return c.ListTasks(ctx, start, end, []string{"pool:Skia"}, "")
}

func (c *TestClient) ListTaskResults(ctx context.Context, start, end time.Time, tags []string, state string, includePerformanceStats bool) ([]*swarming_api.SwarmingRpcsTaskResult, error) {
	tasks, err := c.ListTasks(ctx, start, end, tags, state)
	if err != nil {
		return nil, err
	}
	rv := make([]*swarming_api.SwarmingRpcsTaskResult, len(tasks), len(tasks))
	for i, t := range tasks {
		rv[i] = t.TaskResult
	}
	return rv, nil
}

func (c *TestClient) CancelTask(ctx context.Context, id string, killRunning bool) error {
	return nil
}

// md5Tags returns a MD5 hash of the task tags, excluding task ID.
func md5Tags(tags []string) string {
	filtered := make([]string, 0, len(tags))
	for _, t := range tags {
		if !strings.HasPrefix(t, "sk_id") {
			filtered = append(filtered, t)
		}
	}
	sort.Strings(filtered)
	rv, err := util.MD5SSlice(filtered)
	if err != nil {
		sklog.Fatal(err)
	}
	return rv
}

// TriggerTask automatically appends its result to the mocked tasks set by
// MockTasks.
func (c *TestClient) TriggerTask(ctx context.Context, t *swarming_api.SwarmingRpcsNewTaskRequest) (*swarming_api.SwarmingRpcsTaskRequestMetadata, error) {
	c.triggerMtx.Lock()
	defer c.triggerMtx.Unlock()
	md5 := md5Tags(t.Tags)
	if c.triggerFailure[md5] {
		delete(c.triggerFailure, md5)
		return nil, fmt.Errorf("Mocked trigger failure!")
	}

	createdTs := time.Now().UTC().Format(swarming.TIMESTAMP_FORMAT)
	id := uuid.New().String()
	rv := &swarming_api.SwarmingRpcsTaskRequestMetadata{
		Request: &swarming_api.SwarmingRpcsTaskRequest{
			CreatedTs:      createdTs,
			ExpirationSecs: t.ExpirationSecs,
			Name:           t.Name,
			Priority:       t.Priority,
			Properties:     t.Properties,
			Tags:           t.Tags,
			TaskSlices:     t.TaskSlices,
		},
		TaskId: id,
		TaskResult: &swarming_api.SwarmingRpcsTaskResult{
			CreatedTs: createdTs,
			Name:      t.Name,
			State:     swarming.TASK_STATE_PENDING,
			TaskId:    id,
			Tags:      t.Tags,
		},
	}
	if c.triggerNoResource[md5] {
		delete(c.triggerNoResource, md5)
		rv.TaskResult.State = swarming.TASK_STATE_NO_RESOURCE
	} else if extraTags, ok := c.triggerDedupe[md5]; ok {
		delete(c.triggerDedupe, md5)
		rv.TaskResult.State = swarming.TASK_STATE_COMPLETED // No deduplicated state.
		rv.TaskResult.DedupedFrom = uuid.New().String()
		rv.TaskResult.Tags = append(rv.TaskResult.Tags, extraTags...)
	}
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	c.taskList = append(c.taskList, rv)
	return rv, nil
}

func (c *TestClient) RetryTask(ctx context.Context, t *swarming_api.SwarmingRpcsTaskRequestMetadata) (*swarming_api.SwarmingRpcsTaskRequestMetadata, error) {
	return c.TriggerTask(ctx, &swarming_api.SwarmingRpcsNewTaskRequest{
		Name:     t.Request.Name,
		Priority: t.Request.Priority,
		Tags:     t.Request.Tags,
		User:     t.Request.User,
	})
}

func (c *TestClient) GetTask(ctx context.Context, id string, includePerformanceStats bool) (*swarming_api.SwarmingRpcsTaskResult, error) {
	m, err := c.GetTaskMetadata(ctx, id)
	if err != nil {
		return nil, err
	}
	return m.TaskResult, nil
}

func (c *TestClient) GetTaskMetadata(ctx context.Context, id string) (*swarming_api.SwarmingRpcsTaskRequestMetadata, error) {
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	for _, t := range c.taskList {
		if t.TaskId == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("No such task: %s", id)
}

func (c *TestClient) DeleteBots(ctx context.Context, bots []string) error {
	return nil
}

func (c *TestClient) MockBots(bots []*swarming_api.SwarmingRpcsBotInfo) {
	c.botListMtx.Lock()
	defer c.botListMtx.Unlock()
	c.botList = bots
}

// MockTasks sets the tasks that can be returned from ListTasks, ListSkiaTasks,
// GetTaskMetadata, and GetTask. Replaces any previous tasks, including those
// automatically added by TriggerTask.
func (c *TestClient) MockTasks(tasks []*swarming_api.SwarmingRpcsTaskRequestMetadata) {
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	c.taskList = tasks
}

// DoMockTasks calls f for each mocked task, allowing goroutine-safe updates. f
// must not call any other method on c.
func (c *TestClient) DoMockTasks(f func(*swarming_api.SwarmingRpcsTaskRequestMetadata)) {
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	for _, task := range c.taskList {
		f(task)
	}
}

// MockTriggerTaskFailure forces the next call to TriggerTask which matches
// the given tags to fail.
func (c *TestClient) MockTriggerTaskFailure(tags []string) {
	c.triggerMtx.Lock()
	defer c.triggerMtx.Unlock()
	c.triggerFailure[md5Tags(tags)] = true
}

// MockTriggerTaskDeduped forces the next call to TriggerTask which matches
// the given tags to result in a deduplicated task. The optional extraTags are
// added to the TaskResult in the TaskRequestMetadata returned by TriggerTask
// and are intended to help test unexpected behavior in deduplicated tasks.
func (c *TestClient) MockTriggerTaskDeduped(tags []string, extraTags ...string) {
	c.triggerMtx.Lock()
	defer c.triggerMtx.Unlock()
	c.triggerDedupe[md5Tags(tags)] = extraTags
}

// MockTriggerTaskNoResource forces the next call to TriggerTask which matches
// the given tags to have NO_RESOURCE state.
func (c *TestClient) MockTriggerTaskNoResource(tags []string) {
	c.triggerMtx.Lock()
	defer c.triggerMtx.Unlock()
	c.triggerNoResource[md5Tags(tags)] = true
}

var _ swarming.ApiClient = &TestClient{}
