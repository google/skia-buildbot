package swarming

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	API_BASE_PATH_PATTERN = "https://%s/_ah/api/swarming/v1/"
	AUTH_SCOPE            = "https://www.googleapis.com/auth/userinfo.email"

	DIMENSION_DEVICE_OS_KEY            = "device_os"
	DIMENSION_DEVICE_TYPE_KEY          = "device_type"
	DIMENSION_GPU_KEY                  = "gpu"
	DIMENSION_OS_KEY                   = "os"
	DIMENSION_POOL_KEY                 = "pool"
	DIMENSION_POOL_VALUE_SKIA          = "Skia"
	DIMENSION_POOL_VALUE_SKIA_CT       = "SkiaCT"
	DIMENSION_POOL_VALUE_SKIA_INTERNAL = "SkiaInternal"
	DIMENSION_POOL_VALUE_CT            = "CT"
	DIMENSION_QUARANTINED_KEY          = "quarantined"

	TASK_STATE_BOT_DIED    = "BOT_DIED"
	TASK_STATE_CANCELED    = "CANCELED"
	TASK_STATE_COMPLETED   = "COMPLETED"
	TASK_STATE_EXPIRED     = "EXPIRED"
	TASK_STATE_KILLED      = "KILLED"
	TASK_STATE_NO_RESOURCE = "NO_RESOURCE"
	TASK_STATE_PENDING     = "PENDING"
	TASK_STATE_RUNNING     = "RUNNING"
	TASK_STATE_TIMED_OUT   = "TIMED_OUT"

	// TIMESTAMP_FORMAT represents the timestamp format used by Swarming APIs. Use
	// with time.Parse/time.Format.
	TIMESTAMP_FORMAT = "2006-01-02T15:04:05.999999"
)

var (
	POOLS_PUBLIC  = []string{DIMENSION_POOL_VALUE_SKIA, DIMENSION_POOL_VALUE_SKIA_CT}
	POOLS_PRIVATE = []string{DIMENSION_POOL_VALUE_CT, DIMENSION_POOL_VALUE_SKIA_INTERNAL}

	TASK_STATES = []string{
		TASK_STATE_BOT_DIED,
		TASK_STATE_CANCELED,
		TASK_STATE_COMPLETED,
		TASK_STATE_EXPIRED,
		TASK_STATE_KILLED,
		TASK_STATE_PENDING,
		TASK_STATE_RUNNING,
		TASK_STATE_TIMED_OUT,
	}

	retriesRE = regexp.MustCompile("retries:([0-9])*")
)

// ApiClient is a Skia-specific wrapper around the Swarming API.
type ApiClient interface {
	// SwarmingService returns the underlying swarming.Service object.
	SwarmingService() *swarming.Service

	// ListBots returns a slice of swarming.SwarmingRpcsBotInfo instances
	// corresponding to the Swarming bots matching the requested dimensions.
	ListBots(ctx context.Context, dimensions map[string]string) ([]*swarming.SwarmingRpcsBotInfo, error)

	// ListFreeBots returns a slice of swarming.SwarmingRpcsBotInfo instances
	// corresponding to the free, alive, and not-quarantined bots in the
	// given pool.
	ListFreeBots(ctx context.Context, pool string) ([]*swarming.SwarmingRpcsBotInfo, error)

	// ListDownBots returns a slice of swarming.SwarmingRpcsBotInfo instances
	// corresponding to the dead or quarantined bots in the given pool.
	ListDownBots(ctx context.Context, pool string) ([]*swarming.SwarmingRpcsBotInfo, error)

	// ListBotsForPool returns a slice of swarming.SwarmingRpcsBotInfo
	// instances corresponding to the Swarming bots in the given pool.
	ListBotsForPool(ctx context.Context, pool string) ([]*swarming.SwarmingRpcsBotInfo, error)

	// GetStates returns a slice of states corresponding to the given task
	// IDs.
	GetStates(ctx context.Context, ids []string) ([]string, error)

	GetStdoutOfTask(ctx context.Context, id string) (*swarming.SwarmingRpcsTaskOutput, error)

	GracefullyShutdownBot(ctx context.Context, id string) (*swarming.SwarmingRpcsTerminateResponse, error)

	// ListBotTasks returns a slice of SwarmingRpcsTaskResult that are the last
	// N tasks done by a bot. When limit is big (>100), this call is very expensive.
	ListBotTasks(ctx context.Context, botID string, limit int) ([]*swarming.SwarmingRpcsTaskResult, error)

	// ListTasks returns a slice of swarming.SwarmingRpcsTaskRequestMetadata
	// instances corresponding to the specified tags and within given time window.
	// The results will have TaskId, TaskResult, and Request fields populated.
	// Specify time.Time{} for start and end if you do not want to restrict on
	// time. Specify "" for state if you do not want to restrict on state.
	ListTasks(ctx context.Context, start, end time.Time, tags []string, state string) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error)

	// ListSkiaTasks is ListTasks limited to pool:Skia.
	ListSkiaTasks(ctx context.Context, start, end time.Time) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error)

	// ListTaskResults returns a slice of swarming.SwarmingRpcsTaskResult
	// instances corresponding to the specified tags and within given time window.
	// Specify time.Time{} for start and end if you do not want to restrict on
	// time. Specify "" for state if you do not want to restrict on state.
	// includePerformanceStats indicates whether or not to load performance
	// information (eg. overhead) in addition to the normal task data.
	ListTaskResults(ctx context.Context, start, end time.Time, tags []string, state string, includePerformanceStats bool) ([]*swarming.SwarmingRpcsTaskResult, error)

	// CancelTask cancels the task with the given ID.
	CancelTask(ctx context.Context, id string, killRunning bool) error

	// TriggerTask triggers a task with the given request.
	TriggerTask(ctx context.Context, t *swarming.SwarmingRpcsNewTaskRequest) (*swarming.SwarmingRpcsTaskRequestMetadata, error)

	// RetryTask triggers a retry of the given task.
	RetryTask(ctx context.Context, t *swarming.SwarmingRpcsTaskRequestMetadata) (*swarming.SwarmingRpcsTaskRequestMetadata, error)

	// GetTask returns a swarming.SwarmingRpcsTaskResult instance
	// corresponding to the given Swarming task.
	GetTask(ctx context.Context, id string, includePerformanceStats bool) (*swarming.SwarmingRpcsTaskResult, error)

	// GetTaskMetadata returns a swarming.SwarmingRpcsTaskRequestMetadata instance
	// corresponding to the given Swarming task.
	GetTaskMetadata(ctx context.Context, id string) (*swarming.SwarmingRpcsTaskRequestMetadata, error)

	DeleteBots(ctx context.Context, bots []string) error
}

type apiClient struct {
	s *swarming.Service
}

// NewApiClient returns an ApiClient instance which uses the given authenticated
// http.Client.
func NewApiClient(c *http.Client, server string) (*apiClient, error) {
	s, err := swarming.New(c)
	if err != nil {
		return nil, err
	}
	s.BasePath = fmt.Sprintf(API_BASE_PATH_PATTERN, server)
	return &apiClient{s}, nil
}

func (c *apiClient) SwarmingService() *swarming.Service {
	return c.s
}

func (c *apiClient) ListBotsForPool(ctx context.Context, pool string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(ctx, map[string]string{
		DIMENSION_POOL_KEY: pool,
	})
}

func (c *apiClient) ListFreeBots(ctx context.Context, pool string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	call := c.s.Bots.List()
	call.Dimensions(fmt.Sprintf("%s:%s", DIMENSION_POOL_KEY, pool))
	call.IsBusy("FALSE")
	call.IsDead("FALSE")
	call.Quarantined("FALSE")
	return ProcessBotsListCall(ctx, call)
}

func (c *apiClient) ListDownBots(ctx context.Context, pool string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	call := c.s.Bots.List()
	call.Dimensions(fmt.Sprintf("%s:%s", DIMENSION_POOL_KEY, pool))
	call.IsDead("TRUE")
	dead, err := ProcessBotsListCall(ctx, call)
	if err != nil {
		return nil, err
	}
	call = c.s.Bots.List()
	call.Dimensions(fmt.Sprintf("%s:%s", DIMENSION_POOL_KEY, pool))
	call.Quarantined("TRUE")
	qBots, err := ProcessBotsListCall(ctx, call)
	if err != nil {
		return nil, err
	}
	return append(dead, qBots...), nil
}

func (c *apiClient) ListBots(ctx context.Context, dimensions map[string]string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	call := c.s.Bots.List()
	dimensionStrs := make([]string, 0, len(dimensions))
	for k, v := range dimensions {
		dimensionStrs = append(dimensionStrs, fmt.Sprintf("%s:%s", k, v))
	}
	call.Dimensions(dimensionStrs...)
	return ProcessBotsListCall(ctx, call)
}

func ProcessBotsListCall(ctx context.Context, call *swarming.BotsListCall) ([]*swarming.SwarmingRpcsBotInfo, error) {
	bots := []*swarming.SwarmingRpcsBotInfo{}
	cursor := ""
	call.Context(ctx)
	for {
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

func (c *apiClient) GetStates(ctx context.Context, ids []string) ([]string, error) {
	resp, err := c.s.Tasks.GetStates().TaskId(ids...).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return resp.States, nil
}

func (c *apiClient) GetStdoutOfTask(ctx context.Context, id string) (*swarming.SwarmingRpcsTaskOutput, error) {
	return c.s.Task.Stdout(id).Context(ctx).Do()
}

func (c *apiClient) GracefullyShutdownBot(ctx context.Context, id string) (*swarming.SwarmingRpcsTerminateResponse, error) {
	return c.s.Bot.Terminate(id).Context(ctx).Do()
}

type limitOption struct {
	limit int
}

func (l *limitOption) Get() (string, string) {
	return "limit", strconv.Itoa(l.limit)
}

func (c *apiClient) ListBotTasks(ctx context.Context, botID string, limit int) ([]*swarming.SwarmingRpcsTaskResult, error) {
	// The paramaters for Do() are a list of things that implement the Get() method
	// which returns a key and a value. This gets turned into key=value on the url
	// request, which works, even though Limit is not part of the client library.
	res, err := c.s.Bot.Tasks(botID).Context(ctx).Do(&limitOption{limit: 1})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func (c *apiClient) ListSkiaTasks(ctx context.Context, start, end time.Time) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.ListTasks(ctx, start, end, []string{"pool:Skia"}, "")
}

func (c *apiClient) ListTaskResults(ctx context.Context, start, end time.Time, tags []string, state string, includePerformanceStats bool) ([]*swarming.SwarmingRpcsTaskResult, error) {
	tasks := []*swarming.SwarmingRpcsTaskResult{}
	cursor := ""
	for {
		list := c.s.Tasks.List()
		if state != "" {
			list.State(state)
		}
		list.Context(ctx)
		list.Limit(100)
		list.Tags(tags...)
		list.IncludePerformanceStats(includePerformanceStats)
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
			return nil, err
		}
		tasks = append(tasks, res.Items...)
		if len(res.Items) == 0 || res.Cursor == "" {
			break
		}
		cursor = res.Cursor
	}
	return tasks, nil
}

// listTaskRequests is a helper for ListTasks.
func (c *apiClient) listTaskRequests(ctx context.Context, start, end time.Time, tags []string, state string) ([]*swarming.SwarmingRpcsTaskRequest, error) {
	reqs := []*swarming.SwarmingRpcsTaskRequest{}
	cursor := ""
	for {
		list := c.s.Tasks.Requests()
		if state != "" {
			list.State(state)
		}
		list.Context(ctx)
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
			return nil, err
		}
		reqs = append(reqs, res.Items...)
		if len(res.Items) == 0 || res.Cursor == "" {
			break
		}
		cursor = res.Cursor
	}
	return reqs, nil
}

func (c *apiClient) ListTasks(ctx context.Context, start, end time.Time, tags []string, state string) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	var wg sync.WaitGroup
	var tasks []*swarming.SwarmingRpcsTaskResult
	var tasksErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		tasks, tasksErr = c.ListTaskResults(ctx, start, end, tags, state, true)
	}()

	var reqs []*swarming.SwarmingRpcsTaskRequest
	var reqsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		reqs, reqsErr = c.listTaskRequests(ctx, start, end, tags, state)
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
		sklog.Warningf("Got different numbers of task requests and results.")
	}
	rv := make([]*swarming.SwarmingRpcsTaskRequestMetadata, 0, len(tasks))
	for _, t := range tasks {
		data := &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     t.TaskId,
			TaskResult: t,
		}
		for i, r := range reqs {
			if util.NewStringSet(t.Tags).Equals(util.NewStringSet(r.Tags)) {
				data.Request = r
				reqs = append(reqs[:i], reqs[i+1:]...)
				break
			}
		}
		if data.Request == nil {
			sklog.Warningf("Failed to find request for task %s", data.TaskId)
			continue
		}
		rv = append(rv, data)
	}
	if len(reqs) != 0 {
		return nil, fmt.Errorf("Failed to find tasks for %d requests", len(reqs))
	}

	return rv, nil
}

func (c *apiClient) CancelTask(ctx context.Context, id string, killRunning bool) error {
	req, reqErr := c.s.Task.Cancel(id, &swarming.SwarmingRpcsTaskCancelRequest{
		KillRunning: killRunning,
	}).Context(ctx).Do()
	if reqErr != nil {
		return reqErr
	}
	if !req.Ok {
		return fmt.Errorf("Could not cancel task %s", id)
	}
	return nil
}

func (c *apiClient) TriggerTask(ctx context.Context, t *swarming.SwarmingRpcsNewTaskRequest) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.s.Tasks.New(t).Context(ctx).Do()
}

func (c *apiClient) RetryTask(ctx context.Context, t *swarming.SwarmingRpcsTaskRequestMetadata) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	// Swarming API does not have a way to Retry commands. This was done
	// intentionally by swarming-eng to reduce API surface.
	newReq := &swarming.SwarmingRpcsNewTaskRequest{}
	newReq.Name = fmt.Sprintf("%s (retry)", t.Request.Name)
	newReq.ParentTaskId = t.Request.ParentTaskId
	newReq.Priority = t.Request.Priority
	newReq.PubsubTopic = t.Request.PubsubTopic
	newReq.PubsubUserdata = t.Request.PubsubUserdata
	newReq.User = t.Request.User
	newReq.ForceSendFields = t.Request.ForceSendFields
	newReq.TaskSlices = t.Request.TaskSlices
	if newReq.TaskSlices == nil {
		newReq.ExpirationSecs = t.Request.ExpirationSecs
		newReq.Properties = t.Request.Properties
	}

	newReq.Tags = t.Request.Tags
	// Add retries tag. Increment it if it already exists.
	foundRetriesTag := false
	for i, tag := range newReq.Tags {
		if retriesRE.FindString(tag) != "" {
			n, err := strconv.Atoi(strings.Split(tag, ":")[1])
			if err != nil {
				sklog.Errorf("retries value in %s is not numeric: %s", tag, err)
				continue
			}
			newReq.Tags[i] = fmt.Sprintf("retries:%d", (n + 1))
			foundRetriesTag = true
		}
	}
	if !foundRetriesTag {
		newReq.Tags = append(newReq.Tags, "retries:1")
	}

	return c.TriggerTask(ctx, newReq)
}

func (c *apiClient) GetTask(ctx context.Context, id string, includePerformanceStats bool) (*swarming.SwarmingRpcsTaskResult, error) {
	call := c.s.Task.Result(id).Context(ctx)
	call.IncludePerformanceStats(includePerformanceStats)
	return call.Do()
}

func (c *apiClient) GetTaskMetadata(ctx context.Context, id string) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	var wg sync.WaitGroup

	// Get the task result.
	var task *swarming.SwarmingRpcsTaskResult
	var taskErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		task, taskErr = c.GetTask(ctx, id, true)
	}()

	// Get the task request.
	var req *swarming.SwarmingRpcsTaskRequest
	var reqErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		req, reqErr = c.s.Task.Request(id).Context(ctx).Do()
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

func (c *apiClient) DeleteBots(ctx context.Context, bots []string) error {
	// Perform the requested operation.
	group := util.NewNamedErrGroup()
	for _, b := range bots {
		b := b // https://golang.org/doc/faq#closures_and_goroutines
		group.Go(b, func() error {
			r, err := c.s.Bot.Delete(b).Context(ctx).Do()
			if err != nil {
				return err
			}
			if !r.Deleted {
				return fmt.Errorf("Could not delete swarming bot: %s", b)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return nil
}

// BotDimensionsToStringMap converts Swarming bot dimensions as represented in
// the Swarming API to a map[string][]string.
func BotDimensionsToStringMap(dims []*swarming.SwarmingRpcsStringListPair) map[string][]string {
	m := make(map[string][]string, len(dims))
	for _, pair := range dims {
		m[pair.Key] = pair.Value
	}
	return m
}

// TaskDimensionsToStringMap converts Swarming task dimensions as represented
// in the Swarming API to a map[string][]string.
func TaskDimensionsToStringMap(dims []*swarming.SwarmingRpcsStringPair) map[string][]string {
	m := make(map[string][]string, len(dims))
	for _, pair := range dims {
		m[pair.Key] = []string{pair.Value}
	}
	return m
}

// BotDimensionsToStringSlice converts Swarming bot dimensions as represented
// in the Swarming API to a []string.
func BotDimensionsToStringSlice(dims []*swarming.SwarmingRpcsStringListPair) []string {
	return PackageDimensions(BotDimensionsToStringMap(dims))
}

// TaskDimensionsToStringSlice converts Swarming task dimensions as represented
// in the Swarming API to a []string.
func TaskDimensionsToStringSlice(dims []*swarming.SwarmingRpcsStringPair) []string {
	return PackageDimensions(TaskDimensionsToStringMap(dims))
}

// StringMapToBotDimensions converts Swarming bot dimensions from a
// map[string][]string to their Swarming API representation.
func StringMapToBotDimensions(dims map[string][]string) []*swarming.SwarmingRpcsStringListPair {
	dimensions := make([]*swarming.SwarmingRpcsStringListPair, 0, len(dims))
	for k, v := range dims {
		dimensions = append(dimensions, &swarming.SwarmingRpcsStringListPair{
			Key:   k,
			Value: v,
		})
	}
	return dimensions
}

// StringMapToTaskDimensions converts Swarming task dimensions from a
// map[string]string to their Swarming API representation.
func StringMapToTaskDimensions(dims map[string]string) []*swarming.SwarmingRpcsStringPair {
	dimensions := make([]*swarming.SwarmingRpcsStringPair, 0, len(dims))
	for k, v := range dims {
		dimensions = append(dimensions, &swarming.SwarmingRpcsStringPair{
			Key:   k,
			Value: v,
		})
	}
	return dimensions
}

// ParseDimensions parses a string slice of dimensions into a map[string][]string.
func ParseDimensions(dims []string) (map[string][]string, error) {
	rv := make(map[string][]string, len(dims))
	for _, dim := range dims {
		split := strings.SplitN(dim, ":", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("key/value pairs must take the form \"key:value\"; %q is invalid", dim)
		}
		rv[split[0]] = append(rv[split[0]], split[1])
	}
	return rv, nil
}

// ParseDimensionsSingleValue parses the MultiString flag into a
// map[string]string. Like ParseDimensions, except a single value is expected
// for each key.
func ParseDimensionsSingleValue(dimensions []string) (map[string]string, error) {
	dims, err := ParseDimensions(dimensions)
	if err != nil {
		return nil, err
	}
	rv := make(map[string]string, len(dims))
	for k, vals := range dims {
		if len(vals) != 1 {
			return nil, fmt.Errorf("Expected a single value for dimension %q; got: %v", k, vals)
		}
		rv[k] = vals[0]
	}
	return rv, nil
}

// PackageDimensions packages a map[string][]string of dimensions into a []string.
func PackageDimensions(dims map[string][]string) []string {
	rv := make([]string, 0, len(dims))
	for k, vals := range dims {
		for _, v := range vals {
			rv = append(rv, fmt.Sprintf("%s:%s", k, v))
		}
	}
	// Sort to make test results predictable.
	sort.Strings(rv)
	return rv
}

// ParseTags parses a string slice of tags into a map[string][]string.
func ParseTags(tags []string) (map[string][]string, error) {
	return ParseDimensions(tags)
}

// PackageTags packages a map[string]string of tags into a []string.
func PackageTags(tags map[string][]string) []string {
	return PackageDimensions(tags)
}

// GetTagValue returns the value for the given tag key from the given Swarming task.
func GetTagValue(t *swarming.SwarmingRpcsTaskResult, tagKey string) (string, error) {
	tagValues, err := ParseTags(t.Tags)
	if err != nil {
		return "", err
	}
	val := tagValues[tagKey]
	if len(val) != 1 {
		return "", fmt.Errorf("Expected a single value for tag key %q", tagKey)
	}
	return val[0], nil
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

// ConvertCIPDInput converts a slice of cipd.Package to a SwarmingRpcsCipdInput.
func ConvertCIPDInput(pkgs []*cipd.Package) *swarming.SwarmingRpcsCipdInput {
	rv := &swarming.SwarmingRpcsCipdInput{
		Packages: []*swarming.SwarmingRpcsCipdPackage{},
	}
	for _, pkg := range pkgs {
		rv.Packages = append(rv.Packages, &swarming.SwarmingRpcsCipdPackage{
			PackageName: pkg.Name,
			Path:        pkg.Path,
			Version:     pkg.Version,
		})
	}
	return rv
}

// GetTaskRequestProperties returns the SwarmingRpcsTaskProperties for the given
// SwarmingRpcsTaskRequestMetadata.
func GetTaskRequestProperties(t *swarming.SwarmingRpcsTaskRequestMetadata) *swarming.SwarmingRpcsTaskProperties {
	if len(t.Request.TaskSlices) > 0 {
		// TODO(borenet): It would probably be better to determine which
		// (if any) of the TaskSlices actually ran, rather than assuming
		// it was the first.
		return t.Request.TaskSlices[0].Properties
	}
	return t.Request.Properties
}

// MakeCASReference returns a SwarmingRpcsCASReference which can be used as input to
// a Swarming task.
func MakeCASReference(digest, casInstance string) (*swarming.SwarmingRpcsCASReference, error) {
	hash, size, err := rbe.StringToDigest(digest)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &swarming.SwarmingRpcsCASReference{
		CasInstance: casInstance,
		Digest: &swarming.SwarmingRpcsDigest{
			Hash:            hash,
			SizeBytes:       size,
			ForceSendFields: []string{"SizeBytes"},
		},
	}, nil
}

var _ ApiClient = &apiClient{}
