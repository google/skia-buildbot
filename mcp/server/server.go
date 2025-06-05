package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/go/urfavecli"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/helloworld"
)

type mcpservice string

const (
	HelloWorld mcpservice = "helloworld"
)

// mcpFlags provides a struct to hold data required by mcp services provided
// via cmdline arguments.
type mcpFlags struct {
	// Name of the service.
	ServiceName string

	// Arguments to pass on to the service.
	ServiceArgs string
}

// AsCliFlags provides a list of the flags supported by the mcpserver cmd.
func (flags *mcpFlags) AsCliFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Destination: &flags.ServiceName,
			Name:        "service",
			Value:       "helloworld",
			Usage:       "The name of the service to run.",
		},
		&cli.StringFlag{
			Destination: &flags.ServiceArgs,
			Name:        "args",
			Value:       "",
			Usage:       "The arguments for the service.",
		},
	}
}

// Cmd execution entry point.
func main() {
	var mcpFlags mcpFlags
	cli.MarkdownDocTemplate = urfavecli.MarkdownDocTemplate

	cliApp := &cli.App{
		Name:  "mcpserver",
		Usage: "Command line tool that runs the MCP service.",
		Before: func(c *cli.Context) error {
			// Log to stdout.
			sklogimpl.SetLogger(stdlogging.New(os.Stdout))

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:        "run",
				Usage:       "mcpserver run --service=helloworld",
				Description: "Runs the process that hosts the mcp service.",
				Flags:       (&mcpFlags).AsCliFlags(),
				Action: func(c *cli.Context) error {
					urfavecli.LogFlags(c)
					sseServer, err := createMcpSSEServer(&mcpFlags)
					if err != nil {
						return err
					}
					if err := sseServer.Start(":8080"); err != nil {
						sklog.Fatalf("Error: %v", err)
					}
					return nil
				},
			},
		},
	}

	err := cliApp.Run(os.Args)
	if err != nil {
		fmt.Printf("\nError: %s\n", err.Error())
		os.Exit(2)
	}
}

// createMcpServer creates a new server that hosts the service based on the
// information in the mcpFlags.
func createMcpSSEServer(mcpFlags *mcpFlags) (*server.SSEServer, error) {
	var service common.McpService
	switch mcpFlags.ServiceName {
	case string(HelloWorld):
		service = helloworld.HelloWorldService{}
	}

	if service == nil {
		return nil, skerr.Fmt("Invalid service %s", mcpFlags.ServiceName)
	}
	s := server.NewMCPServer(
		"Chrome Infra",
		"0.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	tools := service.GetTools()
	for _, tool := range tools {
		options := []mcp.ToolOption{mcp.WithDescription(tool.Description)}
		for _, arg := range tool.Arguments {
			propOptions := []mcp.PropertyOption{}
			if arg.Required {
				propOptions = append(propOptions, mcp.Required())
			}
			propOptions = append(propOptions, mcp.Description(arg.Description))
			options = append(options, mcp.WithString(arg.Name, propOptions...))
		}
		mcpToolSpec := mcp.NewTool(tool.Name, options...)
		s.AddTool(mcpToolSpec, tool.Handler)
	}

	sseServer := server.NewSSEServer(s, server.WithBaseURL("http://localhost:8080"), server.WithKeepAlive(true))

	return sseServer, nil
}
