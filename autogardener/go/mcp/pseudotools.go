package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/autogardener/go/logs"
	"go.skia.org/infra/go/skerr"
	"google.golang.org/genai"
)

type ToolCallFunc func(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error)

type PseudoTool struct {
	callFn ToolCallFunc
	tool   *genai.Tool
}

func (t *PseudoTool) Call(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	return t.callFn(ctx, args)
}

func (t *PseudoTool) Tool() *genai.Tool {
	return t.tool
}

func WrapFuncAsTool(name, description string, parameters *genai.Schema, fn ToolCallFunc) *PseudoTool {
	return &PseudoTool{
		callFn: fn,
		tool: &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Description: description,
					Name:        name,
					Parameters:  parameters,
				},
			},
		},
	}
}

func GetLogLinesTool(logsByStep map[string][]string) *PseudoTool {
	const (
		keyStep       = "step"
		keyStartIndex = "start_index"
		keyEndIndex   = "end_index"
	)
	params := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			keyStep: {
				Description: "Log stream or step ID whose logs should be retrieved. Not used for raw swarming tasks.",
				Type:        genai.TypeString,
			},
			keyStartIndex: {
				Description: "Starting index of log lines to retrieve. Required.",
				Type:        genai.TypeInteger,
			},
			keyEndIndex: {
				Description: "Ending index of log lines to retrieve, exclusive. Required.",
				Type:        genai.TypeInteger,
			},
		},
		Required: []string{keyStep, keyStartIndex, keyEndIndex},
	}
	fn := func(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
		step, err := GetString(keyStep, args, false)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		startIndex, err := GetInt(keyStartIndex, args, true)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		endIndex, err := GetInt(keyEndIndex, args, true)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		lines, ok := logsByStep[step]
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unknown step %q", step)), nil
		}
		if startIndex < 0 {
			startIndex = 0
		}
		if endIndex > len(lines) {
			endIndex = len(lines)
		}
		return mcp.NewToolResultText(logs.RenderLineRange(lines, &logs.LineRange{Start: startIndex, End: endIndex})), nil
	}
	return WrapFuncAsTool("get_step_logs", "Retrieve logs for a step.", params, fn)
}

func GetString(key string, args map[string]interface{}, required bool) (string, error) {
	val, ok := args[key]
	if !ok {
		if required {
			return "", skerr.Fmt("parameter %q is required", key)
		}
		return "", nil
	}
	asStr, ok := val.(string)
	if !ok {
		return "", skerr.Fmt("incorrect type for parameter %q; must be a string", key)
	}
	return asStr, nil
}

func GetInt(key string, args map[string]interface{}, required bool) (int, error) {
	val, ok := args[key]
	if !ok {
		if required {
			return 0, skerr.Fmt("parameter %q is required", key)
		}
		return 0, nil
	}
	asInt, ok := val.(int)
	if !ok {
		return 0, skerr.Fmt("incorrect type for parameter %q; must be an integer", key)
	}
	return asInt, nil
}

type mcpClientWithPseudoTools struct {
	wrapped     MCPClient
	pseudoTools map[string]*PseudoTool
	tools       []*genai.Tool
}

func (c *mcpClientWithPseudoTools) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if pseudoTool, ok := c.pseudoTools[toolName]; ok {
		return pseudoTool.Call(ctx, args)
	}
	return c.wrapped.CallTool(ctx, toolName, args)
}

func (c *mcpClientWithPseudoTools) Tools() []*genai.Tool {
	return c.tools
}

var _ MCPClient = &mcpClientWithPseudoTools{}

func MCPClientWithPseudoTools(wrapped MCPClient, pseudoTools []*PseudoTool, allowedTools []string) MCPClient {
	toolFilter := make(map[string]bool, len(allowedTools))
	for _, t := range allowedTools {
		toolFilter[t] = true
	}
	builtIn := wrapped.Tools()
	filteredMap := make(map[string]*genai.Tool, len(allowedTools))
	for _, t := range builtIn {
		name := t.FunctionDeclarations[0].Name
		if toolFilter[name] {
			filteredMap[name] = t
		}
	}
	pseudoToolMap := make(map[string]*PseudoTool, len(pseudoTools))
	for _, t := range pseudoTools {
		name := t.Tool().FunctionDeclarations[0].Name
		if toolFilter[name] {
			pseudoToolMap[name] = t
			filteredMap[name] = t.Tool()
		}
	}
	filtered := make([]*genai.Tool, 0, len(filteredMap))
	for _, t := range filteredMap {
		filtered = append(filtered, t)
	}
	return &mcpClientWithPseudoTools{
		wrapped:     wrapped,
		pseudoTools: pseudoToolMap,
		tools:       filtered,
	}
}
