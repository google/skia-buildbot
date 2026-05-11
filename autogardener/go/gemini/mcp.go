package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cenkalti/backoff/v4"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/mcp/services/skia/format"
	"golang.org/x/oauth2/google"
	"google.golang.org/genai"
)

type MCPClient struct {
	client *client.Client
}

func NewMCPClient(ctx context.Context, mcpServer string) (*MCPClient, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	c := httputils.ClientConfig{}.WithTokenSource(ts).Client()
	mcpClient, err := client.NewSSEMCPClient(mcpServer, transport.WithHTTPClient(c))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	if err := mcpClient.Start(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}
	_, err = mcpClient.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		},
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &MCPClient{client: mcpClient}, nil
}

func (c *MCPClient) ListTools(ctx context.Context) ([]*genai.Tool, error) {
	tools, err := c.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, skerr.Wrap(err)
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

	return genAiTools, nil
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
	var res *mcp.CallToolResult
	err := doBackoff(toolName, func() error {
		var err error
		res, err = c.client.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      toolName,
				Arguments: args,
			},
		})
		if err != nil {
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
