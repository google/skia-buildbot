package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/mcp/common"
	"google.golang.org/genai"
)

type embeddedService struct {
	srv   common.McpService
	tools []*genai.Tool
}

func NewEmbeddedService(srv common.McpService) *embeddedService {
	genAiTools := []*genai.Tool{}
	for _, tool := range srv.GetTools() {
		funcDeclaration := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  toolArgsToSchema(tool.Arguments),
		}
		genAiTools = append(genAiTools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				funcDeclaration,
			},
		})
	}
	return &embeddedService{
		srv:   srv,
		tools: genAiTools,
	}
}

func (s *embeddedService) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	var tool *common.Tool
	for _, t := range s.srv.GetTools() {
		if t.Name == toolName {
			tool = &t
			break
		}
	}
	if tool == nil {
		return nil, skerr.Fmt("unknown tool %q", toolName)
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
	return tool.Handler(ctx, req)
}

func (s *embeddedService) Tools() []*genai.Tool {
	return s.tools
}

func toolArgsToSchema(args []common.ToolArgument) *genai.Schema {
	res := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: map[string]*genai.Schema{},
		Required:   []string{},
	}
	for _, arg := range args {
		if arg.Required {
			res.Required = append(res.Required, arg.Name)
		}
		s := &genai.Schema{
			Description: arg.Description,
			Enum:        arg.EnumValues,
		}
		switch arg.ArgumentType {
		case common.StringArgument:
			s.Type = genai.TypeString
		case common.BooleanArgument:
			s.Type = genai.TypeBoolean
		case common.NumberArgument:
			s.Type = genai.TypeNumber
		case common.ObjectArgument:
			s.Type = genai.TypeObject
		case common.ArrayArgument:
			s.Type = genai.TypeArray
		}
		res.Properties[arg.Name] = s
	}
	return res
}

// Assert that embeddedService implements MCPClient
var _ MCPClient = &embeddedService{}
