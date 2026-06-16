package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestGetString(t *testing.T) {
	args := map[string]interface{}{
		"key_str": "value",
		"key_int": 123,
	}

	// Success
	val, err := GetString("key_str", args, true)
	require.NoError(t, err)
	require.Equal(t, "value", val)

	// Missing, optional
	val, err = GetString("missing", args, false)
	require.NoError(t, err)
	require.Equal(t, "", val)

	// Missing, required
	val, err = GetString("missing", args, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), `parameter "missing" is required`)

	// Wrong type
	val, err = GetString("key_int", args, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), `incorrect type for parameter "key_int"; must be a string`)
}

func TestGetInt(t *testing.T) {
	args := map[string]interface{}{
		"key_int": 123,
		"key_str": "value",
	}

	// Success
	val, err := GetInt("key_int", args, true)
	require.NoError(t, err)
	require.Equal(t, 123, val)

	// Missing, optional
	val, err = GetInt("missing", args, false)
	require.NoError(t, err)
	require.Equal(t, 0, val)

	// Missing, required
	val, err = GetInt("missing", args, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), `parameter "missing" is required`)

	// Wrong type
	val, err = GetInt("key_str", args, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), `incorrect type for parameter "key_str"; must be an integer`)
}

func TestGetLogLinesTool(t *testing.T) {
	ctx := context.Background()
	logs := map[string][]string{
		"step1": {
			"line 1",
			"line 2",
			"line 3",
			"line 4",
			"line 5",
		},
		"": {
			"raw log 1",
			"raw log 2",
		},
	}

	tool := GetLogLinesTool(logs)
	require.Equal(t, "get_step_logs", tool.Tool().FunctionDeclarations[0].Name)

	getText := func(res *mcp.CallToolResult) string {
		require.NotNil(t, res)
		require.NotEmpty(t, res.Content)
		tc, ok := res.Content[0].(mcp.TextContent)
		require.True(t, ok)
		return tc.Text
	}

	// Success: step1
	res, err := tool.Call(ctx, map[string]interface{}{
		"step":        "step1",
		"start_index": 1,
		"end_index":   3,
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	text := getText(res)
	require.Contains(t, text, "line 2")
	require.Contains(t, text, "line 3")
	require.NotContains(t, text, "line 1")
	require.NotContains(t, text, "line 4")

	// Success: empty step name (raw logs)
	res, err = tool.Call(ctx, map[string]interface{}{
		"step":        "",
		"start_index": 0,
		"end_index":   100, // Should be clamped to 2
	})
	require.NoError(t, err)
	require.False(t, res.IsError)
	text = getText(res)
	require.Contains(t, text, "raw log 1")
	require.Contains(t, text, "raw log 2")

	// Error: unknown step
	res, err = tool.Call(ctx, map[string]interface{}{
		"step":        "unknown",
		"start_index": 0,
		"end_index":   10,
	})
	require.NoError(t, err) // Tool returns result with IsError=true
	require.True(t, res.IsError)
	text = getText(res)
	require.Contains(t, text, `unknown step "unknown"`)

	// Error: missing required args
	res, err = tool.Call(ctx, map[string]interface{}{
		"step": "step1",
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	text = getText(res)
	require.Contains(t, text, "parameter \"start_index\" is required")
}

func TestWrapFuncAsTool(t *testing.T) {
	called := false
	fn := func(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("success"), nil
	}

	params := &genai.Schema{Type: genai.TypeObject}
	tool := WrapFuncAsTool("test_tool", "test desc", params, fn)

	require.NotNil(t, tool)
	require.Equal(t, "test_tool", tool.Tool().FunctionDeclarations[0].Name)
	require.Equal(t, "test desc", tool.Tool().FunctionDeclarations[0].Description)

	res, err := tool.Call(context.Background(), nil)
	require.NoError(t, err)
	require.True(t, called)
	tc, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok)
	require.Equal(t, "success", tc.Text)
}
