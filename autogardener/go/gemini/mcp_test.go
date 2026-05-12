package gemini

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

func TestMCPClient_ReInitOnInvalidSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// Create a fake MCP server.
	s := server.NewMCPServer("TestServer", "1.0.0", server.WithToolCapabilities(true))
	var callCount int32
	s.AddTool(mcp.NewTool("test_tool"), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			// Simulate the error.
			return nil, fmt.Errorf("Invalid session ID")
		}
		return mcp.NewToolResultText("Success"), nil
	})

	// Wrap the server in an SSE server.
	var sse *server.SSEServer
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sse == nil {
			http.Error(w, "server not ready", http.StatusServiceUnavailable)
			return
		}
		sse.ServeHTTP(w, r)
	}))
	defer httpServer.Close()
	sse = server.NewSSEServer(s, server.WithBaseURL(httpServer.URL))

	// Create the client.
	mcpClient, err := NewMCPClientWithClient(ctx, httpServer.URL+"/sse", httpServer.Client())
	require.NoError(t, err)
	require.Equal(t, 1, mcpClient.initCount)

	// Call the tool. The first call will fail with "Invalid session ID".
	// The callTool method should trigger maybeReInit and doBackoff should retry.
	res, err := mcpClient.callTool(ctx, "test_tool", nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "Success", res.Content[0].(mcp.TextContent).Text)

	// Verify that we saw exactly two calls (one fail, one success).
	require.Equal(t, int32(2), atomic.LoadInt32(&callCount))
	require.Equal(t, 2, mcpClient.initCount)
}

func TestMCPClient_Tools(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	s := server.NewMCPServer("TestServer", "1.0.0", server.WithToolCapabilities(true))
	s.AddTool(mcp.NewTool("test_tool"), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("Success"), nil
	})
	var sse *server.SSEServer
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sse == nil {
			return
		}
		sse.ServeHTTP(w, r)
	}))
	defer ts.Close()
	sse = server.NewSSEServer(s, server.WithBaseURL(ts.URL))

	mcpClient, err := NewMCPClientWithClient(ctx, ts.URL+"/sse", ts.Client())
	require.NoError(t, err)

	tools := mcpClient.Tools()
	require.Len(t, tools, 1)
	require.Equal(t, "test_tool", tools[0].FunctionDeclarations[0].Name)
}
