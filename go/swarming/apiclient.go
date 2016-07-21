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
	"go.skia.org/infra/go/util"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
)

const (
	AUTH_SCOPE    = "https://www.googleapis.com/auth/userinfo.email"
	API_BASE_PATH = "https://chromium-swarm.appspot.com/_ah/api/swarming/v1/"

	DIMENSION_POOL_KEY        = "pool"
	DIMENSION_POOL_VALUE_SKIA = "Skia"
	DIMENSION_POOL_VALUE_CT   = "CT"
)

var (
	retriesRE = regexp.MustCompile("retries:([0-9])*")
)

// ApiClient is a Skia-specific wrapper around the Swarming API.
type ApiClient struct {
	s *swarming.Service
}

// NewApiClient returns an ApiClient instance which uses the given authenticated
// http.Client.
func NewApiClient(c *http.Client) (*ApiClient, error) {
	s, err := swarming.New(c)
	if err != nil {
		return nil, err
	}
	s.BasePath = API_BASE_PATH
	return &ApiClient{s}, nil
}

// SwarmingService returns the underlying swarming.Service object.
func (c *ApiClient) SwarmingService() *swarming.Service {
	return c.s
}

// ListSkiaBots returns a slice of swarming.SwarmingRpcsBotInfo instances
// corresponding to the Skia Swarming bots.
func (c *ApiClient) ListSkiaBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_SKIA,
	})
}

// ListCTBots returns a slice of swarming.SwarmingRpcsBotInfo instances
// corresponding to the CT Swarming bots.
func (c *ApiClient) ListCTBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_CT,
	})
}

// ListBots returns a slice of swarming.SwarmingRpcsBotInfo instances
// corresponding to the Swarming bots matching the requested dimensions.
func (c *ApiClient) ListBots(dimensions map[string]string) ([]*swarming.SwarmingRpcsBotInfo, error) {
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

// ListSkiaTasks returns a slice of swarming.SwarmingRpcsTaskResult instances
// corresponding to Skia Swarming tasks within the given time window.
func (c *ApiClient) ListSkiaTasks(start, end time.Time) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.ListTasks(start, end, []string{"pool:Skia"}, "")
}

// ListTasks returns a slice of swarming.SwarmingRpcsTaskResult instances
// corresponding to the specified tags and within given time window.
// Specify time.Time{} for start and end if you do not want to restrict on time.
func (c *ApiClient) ListTasks(start, end time.Time, tags []string, state string) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
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

func (c *ApiClient) CancelTask(id string) error {
	req, reqErr := c.s.Task.Cancel(id).Do()
	if reqErr != nil {
		return reqErr
	}
	if !req.Ok {
		return fmt.Errorf("Could not cancel task %s", id)
	}
	return nil
}

func (c *ApiClient) TriggerTask(t *swarming.SwarmingRpcsNewTaskRequest) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.s.Tasks.New(t).Do()
}

func (c *ApiClient) RetryTask(t *swarming.SwarmingRpcsTaskRequestMetadata) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
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

// GetTask returns a swarming.SwarmingRpcsTaskRequestMetadata instance
// corresponding to the given Skia Swarming task.
func (c *ApiClient) GetTask(id string) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	var wg sync.WaitGroup

	// Get the task result.
	var task *swarming.SwarmingRpcsTaskResult
	var taskErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		call := c.s.Task.Result(id)
		call.IncludePerformanceStats(true)
		task, taskErr = call.Do()
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

// GetTagValue returns the value for the given tag key from the given Swarming task.
func GetTagValue(t *swarming.SwarmingRpcsTaskRequestMetadata, tagKey string) (string, error) {
	val := ""
	for _, tag := range t.TaskResult.Tags {
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

// parseTimestamp returns a time.Time for the given timestamp.
func parseTimestamp(ts string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05", ts)
}

// Created returns a time.Time for the given task's created time.
func Created(t *swarming.SwarmingRpcsTaskRequestMetadata) (time.Time, error) {
	return parseTimestamp(t.Request.CreatedTs)
}

// Started returns a time.Time for the given task's started time.
func Started(t *swarming.SwarmingRpcsTaskRequestMetadata) (time.Time, error) {
	return parseTimestamp(t.TaskResult.StartedTs)
}

// Completed returns a time.Time for the given task's started time.
func Completed(t *swarming.SwarmingRpcsTaskRequestMetadata) (time.Time, error) {
	return parseTimestamp(t.TaskResult.CompletedTs)
}
