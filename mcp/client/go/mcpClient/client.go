package mcpClient

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
	"google.golang.org/genai"
)

// MCPClient defines a struct for calling MCP services.
type MCPClient struct {
	serverUrl   string
	client      *client.Client
	toolNameMap map[string]string
}

// NewMCPClient returns a new instance of the MCP client.
func NewMCPClient(ctx context.Context, serverUrl string) (*MCPClient, error) {
	// Attach oauth tokens to all the requests from the client to the MCP servers.
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		sklog.Fatalf("Error creating oauth token source.")
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()

	// Create a new SSE client.
	client, err := client.NewSSEMCPClient(serverUrl, transport.WithHTTPClient(httpClient))
	if err != nil {
		sklog.Errorf("Error creating new SSE client: %v", err)
		return nil, err
	}

	err = client.Start(ctx)
	if err != nil {
		sklog.Errorf("Error starting transport: %v", err)
		return nil, err
	}
	_, err = client.Initialize(ctx, mcp.InitializeRequest{
		Params: struct {
			ProtocolVersion string                 `json:"protocolVersion"`
			Capabilities    mcp.ClientCapabilities `json:"capabilities"`
			ClientInfo      mcp.Implementation     `json:"clientInfo"`
		}{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "HADES-OAuth",
				Version: "0.1.0",
			},
		},
	})
	if err != nil {
		sklog.Errorf("Init error: %v", err)
		return nil, err
	}

	return &MCPClient{
		serverUrl:   serverUrl,
		client:      client,
		toolNameMap: map[string]string{},
	}, nil
}

// ListTools returns a list of tools supported by the MCP server.
func (m *MCPClient) ListTools(ctx context.Context) ([]*genai.Tool, error) {
	tools, err := m.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}

	// Convert the mcp tools list into a format understandable by Gemini.
	genAiTools := []*genai.Tool{}
	for _, tool := range tools.Tools {
		modelToolName := strings.ReplaceAll(tool.Name, " ", "_")
		m.toolNameMap[modelToolName] = tool.Name
		funcDeclaration := &genai.FunctionDeclaration{
			Name:        modelToolName,
			Description: tool.Description,
			Behavior:    genai.BehaviorBlocking,
			Parameters:  &genai.Schema{Type: genai.TypeObject, Properties: map[string]*genai.Schema{}},
		}
		// Apply the input schema.
		for propName, propVal := range tool.InputSchema.Properties {
			propSchema := &genai.Schema{}
			propMap := propVal.(map[string]interface{})
			propSchema.Type = genai.Type(strings.ToUpper(propMap["type"].(string)))
			propSchema.Description = propMap["description"].(string)
			funcDeclaration.Parameters.Required = append(funcDeclaration.Parameters.Required, tool.InputSchema.Required...)
			funcDeclaration.Parameters.Properties[propName] = propSchema
		}
		genAiTools = append(genAiTools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				funcDeclaration,
			},
		})
	}

	return genAiTools, nil
}

// CallTool invokes the tool.
func (m *MCPClient) CallTool(ctx context.Context, modelToolName string, arguments map[string]any) (*mcp.CallToolResult, error) {
	toolName := m.toolNameMap[modelToolName]
	req := mcp.CallToolRequest{
		Request: mcp.Request{
			Method: toolName,
		},
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	}
	return m.client.CallTool(ctx, req)
}
