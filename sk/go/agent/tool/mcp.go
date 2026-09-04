package tool

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/urfave/cli/v2"
	sk_mcp "go.skia.org/infra/autogardener/go/mcp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/mcp/services/skia"
	"google.golang.org/genai"
)

const (
	publicFirestoreInstance   = "production"
	publicTdBtProject         = "skia-public"
	publicTdBtInstance        = "staging"
	publicSwarmingServer      = "chromium-swarm.appspot.com"
	internalFirestoreInstance = "internal"
	internalTdBtProject       = "google.com:skia-corp"
	internalTdBtInstance      = "internal"
	internalSwarmingServer    = "chrome-swarming.appspot.com"
)

func createCommandsForMCPTools(ctx context.Context) ([]*cli.Command, error) {
	mcpClient, err := initMCP(ctx, false /* Tools are the same for public/internal */)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	internalFlag := &cli.BoolFlag{
		Name:  "internal",
		Usage: "If set, use internal data. The user must have read permissions.",
	}
	var commands []*cli.Command
	for _, tool := range mcpClient.Tools() {
		decl := tool.FunctionDeclarations[0]
		toolName := decl.Name
		flags := append(getFlagsFromSchema(decl.Parameters), defaultFlags...)
		flags = append(flags, internalFlag)
		cmd := &cli.Command{
			Name:        toolName,
			Usage:       decl.Description,
			Description: decl.Description,
			Flags:       flags,
			Action: func(c *cli.Context) error {
				return callMCPTool(c, toolName, c.Bool(internalFlag.Name))
			},
		}
		commands = append(commands, cmd)
	}

	return commands, nil
}

var defaultFlags = []cli.Flag{
	&cli.StringFlag{
		Name:  "output-file",
		Usage: "Write output to this file instead of stdout.",
	},
}

func getFlagsFromSchema(schema *genai.Schema) []cli.Flag {
	requiredMap := make(map[string]bool, len(schema.Required))
	for _, required := range schema.Required {
		requiredMap[required] = true
	}

	var flags []cli.Flag
	for name, prop := range schema.Properties {
		description := prop.Description
		required := requiredMap[name]

		switch prop.Type {
		case genai.TypeBoolean:
			flags = append(flags, &cli.BoolFlag{
				Name:  name,
				Usage: description,
			})
		case genai.TypeNumber, genai.TypeInteger:
			flags = append(flags, &cli.IntFlag{
				Name:     name,
				Usage:    description,
				Required: required,
			})
		default:
			// Just default to strings.
			flags = append(flags, &cli.StringFlag{
				Name:     name,
				Usage:    description,
				Required: required,
			})
		}
	}
	return flags
}

func callMCPTool(ctx *cli.Context, toolName string, internal bool) error {
	mcpClient, err := initMCP(ctx.Context, internal)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Don't pass flags we added to the MCP tool.
	isDefaultFlag := map[string]bool{}
	for _, defaultFlag := range defaultFlags {
		for _, defaultFlagName := range defaultFlag.Names() {
			isDefaultFlag[defaultFlagName] = true
		}
	}

	args := make(map[string]interface{})
	for _, flagName := range ctx.FlagNames() {
		if ctx.IsSet(flagName) && !isDefaultFlag[flagName] {
			args[flagName] = ctx.Value(flagName)
		}
	}

	res, err := mcpClient.CallTool(ctx.Context, toolName, args)
	if err != nil {
		return skerr.Wrap(err)
	}
	var textContent strings.Builder
	for _, content := range res.Content {
		textContent.WriteString(content.(mcp.TextContent).Text)
		textContent.WriteString("\n")
	}
	if res.IsError {
		return fmt.Errorf("tool reported an error: %s", textContent.String())
	} else if outputFile := ctx.String("output-file"); outputFile != "" {
		return util.WithWriteFile(outputFile, func(w io.Writer) error {
			_, err := fmt.Fprintln(w, textContent.String())
			return skerr.Wrap(err)
		})
	} else {
		fmt.Println(textContent.String())
	}
	return nil
}

func initMCP(ctx context.Context, internal bool) (sk_mcp.MCPClient, error) {
	srv := &skia.SkiaService{}
	cleanup.AtExit(func() {
		if err := srv.Shutdown(); err != nil {
			sklog.Errorf("Error performing shutdown for service: %v", err)
		}
	})
	firestoreInstance := publicFirestoreInstance
	tdBtProject := publicTdBtProject
	tdBtInstance := publicTdBtInstance
	swarmingServer := publicSwarmingServer
	if internal {
		firestoreInstance = internalFirestoreInstance
		tdBtProject = internalTdBtProject
		tdBtInstance = internalTdBtInstance
		swarmingServer = internalSwarmingServer
	}
	mcpArgs := fmt.Sprintf(
		"--firestore_instance=%s --bigtable_project=%s --bigtable_instance=%s --swarming_server=%s",
		firestoreInstance,
		tdBtProject,
		tdBtInstance,
		swarmingServer,
	)
	if err := srv.Init(mcpArgs); err != nil {
		return nil, skerr.Wrap(err)
	}
	mcpClient := sk_mcp.NewEmbeddedService(srv)
	return mcpClient, nil
}
