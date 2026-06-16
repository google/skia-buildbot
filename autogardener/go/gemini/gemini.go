package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/invopop/jsonschema"
	m3_mcp "github.com/mark3labs/mcp-go/mcp"
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
	ts_types "go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/genai"
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
		cheapModelRL:     utils.NewRateLimiter(cheapRPM, cheapTPM),
		expensiveModelRL: utils.NewRateLimiter(expensiveRPM, expensiveTPM),
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


`

func (c *clientImpl) GetTaskSummary(ctx context.Context, task *ts_types.Task) (*types.TaskSummary, error) {
	defer metrics2.FuncTimer().Stop()

	// Retrieve the task steps so that the agent doesn't have to.
	var taskSteps task_details.GetTaskStepsResult
	if err := mcp.CallToolJSON(ctx, c.mcpClient, "get_task_steps", map[string]interface{}{"task_id": task.Id}, &taskSteps); err != nil {
		return nil, skerr.Wrap(err)
	}

	const promptTmpl = generalPromptHeader + `# Task: Extract the error message for failed task "%s"

**CRITICAL:** This is strictly a log analysis and data extraction workflow. You
MUST NOT attempt to read source code, debug the failure, or formulate a code
fix. ONLY use the tools specified below. DO NOT attempt to retrieve information
about any other tasks.

## Workflow

1. Scan the list of steps below (if any) and find the relevant failed step(s).
   Note that some step failures may be expected, for example a step which tests
   file existence via some command that exits with a non-zero code when it does
   not exist.
2. If the task is a Task Driver or Recipe, you MUST retrieve the logs for the
   failed step. In the case of a raw swarming task, any logs are already
   included below - you do not need to retrieve anything else.
   - Use the "get_task_driver_step_logs" or "get_recipe_step_logs", depending on
     whether this task is a Task Driver or Recipe.
   - These tools are *paginated* and will return a cursor if additional log
     lines remain which were not included in the response. Pass the cursor back
     to the tool as an argument to retrieve the next page of logs. When the tool
     returns an empty cursor, you've seen all of the log lines.
   - Use the "reverse" argument to load log lines starting from the end.
3. Scan the logs and extract a digestible snippet.
   - **WARNING:** Skia tasks (especially "dm" and "nanobench") sometimes
     produce massive amounts of log spam (e.g., graphics API warnings, compiler
     warnings) that are non-fatal red herrings. Do not get distracted by them.
   - Do **not** assume a task timed out just because it ran for a long time or
     generated a lot of spam.
   - To find the _actual_ cause of failure, always check the **end** of the logs
     first. Specifically, look for a "Failures:" section or explicit test
     failure messages right before the step exits.
   - If the error is not extremely obvious, continue requesting log pages until
     you've either found an obvious error or you've seen all of the logs.
4. Analyze the error(s) and present a report to the user.
   - Be sure to include the entire error message or stack trace, unless it is
     excessively long (>100 lines). You can deduplicate lines as appropriate.
   - Be concise. Only include relevant information and avoid repeating or
     restating your conclusion. The error message will be included in the
     "ErrorMessage" section of the response, so do not include it in your
     analysis.
   - If you suspect a problem with the machine which ran the task, include the
     bot ID in your analysis.

%s

%s
`
	prompt := fmt.Sprintf(promptTmpl, task.Id, task_scheduler.TaskWrapper{Task: task}, taskSteps)
	allowedTools := []string{
		"get_task_driver_step_logs",
		"get_recipe_step_logs",
	}
	var res types.TaskSummary
	debugInfo, err := c.generate(ctx, prompt, c.cheapModel, c.cheapModelRL, allowedTools, &res)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	go c.uploadDebugInfo(ctx, debugInfo, "GetTaskSummary", task.Id)
	return &res, nil
}

func (c *clientImpl) generate(ctx context.Context, prompt, model string, rl *utils.RateLimiter, allowTools []string, result interface{}) (*DebugInfo, error) {
	debug := &DebugInfo{
		Prompt: prompt,
		Model:  model,
	}

	// Gemini doesn't use MCP tools directly. Rather, it returns requests to use
	// tools as part of its response, expecting to see the results of those tool
	// calls in our next message. Therefore, we need to repeatedly send messages
	// in a loop, starting with our initial prompt and continuing to run tools
	// and present their results until Gemini returns its ultimate response.

	// Some models (e.g. Gemini 2.5) do not support function calling and JSON
	// mode simultaneously. Therefore, we run the tool-calling loop without
	// JSON mode, and then do one final request with JSON mode to get the
	// result.
	toolConfig := &genai.GenerateContentConfig{}
	for _, tool := range c.mcpClient.Tools() {
		if util.In(tool.FunctionDeclarations[0].Name, allowTools) {
			toolConfig.Tools = append(toolConfig.Tools, tool)
		}
	}
	chat, err := c.client.Chats.Create(ctx, model, toolConfig, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var resp *genai.GenerateContentResponse
	if err := utils.DoBackoff("SendMessage", func() error {
		parts := []genai.Part{{Text: prompt}}
		if err := rl.Wait(ctx, model, c.client, chat.History(false), parts); err != nil {
			return skerr.Wrap(err)
		}
		resp, err = chat.SendMessage(ctx, parts...)
		return err
	}); err != nil {
		return nil, skerr.Wrap(err)
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
				return nil, skerr.Wrapf(err, "tool call %s failed", fc.Name)
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
		if err := utils.DoBackoff("SendMessage", func() error {
			if err := rl.Wait(ctx, model, c.client, chat.History(false), toolResponses); err != nil {
				return skerr.Wrap(err)
			}
			resp, err = chat.SendMessage(ctx, toolResponses...)
			return err
		}); err != nil {
			return nil, skerr.Wrap(err)
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
		if err := rl.Wait(ctx, model, c.client, history, nil); err != nil {
			return skerr.Wrap(err)
		}
		resp, err = c.client.Models.GenerateContent(ctx, model, history, finalConfig)
		return err
	}); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Only the first part of the first candidate seems to be relevant.
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
		r := bytes.NewReader([]byte(resp.Candidates[0].Content.Parts[0].Text))
		debug.Result = resp.Candidates[0].Content.Parts[0].Text
		return debug, skerr.Wrap(json.NewDecoder(r).Decode(result))
	}
	return nil, skerr.Fmt("no output generated")
}

func (c *clientImpl) uploadDebugInfo(ctx context.Context, debug *DebugInfo, parts ...string) {
	if c.gcsBucketDebug == "" {
		return
	}
	object := path.Join(parts...)
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
	Prompt    string               `json:"prompt"`
	Model     string               `json:"model"`
	ToolCalls []DebugInfo_ToolCall `json:"toolCalls"`
	Result    string               `json:"result"`
}

type DebugInfo_ToolCall struct {
	Tool   string                 `json:"tool"`
	Args   map[string]interface{} `json:"args"`
	Result string                 `json:"result"`
}
