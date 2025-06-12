package pinpoint

import (
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

	// bisect specific flags
	BaseRevisionFlagName       = "base_revision"
	ExperimentRevisionFlagName = "experiment_revision"
)

func BenchmarkArgument(required bool) common.ToolArgument {
	return common.ToolArgument{
		Name: BenchmarkFlagName,
		Description: "The benchmark of interest to run. " +
			"For example, press benchmarks commonly refer to one of \"speedometer3.crossbench\", \"jetstream2.crossbench\" or \"motionmark1.3.crossbench\". " +
			"One job (bisect or try) can be associated with only one benchmark (and one story). " +
			"For the full list of supported benchmarks, use the command \"perf list benchmarks\". " +
			"This is a required field.",
		Required:     required,
		ArgumentType: common.StringArgument,
	}
}

// arguments returns the list of arguments pertaining to both
// Pinpoint try and bisect jobs.
// isTryJob is a bool field, where if try job is true, git hash fields are required.
func arguments(isTryJob bool) []common.ToolArgument {
	return []common.ToolArgument{
		{
			Name: BaseGitHashFlagName,
			Description: "A git hash SHA (either full or short form) of the first commit required for a Pinpoint job. " +
				"In the context of bisection, this is the starting commit of the range you'd like to bisect. " +
				"For try jobs (or A/B experiments), this is the base (also refered to the control, or A) of that A/B comparison. " +
				"Pinpoint currently only accepts git hashes based on the Chromium repository (chromium/src) for the base. " +
				"For example, 2d98fb0e9f9f0fdb24c78d8fd29a8a0b029852ba or 2d98fb0 for full or short form respectively from " +
				"https://chromium.googlesource.com/chromium/src/. This is a required field.",
			Required:     isTryJob,
			ArgumentType: common.StringArgument,
		},
		// Reused in Chromeperf tooling
		BenchmarkArgument(true),
		{
			Name: StoryFlagName,
			Description: "A story refers to a set of actions run by the benchmark. In other words, a subset of tests run within the benchmark. " +
				"For example, for the press benchmarks (\"speedometer3.crossbench\", \"jetstream2.cro왜ssbench\" or \"motionmark1.3.crossbench\") " +
				"the story is \"default\". This is a required field",
			Required:     true,
			ArgumentType: common.StringArgument,
		},
		{
			Name: BotConfigurationFlagName,
			Description: "The bot configuration refers to a tester (or test builder) on the Perf waterfall to run the benchmarks on.  " +
				"A tester maps 1:1 with a specific device type. For example, the tester \"mac-m3-pro-perf\" refers " +
				"to testing a benchmark on Mac M3 Pro devices.",
			Required:     true,
			ArgumentType: common.StringArgument,
		},
		{
			Name: ExperimentGitHashFlagName,
			Description: "The git hash SHA (either full or short form) of the other commit for a Pinpoint job. " +
				"In the context of bisection, this is the ending commit of the range you'd like to bisect. " +
				"For a try job (or an A/B test), this is the experiment (also referred to as the treatment, or B). " +
				"Pinpoint currently only accepts git hashes based on the Chromium repository (chromium/src) for the base. " +
				"For example, 2d98fb0e9f9f0fdb24c78d8fd29a8a0b029852ba or 2d98fb0 for full or short form respectively from " +
				"https://chromium.googlesource.com/chromium/src/. This is a required field.",
			Required:     isTryJob,
			ArgumentType: common.StringArgument,
		},
		{
			Name: IterationFlagName,
			Description: "The number of iterations to run the benchmark. " +
				"Higher iterations usually yield more granular benchmark results, but at the tradeoff of consuming " +
				"(and holding onto) additional resources. This value defaults to 10. The recommmended value for try jobs " +
				"is 20. A value over 50 will be rejected. This is an optional field.",
			Required:     false,
			ArgumentType: common.NumberArgument,
		},
		{
			Name: TargetNewPinpoint,
			Description: "Turn on this flag to target the new Pinpoint implementation. New Pinpoint refers to the implementation in the " +
				"buildbot repository. This defaults to false.",
			Required:     false,
			ArgumentType: common.BooleanArgument,
		},
	}
}

// arguments specific to bisect, to support bisecting an anomaly for a culprit.
func bisectArguments() []common.ToolArgument {
	return append(arguments(false), []common.ToolArgument{
		{
			Name: BaseRevisionFlagName,
			Description: "The revision number, or the commit position, for the base of the Pinpoint job. " +
				"This is converted to a git hash using crrev.com/. Because this is the base, in a try job (or A/B test) " +
				"this value is used as the base (or the control, or A) for that experiment. For a bisect job, it defines the starting point " +
				"to run the bisection for when searching for a culprit. If the base git hash is already provided, this value will be ignored. " +
				"One of base revision flag or base git hash is required to run a try or bisect job.",
			Required:     false,
			ArgumentType: common.StringArgument,
		},
		{
			Name: ExperimentRevisionFlagName,
			Description: "The revision number, or the commit position, for the experiment of the Pinpoint job. " +
				"This is converted to a git hash using crrev.com/. Because this is the base, in a try job (or A/B test) " +
				"this value is used as the experiment (or B) for that experiment. For a bisect job, it defines the starting point " +
				"to run the bisection for when searching for a culprit. If the experiment git hash is already provided, this value will be ignored. " +
				"One of experiment revision flag or experiment git hash is required to run a try or bisect job.",
			Required:     false,
			ArgumentType: common.StringArgument,
		},
	}...)
}

// GetTools returns tools supported by Pinpoint.
func GetTools(httpClient *http.Client, crrevClient *backends.CrrevClientImpl) []common.Tool {
	return []common.Tool{
		{
			Name: BisectCommandName,
			Description: "Bisect (bisection) aims to find a culprit for a regression (or anomaly) that was detected for a particular " +
				"benchmark and platform in a particular range of time which is defined by a start (base git hash) and end (experimental git hash). " +
				"It runs a binary search against the commits in the given range, comparing the performance between potential candidates, until it finds the change that caused " +
				"the regression. This will trigger a Pinpoint bisect job, and return both the job id and the url to the Pinpoint job started.",
			Arguments: bisectArguments(),
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				c := NewPinpointClient(request.GetArguments())

				resp, err := c.Bisect(ctx, httpClient, crrevClient)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				b, err := json.Marshal(resp)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}
				return mcp.NewToolResultText(string(b)), nil
			},
		},
		{
			Name: PairwiseCommandName,
			Description: "Try (or try job) is an (A/B test) and will trigger a Pinpoint. This try job compares " +
				"the performance of Chrome on a particular benchmark on a particular platform at two different points " +
				"in time, defined by the base (or control, or A) and the experiment (or the treatment, or B). " +
				"This will trigger a Pinpoint try job, and return both the job id and the url to the Pinpoint job started.",
			Arguments: arguments(true),
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				c := NewPinpointClient(request.GetArguments())

				resp, err := c.TryJob(ctx, httpClient)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}

				b, err := json.Marshal(resp)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), err
				}
				return mcp.NewToolResultText(string(b)), nil
			},
		},
	}
}
