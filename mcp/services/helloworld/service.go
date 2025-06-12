package helloworld

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/mcp/common"
)

type HelloWorldService struct {
}

// Initialize the service with the provided arguments.
func (s HelloWorldService) Init(serviceArgs string) error {
	return nil
}

// GetTools returns the supported tools by the service.
func (s HelloWorldService) GetTools() []common.Tool {
	return []common.Tool{
		{
			Name:        "sayhello",
			Description: "Says hello to the caller.",
			Arguments: []common.ToolArgument{
				{
					Name:        "name",
					Description: "Name of the user",
					Required:    true,
				},
			},
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				name, err := request.RequireString("name")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				return mcp.NewToolResultText(fmt.Sprintf("Hello, %s!", name)), nil
			},
		},
	}
}

func (s *HelloWorldService) Shutdown() error {
	return nil
}

func (s HelloWorldService) GetResources() []common.Resource {
	return []common.Resource{
		{
			Uri:         "hello://readme",
			Name:        "Hello world readme",
			Description: "Hello world MCP service is a sample service that showcases how to write an MCP service in chrome infra.",
			MimeType:    "text/plain",
			Handler: func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.TextResourceContents{
						URI:      "hello://readme",
						MIMEType: "text/plain",
						Text:     "Hello world MCP service is a sample service that showcases how to write an MCP service in chrome infra.",
					},
				}, nil
			},
		},
	}
}
