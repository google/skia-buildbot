package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/cenkalti/backoff"
	"github.com/invopop/jsonschema"
	m3_mcp "github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/autogardener/go/logs"
	"go.skia.org/infra/autogardener/go/mcp"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/autogardener/go/utils"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/mcp/services/skia/task_details"
	"go.skia.org/infra/mcp/services/skia/task_scheduler"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/td"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/genai"
)

const (
	// Number of log lines on either side of a detected "interesting" log line
	// to include in snippets.
	logSnippetContext = 5

	// Number of log lines at the end of the log to include in snippets.
	logSnippetLinesAtEnd = 50

	// Maximum length of an individual log snippet.
	maxLogSnippetLength = 100

	// Maximum number of log snippets.
	maxLogSnippetCount = 20
)

// Client is an interface for the Gemini client, used for testing.
type Client interface {
	GetTaskSummary(ctx context.Context, task *ts_types.Task) (*types.TaskSummary, error)
}

// clientImpl provides high-level interactions with Gemini.
type clientImpl struct {
	client           *genai.Client
	location         string
	cheapModel       string
	cheapModelRL     *utils.RateLimiter
	expensiveModel   string
	expensiveModelRL *utils.RateLimiter
	mcpClient        mcp.MCPClient
	project          string
	gcs              *storage.Client
	gcsBucketDebug   string
}

// NewClient returns a Client instance.
func NewClient(ctx context.Context, project, location, cheapModel, expensiveModel, apiKey, mcpServer, gcsBucketDebug string, cheapRPM, cheapTPM, expensiveRPM, expensiveTPM int) (Client, error) {
	mcpClient, err := mcp.NewMCPClient(ctx, mcpServer)
	if err != nil {
		sklog.Fatal(err)
	}
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Validate models.
	models, err := listModels(ctx, genaiClient)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to list available models")
	}
	sklog.Infof("Available models:\n%s", strings.Join(models, "\n"))
	foundCheap := false
	foundExpensive := false
	for _, model := range models {
		if model == cheapModel {
			foundCheap = true
		}
		if model == expensiveModel {
			foundExpensive = true
		}
	}
	if !foundCheap {
		return nil, skerr.Fmt("Cheap model %q not found.", cheapModel)
	}
	if !foundExpensive {
		return nil, skerr.Fmt("Expensive model %q not found.", expensiveModel)
	}

	ts, err := google.DefaultTokenSource(ctx, auth.ScopeReadWrite)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	gcs, err := storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &clientImpl{
		client:           genaiClient,
		location:         location,
		cheapModel:       cheapModel,
		expensiveModel:   expensiveModel,
		cheapModelRL:     utils.NewRateLimiter(cheapRPM, cheapTPM, cheapModel),
		expensiveModelRL: utils.NewRateLimiter(expensiveRPM, expensiveTPM, expensiveModel),
		mcpClient:        mcpClient,
		project:          project,
		gcs:              gcs,
		gcsBucketDebug:   gcsBucketDebug,
	}, nil
}

const generalPromptHeader = `
# Role

You are a gardener for the Skia project. Your role is to investigate any
failures in Skia's infrastructure, determine their root cause, and provide a
definitive report to developers.


# General Guidance

- Do **not** rely on task creation timestamps to establish chronological order.
  Tasks may be backfilled or retried and therefore the only reliable source is
  the Git commit history.
- If a task succeeded its most recent run (commit-wise, not timestamp-wise),
  then the problem is either resolved or the task is flaky.
- MISHAP indicates that something other than a normal test failure occurred.
  Examples: timeouts, machine disappeared during task execution. These types of
  failures *might* be associated with a commit (eg. additional work was added or
  a test was slowed down to the point where the task is starting to time out),
  but it's more likely that this is related to the machine(s) running the task.
- TSAN tasks are inherently inconsistent; the task will fail if a real problem
  is found, but problems are not found on every execution. Treat these as actual
  failures that need investigation, and include the thread safety findings in
  your report.
- If a task times out, try to determine what it was doing. For example, was it
  running a particular test?


`

func (c *clientImpl) GetTaskSummary(ctx context.Context, task *ts_types.Task) (*types.TaskSummary, error) {
	defer metrics2.FuncTimer().Stop()

	// Retrieve the task steps so that the agent doesn't have to.
	var taskSteps task_details.GetTaskStepsResult
	if err := mcp.CallToolJSON(ctx, c.mcpClient, "get_task_steps", map[string]interface{}{"task_id": task.Id}, &taskSteps); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Reduce down to only the failed steps. Extract log snippets.
	var allowedTools []string
	var taskStepsStr string
	var logSnippets []string
	logsByStep := map[string][]string{}
	if taskSteps.Recipe != nil {
		allowedTools = append(allowedTools, "get_recipe_step_logs")
		prunedRecipeSteps, failedSteps := pruneSuccessfulRecipeSteps(taskSteps.Recipe)
		taskSteps.Recipe = prunedRecipeSteps
		for _, s := range failedSteps {
			// The root step has an empty name. Its log contents are nonsensical
			// (as far as we're concerned), so we skip it.
			if s.Name == "" {
				continue
			}
			lines, err := c.fetchRecipeStepLogs(ctx, taskSteps.SwarmingTaskID, s.StdoutStream)
			if err != nil {
				sklog.Warningf("Failed to fetch recipe step logs for %s: %s", s.Name, err)
				continue
			}
			snippet := logs.RenderLineRanges(lines, logs.ExtractSnippets(lines, logSnippetContext, logSnippetLinesAtEnd, maxLogSnippetLength, maxLogSnippetCount))
			logSnippets = append(logSnippets, fmt.Sprintf("## Log for failed recipe step %q\n\n%s\n", s.Name, snippet))
			logsByStep[s.Name] = lines
		}
		taskStepsStr = taskSteps.String()
	} else if taskSteps.TaskDriver != nil {
		allowedTools = append(allowedTools, "get_task_driver_step_logs")
		prunedTaskDriver, failedSteps := pruneSuccessfulTaskDriverSteps(taskSteps.TaskDriver.StepDisplay)
		taskSteps.TaskDriver.StepDisplay = prunedTaskDriver
		for _, s := range failedSteps {
			lines, err := c.fetchTaskDriverStepLogs(ctx, task.Id, s.Id)
			if err != nil {
				sklog.Warningf("Failed to fetch task driver step logs for %s: %s", s.Name, err)
				continue
			}
			snippet := logs.RenderLineRanges(lines, logs.ExtractSnippets(lines, logSnippetContext, logSnippetLinesAtEnd, maxLogSnippetLength, maxLogSnippetCount))
			logSnippets = append(logSnippets, fmt.Sprintf("## Log for failed task driver step %q\n\n%s\n", s.Name, snippet))
			logsByStep[s.Name] = lines
		}
		taskStepsStr = taskSteps.String()
	} else if taskSteps.SwarmingTaskLogs != "" {
		lines := strings.Split(taskSteps.SwarmingTaskLogs, "\n")
		snippet := logs.RenderLineRanges(lines, logs.ExtractSnippets(lines, logSnippetContext, logSnippetLinesAtEnd, maxLogSnippetLength, maxLogSnippetCount))
		logSnippets = append(logSnippets, fmt.Sprintf("## Raw Swarming Task Log Snippet\n\n%s\n", snippet))
		taskStepsStr = fmt.Sprintf("(raw swarming task; no steps available)\nSwarming Task State: %s", taskSteps.SwarmingTaskState)
		logsByStep[""] = lines
	}

	snippetsStr := strings.Join(logSnippets, "\n")
	if snippetsStr == "" {
		snippetsStr = "(No log snippets could be retrieved automatically.)"
	}

	const promptTmpl = generalPromptHeader + `# Task: Extract the error message for failed task "%s"

**CRITICAL:** This is strictly a log analysis and data extraction workflow. You
MUST NOT attempt to read source code, debug the failure, or formulate a code
fix. ONLY use the tools specified below. DO NOT attempt to retrieve information
about any other tasks.

# Task Details

%s

# Task Steps (if any). Successful steps have been removed for clarity.

%s

# Failed Step Log Snippets

Below are automatic snippets extracted from the failed step(s) in this task.
Attempt to identify the error using these snippets first, but be aware that
not all of the log lines are included and the automated extraction process may
have missed something. Do not hesitate to fetch more log lines if necessary.

%s

# Instructions & Workflow

To successfully complete this task, you MUST follow this exact workflow. Do not skip any steps.

## Workflow

1. Scan the list of steps above (if any) and find the relevant failed step(s).
2. Scan the logs of the failed steps and extract a digestible snippet.
   - You have been given snippets that were extracted using a coarse heuristic
     process. They may or may not contain the actual error. If you have any
     suspicion that they do not, you MUST load more log lines.
   - **WARNING:** Skia tasks (especially "dm" and "nanobench") sometimes
     produce massive amounts of log spam (e.g., graphics API warnings, compiler
     warnings) that are non-fatal red herrings. Do not get distracted by them.
     If these do not immediately precede a test failure or an early exit, they
     are probably not relevant.
   - Do **not** assume a task timed out just because it ran for a long time or
     generated a lot of spam.
   - To find the _actual_ cause of failure, always check the **end** of the logs
     first. Specifically, look for a "Failures:" section or explicit test
     failure messages right before the step exits.
   - If the error is not extremely obvious, continue requesting log lines until
     you've either found an obvious error or you've seen all of the logs.
3. Analyze the error(s) and present a report to the user.
   - Be sure to include the entire error message or stack trace, unless it is
     excessively long (>100 lines). You can deduplicate lines as appropriate.
   - Be concise. Only include relevant information and avoid repeating or
     restating your conclusion. The error message will be included in the
     "ErrorMessage" section of the response, so do not include it in your
     analysis.
   - If you suspect a problem with the machine which ran the task, include the
     bot ID in your analysis.
`
	prompt := fmt.Sprintf(promptTmpl, task.Id, task_scheduler.TaskWrapper{Task: task}, taskStepsStr, snippetsStr)
	mcpWrapper := mcp.MCPClientWithPseudoTools(c.mcpClient, []*mcp.PseudoTool{
		mcp.GetLogLinesTool(logsByStep),
	}, allowedTools)
	var res types.TaskSummary
	if err := c.generate(ctx, prompt, c.cheapModel, c.cheapModelRL, mcpWrapper, fmt.Sprintf("GetTaskSummary/%s", task.Id), &res); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &res, nil
}

func pruneSuccessfulRecipeSteps(step *task_details.RecipeStep) (*task_details.RecipeStep, []*task_details.RecipeStep) {
	if step == nil {
		return nil, nil
	}
	var failedSteps []*task_details.RecipeStep
	if step.Status != "SUCCESS" {
		failedSteps = append(failedSteps, step)
	}
	var prunedSubsteps []*task_details.RecipeStep
	for _, sub := range step.Substeps {
		prunedSub, failedSubSteps := pruneSuccessfulRecipeSteps(sub)
		if prunedSub != nil {
			prunedSubsteps = append(prunedSubsteps, prunedSub)
		}
		failedSteps = append(failedSteps, failedSubSteps...)
	}

	if len(prunedSubsteps) > 0 || step.Status != "SUCCESS" {
		return &task_details.RecipeStep{
			Name:         step.Name,
			Status:       step.Status,
			StdoutStream: step.StdoutStream,
			StderrStream: step.StderrStream,
			Substeps:     prunedSubsteps,
		}, failedSteps
	}
	return nil, nil
}

func pruneSuccessfulTaskDriverSteps(step *display.StepDisplay) (*display.StepDisplay, []*display.StepDisplay) {
	if step == nil {
		return nil, nil
	}
	var failedSteps []*display.StepDisplay
	if step.Result != td.StepResultSuccess {
		failedSteps = append(failedSteps, step)
	}
	var prunedSubsteps []*display.StepDisplay
	for _, sub := range step.Steps {
		prunedSub, failedSubSteps := pruneSuccessfulTaskDriverSteps(sub)
		if prunedSub != nil {
			prunedSubsteps = append(prunedSubsteps, prunedSub)
		}
		failedSteps = append(failedSteps, failedSubSteps...)
	}

	if len(prunedSubsteps) > 0 || step.Result != td.StepResultSuccess {
		return &display.StepDisplay{
			StepProperties: step.StepProperties,
			Result:         step.Result,
			Errors:         step.Errors,
			Started:        step.Started,
			Finished:       step.Finished,
			Data:           step.Data,
			Steps:          prunedSubsteps,
		}, failedSteps
	}
	return nil, nil
}

func (c *clientImpl) fetchRecipeStepLogs(ctx context.Context, swarmingTaskID, logPath string) ([]string, error) {
	return c.fetchStepLogs(ctx, "get_recipe_step_logs", map[string]string{
		"swarming_task_id": swarmingTaskID,
		"log_path":         logPath,
	})
}

func (c *clientImpl) fetchTaskDriverStepLogs(ctx context.Context, taskID, stepID string) ([]string, error) {
	return c.fetchStepLogs(ctx, "get_task_driver_step_logs", map[string]string{
		"task_id": taskID,
		"step_id": stepID,
	})
}

func (c *clientImpl) fetchStepLogs(ctx context.Context, tool string, toolArgs map[string]string) ([]string, error) {
	var allLines []string
	cursor := ""
	for {
		var logsRes task_details.GetLogsResponse
		args := map[string]interface{}{
			"limit":   500,
			"reverse": false,
		}
		for k, v := range toolArgs {
			args[k] = v
		}
		if cursor != "" {
			args["cursor"] = cursor
		}
		if err := mcp.CallToolJSON(ctx, c.mcpClient, tool, args, &logsRes); err != nil {
			return nil, skerr.Wrap(err)
		}
		allLines = append(allLines, logsRes.Logs...)
		cursor = logsRes.Cursor
		if cursor == "" {
			break
		}
	}
	return allLines, nil
}

func (c *clientImpl) generate(ctx context.Context, prompt, model string, rl *utils.RateLimiter, mcpClient mcp.MCPClient, debugObjectPath string, result interface{}) (rvErr error) {
	// Gemini doesn't use MCP tools directly. Rather, it returns requests to use
	// tools as part of its response, expecting to see the results of those tool
	// calls in our next message. Therefore, we need to repeatedly send messages
	// in a loop, starting with our initial prompt and continuing to run tools
	// and present their results until Gemini returns its ultimate response.

	// Some models (e.g. Gemini 2.5) do not support function calling and JSON
	// mode simultaneously. Therefore, we run the tool-calling loop without
	// JSON mode, and then do one final request with JSON mode to get the
	// result.
	config := &genai.GenerateContentConfig{
		Tools: mcpClient.Tools(),
	}

	debug := &DebugInfo{
		Prompt: prompt,
		Model:  model,
		Config: config,
	}
	defer func() {
		if rvErr != nil {
			debug.Error = rvErr.Error()
		}
		go c.uploadDebugInfo(ctx, debug, debugObjectPath)
	}()

	chat, err := c.client.Chats.Create(ctx, model, config, nil)
	if err != nil {
		return skerr.Wrap(err)
	}

	var resp *genai.GenerateContentResponse
	backoffOp := fmt.Sprintf("SendMessage/%s", debugObjectPath)
	if err := utils.DoBackoff(backoffOp, func() error {
		parts := []genai.Part{{Text: prompt}}
		if err := rl.Wait(ctx, c.client, chat.History(false), parts); err != nil {
			return skerr.Wrap(err)
		}
		resp, err = chat.SendMessage(ctx, parts...)
		if err != nil && strings.Contains(err.Error(), "token count exceeds the maximum") {
			return backoff.Permanent(err)
		}
		return err
	}); err != nil {
		return skerr.Wrap(err)
	}

	for {
		functionCalls := resp.FunctionCalls()
		if len(functionCalls) == 0 {
			break
		}

		var toolResponses []genai.Part
		for _, fc := range functionCalls {
			toolRes, err := c.mcpClient.CallTool(ctx, fc.Name, fc.Args)
			if err != nil {
				return skerr.Wrapf(err, "tool call %s failed", fc.Name)
			}

			var sb strings.Builder
			for _, content := range toolRes.Content {
				if tc, ok := content.(m3_mcp.TextContent); ok {
					sb.WriteString(tc.Text)
				} else {
					sb.WriteString(fmt.Sprintf("%v", content))
				}
			}
			// According to the docs for genai.FunctionResponse.Response, the
			// "output" and "error" keys are special and differentiate between
			// normal output and a tool error. If not present, the whole map is
			// considered to be the tool output.
			key := "output"
			if toolRes.IsError {
				key = "error"
			}
			toolResponses = append(toolResponses, genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name: fc.Name,
					Response: map[string]any{
						key: sb.String(),
					},
				},
			})
			debug.ToolCalls = append(debug.ToolCalls, DebugInfo_ToolCall{
				Tool:   fc.Name,
				Args:   fc.Args,
				Result: sb.String(),
			})
		}
		if err := utils.DoBackoff(backoffOp, func() error {
			if err := rl.Wait(ctx, c.client, chat.History(false), toolResponses); err != nil {
				return skerr.Wrap(err)
			}
			resp, err = chat.SendMessage(ctx, toolResponses...)
			return err
		}); err != nil {
			return skerr.Wrap(err)
		}
	}

	// Now that the tool-calling loop is finished, we request the final result
	// using JSON mode.
	finalConfig := &genai.GenerateContentConfig{
		ResponseMIMEType:   "application/json",
		ResponseJsonSchema: jsonschema.Reflect(result),
	}
	if err := utils.DoBackoff("GenerateContent", func() error {
		// We use the history from the chat but perform a new GenerateContent
		// call with the JSON config.
		history := chat.History(false)
		// No new parts, just asking for the final structured output based on history.
		if err := rl.Wait(ctx, c.client, history, nil); err != nil {
			return skerr.Wrap(err)
		}
		resp, err = c.client.Models.GenerateContent(ctx, model, history, finalConfig)
		return err
	}); err != nil {
		return skerr.Wrap(err)
	}

	// Only the first part of the first candidate seems to be relevant.
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
		r := bytes.NewReader([]byte(resp.Candidates[0].Content.Parts[0].Text))
		debug.Result = resp.Candidates[0].Content.Parts[0].Text
		return skerr.Wrap(json.NewDecoder(r).Decode(result))
	}
	return skerr.Fmt("no output generated")
}

func (c *clientImpl) uploadDebugInfo(ctx context.Context, debug *DebugInfo, object string) {
	if c.gcsBucketDebug == "" {
		return
	}
	w := c.gcs.Bucket(c.gcsBucketDebug).Object(object).NewWriter(ctx)
	defer util.Close(w)
	if err := json.NewEncoder(w).Encode(debug); err != nil {
		sklog.Errorf("failed uploading debug info: %s", err)
	}
}

func listModels(ctx context.Context, client *genai.Client) ([]string, error) {
	page, err := client.Models.List(ctx, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	models := []string{}
	for {
		for _, m := range page.Items {
			if !strings.Contains(m.Name, "gemini") {
				continue
			}
			// The model names returned by the API might be prefixed with "models/".
			name := strings.TrimPrefix(m.Name, "models/")
			models = append(models, name)
		}
		if page.NextPageToken == "" {
			break
		}
		page, err = page.Next(ctx)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	sort.Strings(models)
	return models, nil
}

type summaryForTasksWithinTaskName struct {
	TaskIDs []string
	types.TaskSummary
}

func (r summaryForTasksWithinTaskName) String() string {
	return fmt.Sprintf(`**Task IDs:** %s
**Error:** %s
**Analysis:** %s
`, strings.Join(r.TaskIDs, ", "), r.ErrorMessage, r.Analysis)
}

type DebugInfo struct {
	Prompt    string                       `json:"prompt"`
	Model     string                       `json:"model"`
	Config    *genai.GenerateContentConfig `json:"config"`
	ToolCalls []DebugInfo_ToolCall         `json:"toolCalls"`
	Result    string                       `json:"result"`
	Error     string                       `json:"error"`
}

type DebugInfo_ToolCall struct {
	Tool   string                 `json:"tool"`
	Args   map[string]interface{} `json:"args"`
	Result string                 `json:"result"`
}
