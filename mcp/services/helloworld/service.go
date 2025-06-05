package helloworld

import (
	"fmt"

	"github.com/invopop/jsonschema"
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
			InputSchema: jsonschema.Schema{
				Type: "string",
			},
		},
	}
}

// CallTool invokes the specified tool.
func (s HelloWorldService) CallTool(request common.CallToolRequest) common.CallToolResponse {
	return common.CallToolResponse{
		Result: fmt.Sprintf("Hello %s", request.Arguments["user"]),
	}
}
