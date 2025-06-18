package main

import (
	"bufio"
	"context"
	"flag"
	"os"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/mcp/client/go/mcpClient"
)

func main() {
	server := flag.String("server", "", "URL to the SSE MCP server")
	flag.Parse()
	sklog.Infof("Starting MCP client")
	ctx := context.Background()
	if *server == "" {
		sklog.Fatalf("Server param is required.")
	}
	hadesClient, err := mcpClient.NewMCPClient(ctx, *server)
	if err != nil {
		sklog.Fatal(err)
	}

	chatMgr, err := mcpClient.NewChatManager(ctx, hadesClient)
	if err != nil {
		sklog.Fatal(err)
	}
	chat, err := chatMgr.StartChat(ctx)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Info("Enter prompt: ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := scanner.Text()
		res, err := chatMgr.SendChatMessage(ctx, chat, msg)
		if err != nil {
			sklog.Fatal(err)
		}

		sklog.Infof("Received response: %s", res)
		sklog.Info("Enter prompt: ")
	}
}
