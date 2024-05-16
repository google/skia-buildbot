package testutils

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/go/util"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TestClient struct {
	botList    []*apipb.BotInfo
	botListMtx sync.RWMutex

	taskList    []*apipb.TaskRequestMetadataResponse
	taskListMtx sync.RWMutex

	triggerDedupe     map[string][]string
	triggerFailure    map[string]bool
	triggerNoResource map[string]bool
	triggerMtx        sync.Mutex
}

func NewTestClient() *TestClient {
	return &TestClient{
		botList:           []*apipb.BotInfo{},
		taskList:          []*apipb.TaskRequestMetadataResponse{},
		triggerDedupe:     map[string][]string{},
		triggerFailure:    map[string]bool{},
		triggerNoResource: map[string]bool{},
	}
}

func (c *TestClient) ListTaskStates(_ context.Context, in *apipb.TaskStatesRequest, opts ...grpc.CallOption) (*apipb.TaskStates, error) {
	rv := make([]apipb.TaskState, 0, len(in.TaskId))
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	for _, id := range in.TaskId {
		found := false
		for _, t := range c.taskList {
			if t.TaskId == id {
				rv = append(rv, t.TaskResult.State)
				found = true
				break
			}
		}
		if !found {
			return nil, skerr.Fmt("unknown task %q", id)
		}
	}
	return &apipb.TaskStates{
		States: rv,
	}, nil
}

func (c *TestClient) ListBots(_ context.Context, in *apipb.BotsRequest, opts ...grpc.CallOption) (*apipb.BotInfoListResponse, error) {
	c.botListMtx.RLock()
	defer c.botListMtx.RUnlock()
	rv := make([]*apipb.BotInfo, 0, len(c.botList))
	for _, b := range c.botList {
		match := true
		for _, reqDim := range in.Dimensions {
			dMatch := false
			for _, botDim := range b.Dimensions {
				if reqDim.Key == botDim.Key && util.In(reqDim.Value, botDim.Value) {
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
	return &apipb.BotInfoListResponse{
		Items: rv,
	}, nil
}

func (c *TestClient) ListTasks(_ context.Context, req *apipb.TasksWithPerfRequest, _ ...grpc.CallOption) (*apipb.TaskListResponse, error) {
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	rv := make([]*apipb.TaskResultResponse, 0, len(c.taskList))
	tagSet := util.NewStringSet(req.Tags)
	for _, t := range c.taskList {
		created := t.TaskResult.CreatedTs.AsTime()
		if req.Start != nil && req.Start.AsTime().After(created) {
			continue
		}
		if req.End != nil && req.End.AsTime().Before(created) {
			continue
		}
		if len(tagSet.Intersect(util.NewStringSet(t.Request.Tags))) == len(req.Tags) {
			if req.State == apipb.StateQuery_QUERY_ALL || t.TaskResult.State.String() == req.State.String() {
				rv = append(rv, t.TaskResult)
			}
		}
	}
	return &apipb.TaskListResponse{
		Items: rv,
	}, nil
}

func (c *TestClient) ListBotTasks(_ context.Context, in *apipb.BotTasksRequest, opts ...grpc.CallOption) (*apipb.TaskListResponse, error) {
	// For now, just return all tasks in the list.  This could probably be better.
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	rv := make([]*apipb.TaskResultResponse, 0, len(c.taskList))
	for _, t := range c.taskList {
		rv = append(rv, t.TaskResult)
	}
	return &apipb.TaskListResponse{
		Items: rv,
	}, nil
}

func (c *TestClient) CancelTask(_ context.Context, _ *apipb.TaskCancelRequest, _ ...grpc.CallOption) (*apipb.CancelResponse, error) {
	return nil, nil
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

// NewTask automatically appends its result to the mocked tasks set by
// MockTasks.
func (c *TestClient) NewTask(ctx context.Context, t *apipb.NewTaskRequest, _ ...grpc.CallOption) (*apipb.TaskRequestMetadataResponse, error) {
	c.triggerMtx.Lock()
	defer c.triggerMtx.Unlock()
	md5 := md5Tags(t.Tags)
	if c.triggerFailure[md5] {
		delete(c.triggerFailure, md5)
		return nil, skerr.Fmt("mocked trigger failure!")
	}

	createdTs := timestamppb.New(now.Now(ctx).UTC())
	id := uuid.New().String()
	rv := &apipb.TaskRequestMetadataResponse{
		Request: &apipb.TaskRequestResponse{
			CreatedTs:      createdTs,
			ExpirationSecs: t.ExpirationSecs,
			Name:           t.Name,
			Priority:       t.Priority,
			Properties:     t.Properties,
			Tags:           t.Tags,
			TaskSlices:     t.TaskSlices,
		},
		TaskId: id,
		TaskResult: &apipb.TaskResultResponse{
			CreatedTs: createdTs,
			Name:      t.Name,
			State:     apipb.TaskState_PENDING,
			TaskId:    id,
			Tags:      t.Tags,
		},
	}
	if c.triggerNoResource[md5] {
		delete(c.triggerNoResource, md5)
		rv.TaskResult.State = apipb.TaskState_NO_RESOURCE
	} else if extraTags, ok := c.triggerDedupe[md5]; ok {
		delete(c.triggerDedupe, md5)
		rv.TaskResult.State = apipb.TaskState_COMPLETED // No deduplicated state.
		rv.TaskResult.DedupedFrom = uuid.New().String()
		rv.TaskResult.Tags = append(rv.TaskResult.Tags, extraTags...)
	}
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	c.taskList = append(c.taskList, rv)
	return rv, nil
}

func (c *TestClient) RetryTask(ctx context.Context, t *apipb.TaskRequestMetadataResponse) (*apipb.TaskRequestMetadataResponse, error) {
	return c.NewTask(ctx, &apipb.NewTaskRequest{
		Name:     t.Request.Name,
		Priority: t.Request.Priority,
		Tags:     t.Request.Tags,
		User:     t.Request.User,
	})
}

func (c *TestClient) GetResult(_ context.Context, in *apipb.TaskIdWithPerfRequest, opts ...grpc.CallOption) (*apipb.TaskResultResponse, error) {
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	for _, t := range c.taskList {
		if t.TaskId == in.TaskId {
			return t.TaskResult, nil
		}
	}
	return nil, skerr.Fmt("no such task: %s", in.TaskId)
}

func (c *TestClient) GetRequest(_ context.Context, in *apipb.TaskIdRequest, opts ...grpc.CallOption) (*apipb.TaskRequestResponse, error) {
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	for _, t := range c.taskList {
		if t.TaskId == in.TaskId {
			return t.Request, nil
		}
	}
	return nil, skerr.Fmt("no such task: %s", in.TaskId)
}

func (c *TestClient) DeleteBots(_ context.Context, bots []string) error {
	return nil
}

func (c *TestClient) MockBots(bots []*apipb.BotInfo) {
	c.botListMtx.Lock()
	defer c.botListMtx.Unlock()
	c.botList = bots
}

// MockTasks sets the tasks that can be returned from ListTasks, ListSkiaTasks,
// GetTaskMetadata, and GetTask. Replaces any previous tasks, including those
// automatically added by TriggerTask.
func (c *TestClient) MockTasks(tasks []*apipb.TaskRequestMetadataResponse) {
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	c.taskList = tasks
}

// DoMockTasks calls f for each mocked task, allowing goroutine-safe updates. f
// must not call any other method on c.
func (c *TestClient) DoMockTasks(f func(*apipb.TaskRequestMetadataResponse)) {
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

// These aren't used by our tests.
func (c *TestClient) BatchGetResult(context.Context, *apipb.BatchGetResultRequest, ...grpc.CallOption) (*apipb.BatchGetResultResponse, error) {
	return nil, skerr.Fmt("not implemented")
}
func (c *TestClient) CancelTasks(context.Context, *apipb.TasksCancelRequest, ...grpc.CallOption) (*apipb.TasksCancelResponse, error) {
	return nil, skerr.Fmt("not implemented")
}
func (c *TestClient) CountTasks(context.Context, *apipb.TasksCountRequest, ...grpc.CallOption) (*apipb.TasksCount, error) {
	return nil, skerr.Fmt("not implemented")
}
func (c *TestClient) CountBots(context.Context, *apipb.BotsCountRequest, ...grpc.CallOption) (*apipb.BotsCount, error) {
	return nil, skerr.Fmt("not implemented")
}
func (c *TestClient) DeleteBot(_ context.Context, in *apipb.BotRequest, opts ...grpc.CallOption) (*apipb.DeleteResponse, error) {
	return nil, skerr.Fmt("not implemented")
}
func (c *TestClient) GetBot(_ context.Context, in *apipb.BotRequest, opts ...grpc.CallOption) (*apipb.BotInfo, error) {
	return nil, skerr.Fmt("not implemented")
}
func (c *TestClient) GetBotDimensions(_ context.Context, in *apipb.BotsDimensionsRequest, opts ...grpc.CallOption) (*apipb.BotsDimensions, error) {
	return nil, skerr.Fmt("not implemented")
}
func (c *TestClient) GetStdout(_ context.Context, in *apipb.TaskIdWithOffsetRequest, opts ...grpc.CallOption) (*apipb.TaskOutputResponse, error) {
	return nil, skerr.Fmt("not implemented")
}
func (c *TestClient) ListBotEvents(_ context.Context, in *apipb.BotEventsRequest, opts ...grpc.CallOption) (*apipb.BotEventsResponse, error) {
	return nil, skerr.Fmt("not implemented")
}
func (c *TestClient) ListTaskRequests(_ context.Context, in *apipb.TasksRequest, opts ...grpc.CallOption) (*apipb.TaskRequestsResponse, error) {
	return nil, skerr.Fmt("not implemented")
}
func (c *TestClient) TerminateBot(_ context.Context, in *apipb.TerminateRequest, opts ...grpc.CallOption) (*apipb.TerminateResponse, error) {
	return nil, skerr.Fmt("not implemented")
}

var _ swarmingv2.SwarmingV2Client = &TestClient{}
