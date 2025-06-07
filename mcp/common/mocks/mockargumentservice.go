package mocks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/mcp/common"
)

// This file contains mock implementations tailored for testing the ArgumentType
// of a service's ToolArgument.

// mockArgumentService helps test the argument type switch in createMcpSSEServer.
type MockArgumentService struct {
	ArgTypeToTest common.ToolArgumentType
	InitError     error
	CustomTools   []common.Tool
}

func (m *MockArgumentService) Init(serviceArgs string) error {
	return m.InitError
}

func (m *MockArgumentService) GetTools() []common.Tool {
	if m.CustomTools != nil {
		return m.CustomTools
	}
	toolArg := common.ToolArgument{
		Name:         "testArg",
		Description:  "A test argument.",
		Required:     false,
		ArgumentType: m.ArgTypeToTest,
	}
	if m.ArgTypeToTest == common.ArrayArgument {
		// Provide a default schema for ArrayArgument to pass the check in server.go
		toolArg.ArraySchema = map[string]any{"type": "string"}
	}
	return []common.Tool{{
		Name:        "testTool",
		Description: "A tool for testing argument types.",
		Arguments:   []common.ToolArgument{toolArg},
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return nil, nil // Handler not important for this test
		},
	},
	}
}
