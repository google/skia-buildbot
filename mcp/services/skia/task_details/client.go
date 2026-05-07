package task_details

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"github.com/mark3labs/mcp-go/mcp"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/logdog/client/coordinator"
	"go.chromium.org/luci/logdog/common/fetcher"
	"go.chromium.org/luci/logdog/common/types"
	annopb "go.chromium.org/luci/luciexe/legacy/annotee/proto"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	td_db "go.skia.org/infra/task_driver/go/db"
	td_bigtable "go.skia.org/infra/task_driver/go/db/bigtable"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/logs"
	ts_db "go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"golang.org/x/oauth2/google"
	"google.golang.org/protobuf/proto"
)

const (
	logdogProject = "skia"
	logdogHost    = "logs.chromium.org"

	logdogPathTmplRun      = "%s/+/annotations"
	logdogPathTmplStepLogs = "%s/+/%s"
)

type TaskDetailsClient struct {
	swarm  swarming.ApiClient
	td     td_db.DB
	tdLogs *logs.LogsManager
	ts     ts_db.DBCloser
	logdog *coordinator.Client
}

func NewClient(ctx context.Context, btProject, btInstance, firestoreInstance string) (*TaskDetailsClient, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, bigtable.Scope, datastore.ScopeDatastore)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	tdDB, err := td_bigtable.NewBigTableDB(ctx, btProject, btInstance, ts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	tsDB, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, firestoreInstance, ts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	c := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	swarm, err := swarming.NewApiClient(c, swarming.SWARMING_SERVER)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	prpcClient := prpc.Client{
		C:       c,
		Host:    logdogHost,
		Options: prpc.DefaultOptions(),
	}
	coord := coordinator.NewClient(&prpcClient)
	tdLogs, err := logs.NewLogsManager(ctx, btProject, btInstance, ts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &TaskDetailsClient{
		swarm:  swarm,
		td:     tdDB,
		tdLogs: tdLogs,
		ts:     tsDB,
		logdog: coord,
	}, nil
}

func (c *TaskDetailsClient) GetTaskStepsHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	taskID, err := req.RequireString(argTaskID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// The task might be run as a Task Driver, a Recipe, or just a plain task.
	// Try Task Driver first.
	var res GetTaskStepsResult
	td, err := c.td.GetTaskDriver(ctx, taskID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if td != nil {
		tdDisplay, err := display.TaskDriverForDisplay(td)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		res.TaskDriver = tdDisplay
		return &res, nil
	}

	// Fall back to Recipe steps via LogDog.
	task, err := c.ts.GetTaskById(ctx, taskID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	step, err := c.fetchLogDogSteps(ctx, task.SwarmingTaskId)
	if err == nil {
		res.Recipe = step
		// Populate SwarmingTaskID in case it's needed for log retrieval.
		res.SwarmingTaskID = task.SwarmingTaskId
		return &res, nil
	} else if !strings.Contains(err.Error(), "coordinator: no access") {
		return nil, skerr.Wrap(err)
	}

	// If we couldn't find recipe steps, just return the Swarming task logs.
	swarmOutput, err := c.swarm.GetStdoutOfTask(ctx, task.SwarmingTaskId)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	res.SwarmingTaskState = swarmOutput.State
	res.SwarmingTaskLogs = swarmOutput.Output
	return &res, nil
}

func (c *TaskDetailsClient) GetRecipeStepLogsHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	swarmingTaskID, err := req.RequireString(argSwarmingTaskID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	logPath, err := req.RequireString(argLogPath)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	startIndex, err := req.RequireInt(argStartIndex)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	limit, err := req.RequireInt(argLimit)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	lines, err := c.fetchLogDogStepLogs(ctx, swarmingTaskID, logPath, startIndex, limit)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return LogLines(lines), nil
}

func (c *TaskDetailsClient) GetTaskDriverLogsHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	taskID, err := req.RequireString(argTaskID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	stepID, err := req.RequireString(argStepID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	logID, err := req.RequireString(argLogID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	cursor := req.GetString(argCursor, "")
	limit := req.GetInt(argLimit, 0)

	logs, cursor, err := c.tdLogs.Search(ctx, taskID, stepID, logID, cursor, limit, false)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	response := TaskDriverLogsResponse{
		Cursor: cursor,
	}
	for _, entry := range logs {
		if entry.TextPayload != "" {
			response.Logs = append(response.Logs, entry.TextPayload)
		} else if entry.JsonPayload != nil {
			b, err := json.Marshal(entry.JsonPayload)
			if err != nil {
				return nil, skerr.Wrap(err)
			} else {
				response.Logs = append(response.Logs, string(b))
			}
		} else {
			response.Logs = append(response.Logs, "")
		}
	}

	return &response, nil
}

func (c *TaskDetailsClient) fetchLogDogSteps(ctx context.Context, taskID string) (*annopb.Step, error) {
	path := fmt.Sprintf(logdogPathTmplRun, fixupSwarmingTaskID(taskID))
	stream := c.logdog.Stream(logdogProject, types.StreamPath(path))
	var state coordinator.LogStream
	le, err := stream.Tail(ctx, coordinator.WithState(&state), coordinator.Complete())
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to tail stream")
	}
	if le == nil {
		return nil, skerr.Fmt("no annotation entries found in stream")
	}

	if state.Desc.ContentType != annopb.ContentTypeAnnotations {
		return nil, skerr.Fmt("expected annotations but found %s", state.Desc.ContentType)
	}
	dg := le.GetDatagram()
	if dg == nil {
		return nil, skerr.Fmt("no datagram found for step!")
	}
	var step annopb.Step
	if err := proto.Unmarshal(dg.Data, &step); err != nil {
		return nil, skerr.Wrapf(err, "failed to unmarshal datagram data")
	}
	return &step, nil
}

func (c *TaskDetailsClient) fetchLogDogStepLogs(ctx context.Context, taskID, logPath string, index, count int) ([]string, error) {
	path := fmt.Sprintf(logdogPathTmplStepLogs, fixupSwarmingTaskID(taskID), logPath)
	f := c.logdog.Stream(logdogProject, types.StreamPath(path)).Fetcher(ctx, &fetcher.Options{
		Index: types.MessageIndex(index),
		Count: int64(count),
	})

	var logLines []string
	for {
		le, err := f.NextLogEntry()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to fetch log entry")
		}
		if text := le.GetText(); text != nil {
			for _, line := range text.Lines {
				logLines = append(logLines, string(line.Value))
			}
		}
	}

	return logLines, nil
}

// fixupSwarmingTaskID ensures that the given Swarming task ID is a *run* ID as
// opposed to a *request* ID. The request ID ends with a zero, while the first
// for a given request ends in a one.
func fixupSwarmingTaskID(taskID string) string {
	if len(taskID) > 0 && taskID[len(taskID)-1] == '0' {
		return taskID[:len(taskID)-1] + "1"
	}
	return taskID
}

type GetTaskStepsResult struct {
	TaskDriver *display.TaskDriverRunDisplay `json:"task_driver,omitempty"`

	Recipe *annopb.Step `json:"recipe,omitempty"`

	SwarmingTaskID    string `json:"swarming_task_id,omitempty"`
	SwarmingTaskState string `json:"swarming_task_state,omitempty"`
	SwarmingTaskLogs  string `json:"swarming_task_logs,omitempty"`
}

func (r GetTaskStepsResult) String() string {
	var sb strings.Builder
	if r.TaskDriver != nil {
		_, _ = fmt.Fprintf(&sb, "# Task Driver\n\n")
		printTaskDriverStep(&sb, r.TaskDriver.StepDisplay, 0)
	} else if r.Recipe != nil {
		_, _ = fmt.Fprintf(&sb, "# Recipe\n\n")
		_, _ = fmt.Fprintf(&sb, "**Swarming Task ID:** %s\n", r.SwarmingTaskID)
		_, _ = fmt.Fprintf(&sb, "**Steps:**\n")
		printRecipeStep(&sb, r.Recipe, 0)
	} else {
		_, _ = fmt.Fprintf(&sb, "# Swarming Task\n\n")
		_, _ = fmt.Fprintf(&sb, "**ID:**    %s\n", r.SwarmingTaskID)
		_, _ = fmt.Fprintf(&sb, "**State:** %s\n", r.SwarmingTaskState)
		_, _ = fmt.Fprintf(&sb, "**Logs:**\n")
		_, _ = fmt.Fprintf(&sb, "```\n%s\n```\n", r.SwarmingTaskLogs)
	}
	return sb.String()
}

func printTaskDriverStep(w io.Writer, step *display.StepDisplay, depth int) {
	_, _ = fmt.Fprintf(w, "%s- %s (%s)\n", strings.Repeat("  ", depth), step.Name, step.Result)
	for _, subStep := range step.Steps {
		printTaskDriverStep(w, subStep, depth+1)
	}
}

func printRecipeStep(w io.Writer, step *annopb.Step, depth int) {
	_, _ = fmt.Fprintf(w, "%s- %s (%s)\n", strings.Repeat("  ", depth), step.Name, step.Status)
	for _, subStep := range step.Substep {
		printRecipeStep(w, subStep.GetStep(), depth+1)
	}
}

type LogLines []string

func (l LogLines) String() string {
	return strings.Join(l, "\n")
}

type TaskDriverLogsResponse struct {
	Logs   []string `json:"log_lines"`
	Cursor string   `json:"cursor"`
}

func (r TaskDriverLogsResponse) String() string {
	str := ""
	if r.Cursor != "" {
		str = fmt.Sprintf("# Cursor:\n\n%s\n\n", r.Cursor)
	}
	return str + fmt.Sprintf("# Logs:\n\n%s", strings.Join(r.Logs, "\n"))
}
