// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func runAutoreview(
	ctx context.Context,
	cwd string,
	extraArgs []string,
) (*mcp.CallToolResult, error) {
	cmdArgs := append([]string{
		"run", "--noshow_progress", "--noshow_loading_progress",
		"--show_result=0", "--ui_event_filters=-info,-stdout,-stderr",
		"//cmd/autoreview", "--"}, extraArgs...)
	cmd := exec.CommandContext(ctx, "bazelisk", cmdArgs...)
	cmd.Dir = cwd

	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		output = fmt.Sprintf("Execution Failed: %v\n%s", err, output)
	}

	return mcp.NewToolResultText(output), nil
}

func main() {
	cwd := flag.String("cwd", "", "Working directory for tools")
	flag.Parse()

	s := server.NewMCPServer(
		"buildbot",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	runTool := mcp.NewTool("autoreview_run",
		mcp.WithDescription(
			"Returns AI code review feedback.\n"+
				"Executes `cmd/autoreview` with the given list of "+
				"parameters. Returns a string result of the execution.",
		),
		mcp.WithArray("args",
			mcp.Description("List of arguments to pass to the tool"),
			mcp.Required(),
			mcp.Items(map[string]any{
				"type": "string",
			}),
		),
	)

	s.AddTool(runTool, func(
		ctx context.Context, request mcp.CallToolRequest,
	) (*mcp.CallToolResult, error) {
		return runAutoreview(ctx, *cwd, request.GetStringSlice("args", nil))
	})

	helpTool := mcp.NewTool("autoreview_help",
		mcp.WithDescription(
			"Returns information about supported arguments in the"+
				"`cmd/autoreview` tool.",
		),
	)

	s.AddTool(helpTool, func(
		ctx context.Context, request mcp.CallToolRequest,
	) (*mcp.CallToolResult, error) {
		return runAutoreview(ctx, *cwd, []string{"-help"})
	})

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
