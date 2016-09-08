package swarming

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
)

const (
	AUTH_SCOPE    = "https://www.googleapis.com/auth/userinfo.email"
	API_BASE_PATH = "https://chromium-swarm.appspot.com/_ah/api/swarming/v1/"

	DIMENSION_POOL_KEY                 = "pool"
	DIMENSION_POOL_VALUE_SKIA          = "Skia"
	DIMENSION_POOL_VALUE_SKIA_TRIGGERS = "SkiaTriggers"
	DIMENSION_POOL_VALUE_CT            = "CT"

	// TIMESTAMP_FORMAT represents the timestamp format used by Swarming APIs. Use
	// with time.Parse/time.Format.
	TIMESTAMP_FORMAT = "2006-01-02T15:04:05.999999"
)

var (
	retriesRE = regexp.MustCompile("retries:([0-9])*")
)

// ApiClient is a Skia-specific wrapper around the Swarming API.
type ApiClient interface {
	// SwarmingService returns the underlying swarming.Service object.
	SwarmingService() *swarming.Service

	// ListBots returns a slice of swarming.SwarmingRpcsBotInfo instances
	// corresponding to the Swarming bots matching the requested dimensions.
	ListBots(dimensions map[string]string) ([]*swarming.SwarmingRpcsBotInfo, error)

	// ListSkiaBots returns a slice of swarming.SwarmingRpcsBotInfo instances
	// corresponding to the Skia Swarming bots.
	ListSkiaBots() ([]*swarming.SwarmingRpcsBotInfo, error)

	// ListSkiaTriggerBots returns a slice of swarming.SwarmingRpcsBotInfo instances
	// corresponding to the Skia Swarming Trigger bots.
	ListSkiaTriggerBots() ([]*swarming.SwarmingRpcsBotInfo, error)

	// ListCTBots returns a slice of swarming.SwarmingRpcsBotInfo instances
	// corresponding to the CT Swarming bots.
	ListCTBots() ([]*swarming.SwarmingRpcsBotInfo, error)

	GracefullyShutdownBot(id string) (*swarming.SwarmingRpcsTerminateResponse, error)

	// ListTasks returns a slice of swarming.SwarmingRpcsTaskResult instances
	// corresponding to the specified tags and within given time window.
	// Specify time.Time{} for start and end if you do not want to restrict on time.
	ListTasks(start, end time.Time, tags []string, state string) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error)

	// ListSkiaTasks returns a slice of swarming.SwarmingRpcsTaskResult instances
	// corresponding to Skia Swarming tasks within the given time window.
	ListSkiaTasks(start, end time.Time) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error)

	// CancelTask cancels the task with the given ID.
	CancelTask(id string) error

	// TriggerTask triggers a task with the given request.
	TriggerTask(t *swarming.SwarmingRpcsNewTaskRequest) (*swarming.SwarmingRpcsTaskRequestMetadata, error)

	// RetryTask triggers a retry of the given task.
	RetryTask(t *swarming.SwarmingRpcsTaskRequestMetadata) (*swarming.SwarmingRpcsTaskRequestMetadata, error)

	// GetTask returns a swarming.SwarmingRpcsTaskResult instance
	// corresponding to the given Swarming task.
	GetTask(id string) (*swarming.SwarmingRpcsTaskResult, error)

	// GetTaskMetadata returns a swarming.SwarmingRpcsTaskRequestMetadata instance
	// corresponding to the given Swarming task.
	GetTaskMetadata(id string) (*swarming.SwarmingRpcsTaskRequestMetadata, error)
}

type apiClient struct {
	s *swarming.Service
}

// NewApiClient returns an ApiClient instance which uses the given authenticated
// http.Client.
func NewApiClient(c *http.Client) (ApiClient, error) {
	s, err := swarming.New(c)
	if err != nil {
		return nil, err
	}
	s.BasePath = API_BASE_PATH
	return &apiClient{s}, nil
}

func (c *apiClient) SwarmingService() *swarming.Service {
	return c.s
}

func (c *apiClient) ListSkiaBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_SKIA,
	})
}

func (c *apiClient) ListSkiaTriggerBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_SKIA_TRIGGERS,
	})
}

func (c *apiClient) ListCTBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_CT,
	})
}

func (c *apiClient) ListBots(dimensions map[string]string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	bots := []*swarming.SwarmingRpcsBotInfo{}
	cursor := ""
	for {
		call := c.s.Bots.List()
		dimensionStrs := make([]string, 0, len(dimensions))
		for k, v := range dimensions {
			dimensionStrs = append(dimensionStrs, fmt.Sprintf("%s:%s", k, v))
		}
		call.Dimensions(dimensionStrs...)
		call.Limit(100)
		if cursor != "" {
			call.Cursor(cursor)
		}
		res, err := call.Do()
		if err != nil {
			return nil, err
		}
		bots = append(bots, res.Items...)
		if len(res.Items) == 0 || res.Cursor == "" {
			break
		}
		cursor = res.Cursor
	}

	return bots, nil
}

func (c *apiClient) GracefullyShutdownBot(id string) (*swarming.SwarmingRpcsTerminateResponse, error) {
	return c.s.Bot.Terminate(id).Do()
}

func (c *apiClient) ListSkiaTasks(start, end time.Time) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.ListTasks(start, end, []string{"pool:Skia"}, "")
}

func (c *apiClient) ListTasks(start, end time.Time, tags []string, state string) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	var wg sync.WaitGroup

	// Query for task results.
	tasks := []*swarming.SwarmingRpcsTaskResult{}
	var tasksErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		cursor := ""
		for {
			list := c.s.Tasks.List()
			if state != "" {
				list.State(state)
			}
			list.Limit(100)
			list.Tags(tags...)
			list.IncludePerformanceStats(true)
			if !start.IsZero() {
				list.Start(float64(start.Unix()))
			}
			if !end.IsZero() {
				list.End(float64(end.Unix()))
			}
			if cursor != "" {
				list.Cursor(cursor)
			}
			res, err := list.Do()
			if err != nil {
				tasksErr = err
				return
			}
			tasks = append(tasks, res.Items...)
			if len(res.Items) == 0 || res.Cursor == "" {
				break
			}
			cursor = res.Cursor
		}
	}()

	// Query for task requests.
	reqs := []*swarming.SwarmingRpcsTaskRequest{}
	var reqsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		cursor := ""
		for {
			list := c.s.Tasks.Requests()
			if state != "" {
				list.State(state)
			}
			list.Limit(100)
			list.Tags(tags...)
			if !start.IsZero() {
				list.Start(float64(start.Unix()))
			}
			if !end.IsZero() {
				list.End(float64(end.Unix()))
			}
			if cursor != "" {
				list.Cursor(cursor)
			}
			res, err := list.Do()
			if err != nil {
				reqsErr = err
				return
			}
			reqs = append(reqs, res.Items...)
			if len(res.Items) == 0 || res.Cursor == "" {
				break
			}
			cursor = res.Cursor
		}
	}()

	wg.Wait()
	if tasksErr != nil {
		return nil, tasksErr
	}
	if reqsErr != nil {
		return nil, reqsErr
	}

	// Match requests to results.
	if len(tasks) != len(reqs) {
		glog.Warningf("Got different numbers of task requests and results.")
	}
	rv := make([]*swarming.SwarmingRpcsTaskRequestMetadata, 0, len(tasks))
	for _, t := range tasks {
		data := &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     t.TaskId,
			TaskResult: t,
		}
		for i, r := range reqs {
			if util.SSliceEqual(t.Tags, r.Tags) {
				data.Request = r
				reqs = append(reqs[:i], reqs[i+1:]...)
				break
			}
		}
		if data.Request == nil {
			glog.Warningf("Failed to find request for task %s", data.TaskId)
			continue
		}
		rv = append(rv, data)
	}
	if len(reqs) != 0 {
		return nil, fmt.Errorf("Failed to find tasks for %d requests", len(reqs))
	}

	return rv, nil
}

func (c *apiClient) CancelTask(id string) error {
	req, reqErr := c.s.Task.Cancel(id).Do()
	if reqErr != nil {
		return reqErr
	}
	if !req.Ok {
		return fmt.Errorf("Could not cancel task %s", id)
	}
	return nil
}

func (c *apiClient) TriggerTask(t *swarming.SwarmingRpcsNewTaskRequest) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.s.Tasks.New(t).Do()
}

func (c *apiClient) RetryTask(t *swarming.SwarmingRpcsTaskRequestMetadata) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	// Swarming API does not have a way to Retry commands. This was done
	// intentionally by swarming-eng to reduce API surface.
	newReq := &swarming.SwarmingRpcsNewTaskRequest{}
	newReq.Name = fmt.Sprintf("%s (retry)", t.Request.Name)
	newReq.ParentTaskId = t.Request.ParentTaskId
	newReq.ExpirationSecs = t.Request.ExpirationSecs
	newReq.Priority = t.Request.Priority
	newReq.Properties = t.Request.Properties
	newReq.PubsubTopic = t.Request.PubsubTopic
	newReq.PubsubUserdata = t.Request.PubsubUserdata
	newReq.User = t.Request.User
	newReq.ForceSendFields = t.Request.ForceSendFields

	newReq.Tags = t.Request.Tags
	// Add retries tag. Increment it if it already exists.
	foundRetriesTag := false
	for i, tag := range newReq.Tags {
		if retriesRE.FindString(tag) != "" {
			n, err := strconv.Atoi(strings.Split(tag, ":")[1])
			if err != nil {
				glog.Errorf("retries value in %s is not numeric: %s", tag, err)
				continue
			}
			newReq.Tags[i] = fmt.Sprintf("retries:%d", (n + 1))
			foundRetriesTag = true
		}
	}
	if !foundRetriesTag {
		newReq.Tags = append(newReq.Tags, "retries:1")
	}

	return c.TriggerTask(newReq)
}

func (c *apiClient) GetTask(id string) (*swarming.SwarmingRpcsTaskResult, error) {
	call := c.s.Task.Result(id)
	call.IncludePerformanceStats(true)
	return call.Do()
}

func (c *apiClient) GetTaskMetadata(id string) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	var wg sync.WaitGroup

	// Get the task result.
	var task *swarming.SwarmingRpcsTaskResult
	var taskErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		task, taskErr = c.GetTask(id)
	}()

	// Get the task request.
	var req *swarming.SwarmingRpcsTaskRequest
	var reqErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		req, reqErr = c.s.Task.Request(id).Do()
	}()

	wg.Wait()
	if taskErr != nil {
		return nil, taskErr
	}
	if reqErr != nil {
		return nil, reqErr
	}

	return &swarming.SwarmingRpcsTaskRequestMetadata{
		TaskId:     task.TaskId,
		TaskResult: task,
		Request:    req,
	}, nil
}

// TagValues returns map[tag_key]tag_value for all tags from the given Swarming task.
func TagValues(t *swarming.SwarmingRpcsTaskResult) (map[string]string, error) {
	rv := make(map[string]string, len(t.Tags))
	for _, tag := range t.Tags {
		split := strings.SplitN(tag, ":", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("Invalid Swarming task tag: %q %v", tag, t)
		}
		rv[split[0]] = split[1]
	}
	return rv, nil
}

// GetTagValue returns the value for the given tag key from the given Swarming task.
func GetTagValue(t *swarming.SwarmingRpcsTaskResult, tagKey string) (string, error) {
	val := ""
	for _, tag := range t.Tags {
		split := strings.SplitN(tag, ":", 2)
		if len(split) != 2 {
			return "", fmt.Errorf("Invalid Swarming task tag: %q", tag)
		}
		if split[0] == tagKey {
			val = split[1]
			break
		}
	}
	return val, nil
}

// ParseTimestamp returns a UTC time.Time for the given timestamp.
func ParseTimestamp(ts string) (time.Time, error) {
	return time.Parse(TIMESTAMP_FORMAT, ts)
}

// Created returns a time.Time for the given task's created time.
func Created(t *swarming.SwarmingRpcsTaskRequestMetadata) (time.Time, error) {
	return ParseTimestamp(t.Request.CreatedTs)
}

// Started returns a time.Time for the given task's started time.
func Started(t *swarming.SwarmingRpcsTaskRequestMetadata) (time.Time, error) {
	return ParseTimestamp(t.TaskResult.StartedTs)
}

// Completed returns a time.Time for the given task's started time.
func Completed(t *swarming.SwarmingRpcsTaskRequestMetadata) (time.Time, error) {
	return ParseTimestamp(t.TaskResult.CompletedTs)
}

func ParseDimensions(dimensionFlags *common.MultiString) (map[string]string, error) {
	dims := map[string]string{}
	if dimensionFlags == nil {
		return dims, nil
	}
	for _, dim := range *dimensionFlags {
		split := strings.SplitN(dim, ":", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("dimension must take the form \"key:value\"; %q is invalid", dim)
		}
		dims[split[0]] = split[1]
	}
	return dims, nil
}
