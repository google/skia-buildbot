package pinpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/pinpoint/go/backends"
)

// reusable identifiers for the flags.
const (
	BaseGitHashFlagName       = "base_git_hash"
	BenchmarkFlagName         = "benchmark"
	BotConfigurationFlagName  = "bot_configuration"
	ExperimentGitHashFlagName = "experiment_git_hash"
	IterationFlagName         = "iteration"
	StoryFlagName             = "story"
	TargetNewPinpoint         = "target_new_pinpoint"
	UserFlagName              = "user"
	BugIDFlagName             = "bug_id"
	ProjectFlagName           = "project"
	BasePatchFlagName         = "base_patch"
	ExperimentPatchFlagName   = "experiment_patch"

	// bisect specific flags
	BaseRevisionFlagName       = "base_revision"
	ExperimentRevisionFlagName = "experiment_revision"
)

func BenchmarkArgument(required bool) common.ToolArgument {
	return common.ToolArgument{
		Name:         BenchmarkFlagName,
		Description:  benchmarkDescription,
		Required:     required,
		ArgumentType: common.StringArgument,
	}
}

// arguments returns the list of arguments pertaining to both
// Pinpoint try and bisect jobs.
// isTryJob is a bool field, where if try job is true, git hash fields are required.
func arguments(isTryJob bool) []common.ToolArgument {
	// TODO(jeffyoon@) pass the user
	return []common.ToolArgument{
		{
			Name:         BaseGitHashFlagName,
			Description:  baseGitHashDescription,
			Required:     isTryJob,
			ArgumentType: common.StringArgument,
		},
		// Reused in Chromeperf tooling
		BenchmarkArgument(true),
		{
			Name:         StoryFlagName,
			Description:  storyDescription,
			Required:     true,
			ArgumentType: common.StringArgument,
		},
		{
			Name:         BotConfigurationFlagName,
			Description:  botConfigurationDescription,
			Required:     true,
			ArgumentType: common.StringArgument,
		},
		{
			Name:         ExperimentGitHashFlagName,
			Description:  experimentGitHashDescription,
			Required:     isTryJob,
			ArgumentType: common.StringArgument,
		},
		{
			Name:         IterationFlagName,
			Description:  iterationDescription,
			Required:     false,
			ArgumentType: common.NumberArgument,
		},
		{
			Name:         TargetNewPinpoint,
			Description:  newPinpointDescription,
			Required:     false,
			ArgumentType: common.BooleanArgument,
		},
	}
}

// arguments specific to bisect, to support bisecting an anomaly for a culprit.
func bisectArguments() []common.ToolArgument {
	// TODO(jeffyoon@) comparison magnitude
	return append(arguments(false), []common.ToolArgument{
		{
			Name:         BaseRevisionFlagName,
			Description:  baseRevisionDescription,
			Required:     false,
			ArgumentType: common.StringArgument,
		},
		{
			Name:         ExperimentRevisionFlagName,
			Description:  experimentRevisionDescription,
			Required:     false,
			ArgumentType: common.StringArgument,
		},
	}...)
}

func pairwiseArguments() []common.ToolArgument {
	return append(arguments(true), []common.ToolArgument{
		{
			Name:         UserFlagName,
			Description:  userDescription,
			Required:     true,
			ArgumentType: common.StringArgument,
		},
		{
			Name:         BugIDFlagName,
			Description:  bugIDDescription,
			Required:     false,
			ArgumentType: common.StringArgument,
		},
		{
			Name:         ProjectFlagName,
			Description:  projectDescription,
			Required:     false,
			ArgumentType: common.StringArgument,
		},
		{
			Name:         BasePatchFlagName,
			Description:  basePatchDescription,
			Required:     false,
			ArgumentType: common.StringArgument,
		},
		{
			Name:         ExperimentPatchFlagName,
			Description:  experimentPatchDescription,
			Required:     false,
			ArgumentType: common.StringArgument,
		},
	}...)
}

// GetTools returns tools supported by Pinpoint.
func GetTools(httpClient *http.Client, crrevClient *backends.CrrevClientImpl) []common.Tool {
	return []common.Tool{
		{
			Name:        BisectCommandName,
			Description: bisectionDescription,
			Arguments:   bisectArguments(),
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				c := NewPinpointClient(request.GetArguments())

				resp, err := c.Bisect(ctx, httpClient, crrevClient)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				var b bytes.Buffer
				err = json.NewEncoder(&b).Encode(resp)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}
				return mcp.NewToolResultText(b.String()), nil
			},
		},
		{
			Name:        PairwiseCommandName,
			Description: pairwiseDescription,
			Arguments:   pairwiseArguments(),
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				c := NewPinpointClient(request.GetArguments())

				resp, err := c.TryJob(ctx, httpClient)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				var b bytes.Buffer
				err = json.NewEncoder(&b).Encode(resp)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}
				return mcp.NewToolResultText(b.String()), nil
			},
		},
	}
}
