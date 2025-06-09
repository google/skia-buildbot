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
	"go.skia.org/infra/mcp/services/perf"
)

type mcpservice string

const (
	HelloWorld mcpservice = "helloworld"
	Perf       mcpservice = "perf"
)

// serviceFactory defines a function that creates a McpService instance.
type serviceFactory func() common.McpService

// serviceRegistry holds the mapping from service names to their factory functions.
// This allows for easier testing by registering mock services.
var serviceRegistry = map[string]serviceFactory{
	string(HelloWorld): func() common.McpService { return helloworld.HelloWorldService{} },
	string(Perf):       func() common.McpService { return &perf.PerfService{} },
}

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
	factory, ok := serviceRegistry[mcpFlags.ServiceName]
	if !ok {
		return nil, skerr.Fmt("Unknown service: %s", mcpFlags.ServiceName)
	}
	service := factory()

	err := service.Init(mcpFlags.ServiceArgs)
	if err != nil {
		return nil, err
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
			switch arg.ArgumentType {
			case common.StringArgument:
				options = append(options, mcp.WithString(arg.Name, propOptions...))
			case common.BooleanArgument:
				options = append(options, mcp.WithBoolean(arg.Name, propOptions...))
			case common.NumberArgument:
				options = append(options, mcp.WithNumber(arg.Name, propOptions...))
			case common.ObjectArgument:
				options = append(options, mcp.WithObject(arg.Name, propOptions...))
			case common.ArrayArgument:
				options = append(options, mcp.WithArray(arg.Name, propOptions...))
			default:
				return nil, skerr.Fmt("Invalid argument type %v", arg.ArgumentType)
			}
		}
		mcpToolSpec := mcp.NewTool(tool.Name, options...)
		s.AddTool(mcpToolSpec, tool.Handler)
	}

	sseServer := server.NewSSEServer(s, server.WithBaseURL("http://localhost:8080"), server.WithKeepAlive(true))

	return sseServer, nil
}
