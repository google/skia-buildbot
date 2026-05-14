package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/mcp/services/skia/format"
	"golang.org/x/oauth2/google"
	"google.golang.org/genai"
)

type MCPClient struct {
	client     *client.Client
	httpClient *http.Client
	mcpServer  string
	mtx        sync.RWMutex
	tools      []*genai.Tool

	initializing bool
	// initCount is used in testing to verify that we re-initialize the
	// connection when it is broken.
	initCount int
}

func NewMCPClient(ctx context.Context, mcpServer string) (*MCPClient, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	httpClient := httputils.ClientConfig{}.WithTokenSource(ts).Client()
	return NewMCPClientWithClient(ctx, mcpServer, httpClient)
}

func NewMCPClientWithClient(ctx context.Context, mcpServer string, httpClient *http.Client) (*MCPClient, error) {
	rv := &MCPClient{
		httpClient: httpClient,
		mcpServer:  mcpServer,
	}
	if err := rv.init(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// init (re)initializes the underlying MCP client.
func (c *MCPClient) init(ctx context.Context) error {
	metrics2.GetCounter("autogardener_reinit_count")
	sklog.Infof("initializing MCP server connection")

	mcpClient, err := client.NewSSEMCPClient(c.mcpServer, transport.WithHTTPClient(c.httpClient))
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := mcpClient.Start(ctx); err != nil {
		return skerr.Wrapf(err, "failed to start MCP client")
	}
	if _, err := mcpClient.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		},
	}); err != nil {
		return skerr.Wrapf(err, "failed to initialize MCP client")
	}

	tools, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return skerr.Wrapf(err, "failed to list MCP server tools")
	}

	genAiTools := []*genai.Tool{}
	for _, tool := range tools.Tools {
		funcDeclaration := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  convertSchema(tool.InputSchema),
		}
		genAiTools = append(genAiTools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				funcDeclaration,
			},
		})
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	oldClient := c.client
	c.client = mcpClient
	c.tools = genAiTools
	c.initializing = false
	c.initCount++
	if oldClient != nil {
		_ = oldClient.Close()
	}
	sklog.Infof("Init complete")
	return nil
}

func (c *MCPClient) maybeReInit(ctx context.Context) {
	c.mtx.Lock()
	if c.initializing {
		c.mtx.Unlock()
		return
	}
	c.initializing = true
	c.mtx.Unlock()

	if err := c.init(ctx); err != nil {
		sklog.Errorf("Failed to re-initialize client: %s", err)
		c.mtx.Lock()
		c.initializing = false
		c.mtx.Unlock()
	}
}

func (c *MCPClient) Tools() []*genai.Tool {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	return c.tools
}

func convertSchema(s mcp.ToolInputSchema) *genai.Schema {
	res := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: make(map[string]*genai.Schema),
		Required:   s.Required,
	}
	for name, prop := range s.Properties {
		res.Properties[name] = convertProperty(prop)
	}
	return res
}

func convertProperty(prop any) *genai.Schema {
	m, ok := prop.(map[string]any)
	if !ok {
		return nil
	}
	res := &genai.Schema{}
	if t, ok := m["type"].(string); ok {
		res.Type = genai.Type(strings.ToUpper(t))
	}
	if d, ok := m["description"].(string); ok {
		res.Description = d
	}
	if res.Type == genai.TypeObject {
		if props, ok := m["properties"].(map[string]any); ok {
			res.Properties = make(map[string]*genai.Schema)
			for k, v := range props {
				res.Properties[k] = convertProperty(v)
			}
		}
	}
	return res
}

func (c *MCPClient) callTool(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	defer metrics2.FuncTimer().Stop()
	var res *mcp.CallToolResult
	err := doBackoff(toolName, func() error {
		c.mtx.RLock()
		defer c.mtx.RUnlock()

		// Use a timeout to prevent permanent hangs.
		tCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		var err error
		res, err = c.client.CallTool(tCtx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      toolName,
				Arguments: args,
			},
		})
		if err != nil {
			if strings.Contains(err.Error(), "Invalid session ID") {
				// Something has gone wrong between client and server. Try re-
				// initializing the connection.
				go c.maybeReInit(ctx)
				return err
			}

			// Any error from the MCP server itself is included in its JSON
			// response. If we failed to parse the JSON, then we failed to
			// communicate with the server in some way. We'll retry in that
			// case.
			if strings.Contains(err.Error(), "unexpected end of JSON input") {
				return err
			} else {
				return backoff.Permanent(err)
			}
		}
		return nil
	})
	return res, skerr.Wrap(err)
}

func (c *MCPClient) callToolJSON(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error {
	args[format.ArgFormat] = format.FormatJSON
	res, err := c.callTool(ctx, toolName, args)
	if err != nil {
		return skerr.Wrap(err)
	}
	var textContent strings.Builder
	for _, content := range res.Content {
		if tc, ok := content.(mcp.TextContent); ok {
			textContent.WriteString(tc.Text)
		} else {
			textContent.WriteString(fmt.Sprintf("%v", content))
		}
		textContent.WriteString("\n")
	}
	if res.IsError {
		return skerr.Fmt("tool reported an error: %s", textContent.String())
	}
	return skerr.Wrap(json.NewDecoder(bytes.NewReader([]byte(textContent.String()))).Decode(result))
}
