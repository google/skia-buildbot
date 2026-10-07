package task_details

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"github.com/mark3labs/mcp-go/mcp"
	annopb "go.chromium.org/luci/luciexe/legacy/annotee/proto"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/go/timer"
	td_db "go.skia.org/infra/task_driver/go/db"
	td_bigtable "go.skia.org/infra/task_driver/go/db/bigtable"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/logs"
	ts_db "go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"golang.org/x/oauth2/google"
)

const (
	LogDogProject = "skia"
	logdogHost    = "logs.chromium.org"

	logdogPathTmplRun      = "%s/+/annotations"
	LogDogPathTmplStepLogs = "%s/+/%s"
)

type TaskDetailsClient struct {
	swarm           swarmingv2.SwarmingV2Client
	swarmHttpClient *http.Client
	td              td_db.DB
	tdLogs          *logs.LogsManager
	ts              ts_db.DBCloser
	logdog          LogDogClient
}

func NewClient(ctx context.Context, btProject, btInstance, firestoreInstance, swarmingServer string) (*TaskDetailsClient, error) {
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

	tdLogs, err := logs.NewLogsManager(ctx, btProject, btInstance, ts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	swarmHttpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	swarm := swarmingv2.NewDefaultClient(swarmHttpClient, swarmingServer)

	return &TaskDetailsClient{
		swarm:           swarm,
		swarmHttpClient: swarmHttpClient,
		td:              tdDB,
		tdLogs:          tdLogs,
		ts:              tsDB,
		logdog:          NewLogDogClient(ts),
	}, nil
}

func (c *TaskDetailsClient) GetTaskStepsHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	defer timer.New("GetTaskStepsHandler").Stop()
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

	// Retrieve the Swarming task state, as it'll be needed.
	task, err := c.ts.GetTaskById(ctx, taskID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if task == nil {
		return nil, skerr.Fmt("No such task with ID %s", taskID)
	}
	swarm := c.swarm
	if task.TaskExecutor != "" {
		swarm = swarmingv2.NewDefaultClient(c.swarmHttpClient, task.TaskExecutor)
	}
	swarmTask, err := swarm.GetResult(ctx, &apipb.TaskIdWithPerfRequest{TaskId: task.SwarmingTaskId})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	res.SwarmingTaskState = apipb.TaskState_name[int32(swarmTask.State)]
	res.SwarmingTaskID = task.SwarmingTaskId
	res.SwarmingBotID = task.SwarmingBotId

	// Fall back to Recipe steps via LogDog.
	step, finished, err := c.logdog.GetBuildSteps(ctx, LogDogProject, FixupSwarmingTaskID(task.SwarmingTaskId))
	if err == nil {
		res.Recipe = ToRecipeStep(step)
		if !finished && res.Recipe != nil {
			res.Recipe.Finished = false
		}
		return &res, nil
	} else if !strings.Contains(err.Error(), "coordinator: no access") {
		return nil, skerr.Wrap(err)
	}

	// If we couldn't find recipe steps, just return the Swarming task logs.
	swarmOutput, err := swarm.GetStdout(ctx, &apipb.TaskIdWithOffsetRequest{TaskId: task.SwarmingTaskId})
	if err != nil {
		if !strings.Contains(err.Error(), "404 page not found") {
			return nil, skerr.Wrap(err)
		}
	} else {
		res.SwarmingTaskLogs = string(swarmOutput.Output)
	}
	if strings.TrimSpace(res.SwarmingTaskLogs) == "" {
		res.SwarmingTaskLogs = "(no log output)"
	}
	return &res, nil
}

func (c *TaskDetailsClient) GetRecipeStepLogsHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	defer timer.New("GetRecipeStepLogsHandler").Stop()
	swarmingTaskID, err := req.RequireString(argSwarmingTaskID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	path, err := req.RequireString(argLogPath)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	limit := req.GetInt(argLimit, defaultLogLimit)
	if limit < 0 {
		return nil, skerr.Fmt("limit must be non-negative")
	}
	if limit > maxLogLimit {
		return nil, skerr.Fmt("limit must be 500 or less")
	}
	reverse := req.GetBool(argReverse, false)

	logPath := fmt.Sprintf(LogDogPathTmplStepLogs, FixupSwarmingTaskID(swarmingTaskID), path)

	// Decode the cursor to a starting index.
	// Note: in the reverse case, we use the last startIndex as the cursor, so
	// we derive the current startIndex from the cursor by subtracting the limit
	// and clamping at zero, adjusting the limit if needed.
	startIndex := 0
	if cursor := req.GetString(argCursor, ""); cursor != "" {
		startIndex, err = b64DecodeCursor(cursor)
	} else if reverse {
		// No starting index was provided, and we're loading in reverse. Use the
		// index of the last entry of the stream, plus one to account for the
		// fact that the index range is non-inclusive.
		lastEntry, _, err := c.logdog.GetLastEntry(ctx, LogDogProject, logPath)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		startIndex = int(lastEntry.StreamIndex) + 1
	}
	if reverse {
		if startIndex < limit {
			limit = startIndex
			startIndex = 0
		} else {
			startIndex = startIndex - limit
		}
	}

	// Retrieve the log lines.
	lines, nextIndex, done, err := c.fetchLogDogStepLogs(ctx, logPath, startIndex, limit)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to retrieve logs for task %q", swarmingTaskID)
	}

	// Find the next cursor.
	nextCursor := ""
	if reverse {
		// In the reverse case, we use the last startIndex as the cursor, but if
		// startIndex is zero, we're done.
		if startIndex > 0 {
			nextCursor = b64EncodeCursor(startIndex)
		}
	} else if !done {
		nextCursor = b64EncodeCursor(nextIndex)
	}

	return &GetLogsResponse{
		Cursor: nextCursor,
		Logs:   lines,
	}, nil
}

func b64EncodeCursor(index int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(index)))
}

func b64DecodeCursor(cursor string) (int, error) {
	cursorBytes, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, skerr.Wrapf(err, "invalid cursor %q", cursor)
	}
	startIndex, err := strconv.Atoi(string(cursorBytes))
	if err != nil {
		return 0, skerr.Wrapf(err, "invalid cursor %q", cursor)
	}
	return startIndex, nil
}

func (c *TaskDetailsClient) GetTaskDriverLogsHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	defer timer.New("GetTaskDriverLogsHandler").Stop()
	taskID, err := req.RequireString(argTaskID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	stepID, err := req.RequireString(argStepID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	cursor := req.GetString(argCursor, "")
	limit := req.GetInt(argLimit, defaultLogLimit)
	if limit < 0 {
		return nil, skerr.Fmt("limit must be non-negative")
	}
	if limit > maxLogLimit {
		return nil, skerr.Fmt("limit must be 500 or less")
	}
	logs, cursor, err := c.tdLogs.Search(ctx, taskID, stepID, "", cursor, limit, req.GetBool(argReverse, false))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	response := GetLogsResponse{
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

func (c *TaskDetailsClient) fetchLogDogStepLogs(ctx context.Context, logPath string, index, limit int) ([]string, int, bool, error) {
	entries, tidx, err := c.logdog.FetchLogEntries(ctx, LogDogProject, logPath, index, limit)
	if err != nil {
		return nil, 0, false, skerr.Wrap(err)
	}
	logLines := make([]string, 0, len(entries))
	for _, entry := range entries {
		if text := entry.GetText(); text != nil {
			for _, line := range text.Lines {
				logLines = append(logLines, string(line.Value))
			}
		}
	}
	if len(entries) == 0 {
		return logLines, index, true, nil
	}
	lastIndex := int(entries[len(entries)-1].StreamIndex)
	nextIndex := lastIndex + 1
	if tidx < 0 {
		return logLines, nextIndex, len(entries) < limit, nil
	}
	done := lastIndex >= int(tidx)
	return logLines, nextIndex, done, nil
}

// FixupSwarmingTaskID ensures that the given Swarming task ID is a *run* ID as
// opposed to a *request* ID. The request ID ends with a zero, while the first
// for a given request ends in a one.
func FixupSwarmingTaskID(taskID string) string {
	if len(taskID) > 0 && taskID[len(taskID)-1] == '0' {
		return taskID[:len(taskID)-1] + "1"
	}
	return taskID
}

type RecipeStep struct {
	Name         string        `json:"name"`
	Status       string        `json:"status"`
	StdoutStream string        `json:"stdout_stream,omitempty"`
	StderrStream string        `json:"stderr_stream,omitempty"`
	Substeps     []*RecipeStep `json:"substeps,omitempty"`
	Finished     bool          `json:"finished"`
}

type RecipeLog struct {
	Name string `json:"name"`
}

func ToRecipeStep(step *annopb.Step) *RecipeStep {
	if step == nil {
		return nil
	}
	res := &RecipeStep{
		Name:     step.Name,
		Status:   step.Status.String(),
		Finished: step.Ended != nil && step.Status != annopb.Status_RUNNING && step.Status != annopb.Status_PENDING,
	}
	if step.StdoutStream != nil {
		res.StdoutStream = step.StdoutStream.Name
	}
	if step.StderrStream != nil {
		res.StderrStream = step.StderrStream.Name
	}
	for _, sub := range step.Substep {
		if s := sub.GetStep(); s != nil {
			subStep := ToRecipeStep(s)
			if !subStep.Finished {
				res.Finished = false
			}
			res.Substeps = append(res.Substeps, subStep)
		}
	}
	return res
}

type GetTaskStepsResult struct {
	TaskDriver *display.TaskDriverRunDisplay `json:"task_driver,omitempty"`

	Recipe *RecipeStep `json:"recipe,omitempty"`

	SwarmingTaskID    string `json:"swarming_task_id,omitempty"`
	SwarmingTaskState string `json:"swarming_task_state,omitempty"`
	SwarmingBotID     string `json:"swarming_bot_id,omitempty"`
	SwarmingTaskLogs  string `json:"swarming_task_logs,omitempty"`
}

func (r GetTaskStepsResult) String() string {
	var sb strings.Builder
	if r.TaskDriver != nil && r.TaskDriver.StepDisplay != nil {
		_, _ = fmt.Fprintf(&sb, "## Task Driver details\n\n")
		printTaskDriverStep(&sb, r.TaskDriver.StepDisplay, 0)
	} else if r.Recipe != nil {
		_, _ = fmt.Fprintf(&sb, "## Recipe details\n\n")
		_, _ = fmt.Fprintf(&sb, "**Swarming Task ID:** %s\n", r.SwarmingTaskID)
		_, _ = fmt.Fprintf(&sb, "**Swarming Task State:** %s\n", r.SwarmingTaskState)
		_, _ = fmt.Fprintf(&sb, "**Swarming Bot ID:** %s\n", r.SwarmingBotID)
		_, _ = fmt.Fprintf(&sb, "**Steps:**\n")
		printRecipeStep(&sb, r.Recipe, 0)
	} else {
		_, _ = fmt.Fprintf(&sb, "## Raw Swarming Task (no steps available)\n\n")
		_, _ = fmt.Fprintf(&sb, "**Swarming Task ID:** %s\n", r.SwarmingTaskID)
		_, _ = fmt.Fprintf(&sb, "**Swarming Task State:** %s\n", r.SwarmingTaskState)
		_, _ = fmt.Fprintf(&sb, "**Swarming Bot ID:** %s\n", r.SwarmingBotID)
		_, _ = fmt.Fprintf(&sb, "**Logs:**\n")
		_, _ = fmt.Fprintf(&sb, "```\n%s\n```\n", r.SwarmingTaskLogs)
	}
	return sb.String()
}

func printTaskDriverStep(w io.Writer, step *display.StepDisplay, depth int) {
	_, _ = fmt.Fprintf(w, "%s- id=%s name=%q (%s)\n", strings.Repeat("  ", depth), step.Id, step.Name, step.Result)
	for _, subStep := range step.Steps {
		printTaskDriverStep(w, subStep, depth+1)
	}
}

func printRecipeStep(w io.Writer, step *RecipeStep, depth int) {
	indent := strings.Repeat("  ", depth)
	_, _ = fmt.Fprintf(w, "%s- %q (%s)\n", indent, step.Name, step.Status)
	if step.StdoutStream != "" {
		_, _ = fmt.Fprintf(w, "%s  stdout log path: %s\n", indent, step.StdoutStream)
	}
	if step.StderrStream != "" {
		_, _ = fmt.Fprintf(w, "%s  stderr log path: %s\n", indent, step.StderrStream)
	}
	for _, subStep := range step.Substeps {
		printRecipeStep(w, subStep, depth+1)
	}
}

type GetLogsResponse struct {
	Logs   []string `json:"log_lines"`
	Cursor string   `json:"cursor"`
}

func (r GetLogsResponse) String() string {
	str := ""
	if r.Cursor != "" {
		str = fmt.Sprintf("**Cursor:**\n\n%s\n\n", r.Cursor)
	}
	return str + fmt.Sprintf("**Logs:**\n\n```\n%s\n```", strings.Join(r.Logs, "\n"))
}
