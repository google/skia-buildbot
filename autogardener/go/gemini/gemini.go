package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/mcp/services/skia/task_details"
	"go.skia.org/infra/mcp/services/skia/task_scheduler"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/time/rate"
	"google.golang.org/genai"
)

const maxRequestsPerMinute = 20

// Client provides high-level interactions with Gemini.
type Client struct {
	client    *genai.Client
	lim       *rate.Limiter
	location  string
	model     string
	mcpClient *MCPClient
	project   string
	tools     []*genai.Tool
}

func NewClient(ctx context.Context, project, location, model, apiKey, mcpServer string) (*Client, error) {
	mcpClient, err := NewMCPClient(ctx, mcpServer)
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
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	perSecondLimit := (float64(maxRequestsPerMinute) / float64(time.Minute)) * float64(time.Second)
	return &Client{
		client:    genaiClient,
		lim:       rate.NewLimiter(rate.Limit(perSecondLimit), maxRequestsPerMinute),
		location:  location,
		model:     model,
		mcpClient: mcpClient,
		project:   project,
		tools:     tools,
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
  failures that need investigation.


# Current Task
`

func (c *Client) GetTaskSummary(ctx context.Context, task *ts_types.Task) (*types.TaskSummary, error) {
	// Retrieve the task steps so that the agent doesn't have to.
	var taskSteps task_details.GetTaskStepsResult
	if err := c.mcpClient.callToolJSON(ctx, "get_task_steps", map[string]interface{}{"task_id": task.Id}, &taskSteps); err != nil {
		return nil, skerr.Wrap(err)
	}

	const promptTmpl = generalPromptHeader + `
Deeply investigate the failed task with ID "%s".

**CRITICAL:** This is strictly a log analysis and data extraction workflow. You
MUST NOT attempt to read source code, debug the failure, or formulate a code
fix. ONLY use the tools specified below. DO NOT attempt to retrieve information
about any other tasks.

## Workflow

1. Find the relevant failed step(s). Note that some step failures may be
   expected, for example a step which tests file existence via some command
   that exits with a non-zero code when it does not exist.
2. If the task is a Task Driver or Recipe, retrieve the logs for the failed
   step via the "get_task_driver_step_logs" or "get_recipe_step_logs" tool.
3. Scan the logs and extract a digestible snippet.
   - **WARNING:** Skia tasks (especially "dm" and "nanobench") sometimes
     produce massive amounts of log spam (e.g., graphics API warnings, compiler
     warnings) that are non-fatal red herrings. Do not get distracted by them.
   - Do **not** assume a task timed out just because it ran for a long time or
     generated a lot of spam.
   - To find the _actual_ cause of failure, always check the **end** of the logs
     first. Specifically, look for a "Failures:" section or explicit test
     failure messages right before the step exits.
   - Be sure to include the entire error message or stack trace, unless it is
     excessively long (>100 lines).
4. Analyze the error(s) and present a report to the user.
   - Be concise. Only include relevant information and avoid repeating or
     restating your conclusion. The error message will be included in the
     "ErrorMessage" section of the response, so do not include it in your
     analysis.
   - If you suspect a problem with the machine which ran the task, include the
     bot ID in your analysis.

## Task Details

%s

## Task Steps

%s
`
	prompt := fmt.Sprintf(promptTmpl, task.Id, task_scheduler.TaskWrapper{Task: task}, taskSteps)
	allowedTools := []string{
		"get_task_driver_step_logs",
		"get_recipe_step_logs",
	}
	var res types.TaskSummary
	if err := c.generate(ctx, prompt, allowedTools, &res); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &res, nil
}

func (c *Client) generate(ctx context.Context, prompt string, allowTools []string, result interface{}) error {
	// Gemini doesn't use MCP tools directly. Rather, it returns requests to use
	// tools as part of its response, expecting to see the results of those tool
	// calls in our next message. Therefore, we need to repeatedly send messages
	// in a loop, starting with our initial prompt and continuing to run tools
	// and present their results until Gemini returns its ultimate response.

	config := &genai.GenerateContentConfig{
		ResponseMIMEType:   "application/json",
		ResponseJsonSchema: jsonschema.Reflect(result),
	}
	for _, tool := range c.tools {
		if util.In(tool.FunctionDeclarations[0].Name, allowTools) {
			config.Tools = append(config.Tools, tool)
		}
	}

	chat, err := c.client.Chats.Create(ctx, c.model, config, nil)
	if err != nil {
		return skerr.Wrap(err)
	}
	var resp *genai.GenerateContentResponse
	if err := doBackoff("SendMessage", func() error {
		if err := c.lim.Wait(ctx); err != nil {
			return skerr.Wrap(err)
		}
		resp, err = chat.SendMessage(ctx, genai.Part{Text: prompt})
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
			toolRes, err := c.mcpClient.callTool(ctx, fc.Name, fc.Args)
			if err != nil {
				return skerr.Wrapf(err, "tool call %s failed", fc.Name)
			}

			var sb strings.Builder
			for _, content := range toolRes.Content {
				if tc, ok := content.(mcp.TextContent); ok {
					sb.WriteString(tc.Text)
				} else {
					sb.WriteString(fmt.Sprintf("%v", content))
				}
			}
			toolResponses = append(toolResponses, genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name: fc.Name,
					Response: map[string]any{
						"result": sb.String(),
					},
				},
			})
		}
		if err := doBackoff("SendMessage", func() error {
			if err := c.lim.Wait(ctx); err != nil {
				return skerr.Wrap(err)
			}
			resp, err = chat.SendMessage(ctx, toolResponses...)
			return err
		}); err != nil {
			return skerr.Wrap(err)
		}
	}

	// Only the first part of the first candidate seems to be relevant.
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		r := bytes.NewReader([]byte(resp.Candidates[0].Content.Parts[0].Text))
		return skerr.Wrap(json.NewDecoder(r).Decode(result))
	}
	return skerr.Fmt("no output generated")
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
