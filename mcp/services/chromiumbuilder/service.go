package chromiumbuilder

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vfs"
	"go.skia.org/infra/mcp/common"
)

// ChromiumBuilderService is an MCP service which is capable of generating CLs
// to add new LUCI builders to chromium/src.
type ChromiumBuilderService struct {
	chromiumPath       string
	depotToolsPath     string
	chromiumCheckout   git.Checkout
	depotToolsCheckout git.Checkout
	// Must be held if the server is currently handling a tool request. We do
	// not want multiple concurrent requests to trample on each other's work.
	// This is not a good long-term solution, particularly since we have no way
	// of signalling to the client that they're essentially in a queue. However,
	// it will suffice for initial work since the service will not be handling
	// many requests.
	handlingToolRequestLock sync.Mutex
	// Set to true if the server is shutting down. No more git/exec operations
	// should be performed in this case
	shuttingDown atomic.Bool
	// Should be locked anytime chromiumCheckout is being used or modified.
	chromiumCheckoutLock sync.Mutex
	// Should be locked anytime depotToolsCheckout is being used or modified.
	depotToolsCheckoutLock sync.Mutex
	// Should be locked when Chromium is actively being fetched.
	chromiumFetchLock sync.Mutex
	// Should be locked when a subprocess is being run that is safe to cancel
	// mid-run without additional cleanup.
	safeCancellableCommandLock sync.Mutex
	// Should be set to to the Process currently being run via exec so it can
	// be cancelled if necessary.
	currentProcess exec.Process
	// Should be locked anytime runningProcess is being used or modified.
	currentProcessLock sync.Mutex
}

// Init initializes the service with the provided arguments. serviceArgs is
// expected to be a comma-separated list of key-value pairs in the form
// key=value.
func (s *ChromiumBuilderService) Init(serviceArgs string) error {
	return s.initImpl(context.Background(), serviceArgs, vfs.Local("/"), realCheckoutFactory, realDirectoryCreator, realConcurrentCommandRunner)
}

// GetTools returns the tools supported by the service.
func (s *ChromiumBuilderService) GetTools() []common.Tool {
	sklog.Info("Calling GetTools() for Chromium builder service")
	return []common.Tool{
		{
			Name: "create_ci_combined_builder",
			Description: ("Creates a combined compile/test LUCI builder for Chromium. This means that " +
				"the same builder will be responsible for both compiling and triggering tests. This is " +
				"okay for one-off builders, but adding child testers to an existing parent builder is " +
				"more efficient if multiple testers need to compile with the same GN args. Before the " +
				"generated CL can be submitted, the user will need to file a resource request via " +
				"go/i-need-hw and have it granted. This is to guarantee that there will be sufficient " +
				"GCE capacity for the builder itself as well as test capacity."),
			Arguments: []common.ToolArgument{
				CommonToolArgumentBuilderGroup,
				CommonToolArgumentBuilderName,
				CommonToolArgumentBuilderDescription,
				CommonToolArgumentContactTeamEmail,
				CommonToolArgumentConsoleViewCategory,
				CommonToolArgumentTargetOs,
				CommonToolArgumentTargetArch,
				CommonToolArgumentTargetBits,
				CommonToolArgumentBuildConfig,
				{
					Name: "gn_args",
					Description: ("The GN arg configs for the builder to use when compiling. " +
						"Can be any number of valid configs from " +
						"https://source.chromium.org/chromium/chromium/src/+/main:infra/config/gn_args/gn_args.star. " +
						"At the current moment, only existing GN arg configs are supported, so new ones cannot be created " +
						"as part of this tool."),
					Required:     true,
					ArgumentType: common.ArrayArgument,
					ArraySchema:  map[string]any{"type": "string"},
				},
				CommonToolArgumentTests,
				CommonToolArgumentSwarmingDimensions,
			},
			Handler: s.createCiCombinedBuilderHandler,
		},
		{
			Name: "create_ci_child_tester",
			Description: ("Creates a child tester LUCI builder for Chromium. This is for the case " +
				"where a builder already compiles with the desired GN args. Adding a child tester will " +
				"cause that existing compile builder to compile any necessary test binaries and trigger " +
				"the child tester when it is done. This is more efficient than using a combined " +
				"compile/test builder since the child testers use far fewer resources than builders that " +
				"actually compile. Before the generated CL can be submitted, the user will need to file " +
				"a resource request via go/i-need-hw and have it granted. This is to guarantee that there " +
				"will be sufficient GCE capacity for the builder itself as well as test capacity."),
			Arguments: []common.ToolArgument{
				CommonToolArgumentBuilderGroup,
				CommonToolArgumentBuilderName,
				CommonToolArgumentBuilderDescription,
				CommonToolArgumentContactTeamEmail,
				CommonToolArgumentConsoleViewCategory,
				CommonToolArgumentTargetOs,
				CommonToolArgumentTargetArch,
				CommonToolArgumentTargetBits,
				CommonToolArgumentBuildConfig,
				CommonToolArgumentTests,
				CommonToolArgumentSwarmingDimensions,
				{
					Name: "parent_builder",
					Description: ("The name of the parent builder which will be responsible for compiling " +
						"the test binaries used by the child tester. The parent must be a member of the same " +
						"builder group as the child tester being added, e.g. if the new child tester is being " +
						"added to chromium.fyi, then the parent builder must already exist in chromium.fyi."),
					Required: true,
				},
			},
			Handler: s.createCiChildTesterHandler,
		},
	}
}

func (s *ChromiumBuilderService) GetResources() []common.Resource {
	return []common.Resource{}
}

// createCiCombinedBuilderHandler is the handler for the create_ci_combined_builder
// tool, which creates a combined compile + test builder in Chromium and uploads
// the resulting CL.
func (s *ChromiumBuilderService) createCiCombinedBuilderHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.createCiCombinedBuilderHandlerImpl(ctx, request, vfs.Local("/"), realConcurrentCommandRunner, realEnvironmentGetter)
}

// createCiChildTesterHandler is the handler for the create_ci_child_tester
// tool, which creates a child tester for an existing parent builder in Chromium
// and uploads the resulting CL.
func (s *ChromiumBuilderService) createCiChildTesterHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.createCiChildTesterHandlerImpl(ctx, request, vfs.Local("/"), realConcurrentCommandRunner, realEnvironmentGetter)
}

// Shutdown cleanly shuts down the service. This primarly involves ensuring that
// git operations are either allowed to finish (if they are expected to be fast)
// or are forcibly killed and cleaned up so that no bad state is left on disk.
func (s *ChromiumBuilderService) Shutdown() error {
	return s.shutdownImpl(realDirectoryRemover)
}
