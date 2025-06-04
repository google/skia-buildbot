package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/httputils"
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
					server, err := createMcpServer(&mcpFlags)
					if err != nil {
						return err
					}
					sklog.Infof("Service listening on %s", server.Addr)
					sklog.Fatal(server.ListenAndServe())
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

// StartMcpServer starts a new server that hosts the service based on the
// information in the mcpFlags.
func createMcpServer(mcpFlags *mcpFlags) (*http.Server, error) {
	var service common.McpService
	switch mcpFlags.ServiceName {
	case string(HelloWorld):
		service = helloworld.HelloWorldService{}
	}

	if service == nil {
		return nil, skerr.Fmt("Invalid service name %s specified.", mcpFlags.ServiceName)
	}

	sklog.Infof("Initializing service %s", mcpFlags.ServiceName)
	err := service.Init(mcpFlags.ServiceArgs)
	if err != nil {
		return nil, err
	}

	router := chi.NewRouter()

	sklog.Infof("Registering service handlers.")
	service.RegisterHandlers(router)
	handler := httputils.LoggingGzipRequestResponse(router)
	http.Handle("/", handler)

	return &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}, nil
}
