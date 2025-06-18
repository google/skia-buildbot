package mcpClient

import (
	"context"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/genai"
)

const model = "gemini-2.5-pro-preview-06-05"

// ChatManager defines a struct to handle chat messaging in the CLI.
type ChatManager struct {
	geminiClient *genai.Client
	mcpClient    *MCPClient
}

// NewChatManager returns a new instance of the chat manager.
func NewChatManager(ctx context.Context, mcpClient *MCPClient) (*ChatManager, error) {
	geminiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		sklog.Errorf("Error creating new gemini client: %v", err)
		return nil, err
	}

	return &ChatManager{
		geminiClient: geminiClient,
		mcpClient:    mcpClient,
	}, nil
}

// StartChat starts a new chat session.
func (c *ChatManager) StartChat(ctx context.Context) (*genai.Chat, error) {
	tools, err := c.mcpClient.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{
		Tools: tools,
	}
	return c.geminiClient.Chats.Create(ctx, model, config, nil)
}

// SendChatMessage sends the provided message to the gemini model.
func (c *ChatManager) SendChatMessage(ctx context.Context, chat *genai.Chat, message string) (string, error) {
	resp, err := chat.SendMessage(ctx, genai.Part{Text: message})
	if err != nil {
		sklog.Errorf("Error sending chat message: %v", err)
		return "", err
	}

	if resp.Candidates[0].FinishReason != genai.FinishReasonStop {
		return "", skerr.Fmt("Response was blocked or did not finish as expected. Reason: %s: %s", resp.PromptFeedback.BlockReason, resp.PromptFeedback.BlockReasonMessage)
	}

	responseStr := resp.Candidates[0].Content.Parts[0].Text

	functionCalls := resp.FunctionCalls()
	if len(functionCalls) > 0 {
		sklog.Infof("Calling tools: %v", functionCalls)

		for _, functionCall := range functionCalls {
			sklog.Infof("Calling %s", functionCall.Name)
			result, err := c.mcpClient.CallTool(ctx, functionCall.Name, functionCall.Args)
			if err != nil {
				sklog.Errorf("Error invoking tool %s: %v", functionCall.Name, err)
			}
			responseStr = result.Content[0].(mcp.TextContent).Text
		}
	}

	return responseStr, err
}
