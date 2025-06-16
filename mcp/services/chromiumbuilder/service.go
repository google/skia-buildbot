package chromiumbuilder

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vfs"
	"go.skia.org/infra/mcp/common"
)

const (
	ArgDepotToolsPath string = "depot_tools_path"
	ArgChromiumPath   string = "chromium_path"
)

const (
	BuilderConfigDebug   string = "Debug"
	BuilderConfigRelease string = "Release"
)

const (
	TargetOsAndroid string = "Android"
	TargetOsLinux   string = "Linux"
	TargetOsMac     string = "Mac"
	TargetOsWin     string = "Windows"
)

const (
	TargetArchArm   string = "Arm"
	TargetArchIntel string = "Intel"
)

const (
	ChromiumUrl     string = "https://chromium.googlesource.com/chromium/src"
	DepotToolsUrl   string = "https://chromium.googlesource.com/chromium/tools/depot_tools"
	GerritUrlPrefix string = "https://chromium-review.googlesource.com"
)

var (
	InfraConfigSubdirectory     string = filepath.Join("infra", "config")
	BuilderStarlarkSubdirectory string = filepath.Join(InfraConfigSubdirectory, "subprojects", "chromium", "ci")
)

const (
	CiCombinedBuilderTemplate string = `
ci.builder(
    name = "{{.builderName}}",
    description_html = "{{.builderDescription}}",
    contact_team_email = "{{.contactTeamEmail}}",
    builder_spec = builder_config.builder_spec(
        gclient_config = builder_config.gclient_config(
            config = "chromium",
        ),
        chromium_config = builder_config.chromium_config(
            config = "chromium",
            apply_configs = [
                "mb",
            ],
            build_config = {{.buildConfig}},
            target_arch = {{.targetArch}},
            target_bits = {{.targetBits}},
            target_platform = {{.targetOs}},
        ),
        {{.additionalConfigs}}
    ),
    gn_args = gn_args.config(
        configs = [{{.gnArgs}}],
    ),
    targets = targets.bundle(
        targets = [{{.tests}}],
        mixins = [{{.swarmingDimensions}}],
    ),
    console_view_entry = consoles.console_view_entry(
        category = "{{.consoleViewCategory}}",
    ),
)`

	CiChildTesterTemplate string = `
ci.thin_tester(
    name = "{{.builderName}}",
    description_html = "{{.builderDescription}}",
    parent = "{{.parentBuilder}}",
    contact_team_email = "{{.contactTeamEmail}}",
    builder_spec = builder_config.builder_spec(
        execution_mode = builder_config.execution_mode.TEST,
        gclient_config = builder_config.gclient_config(
            config = "chromium",
        ),
        chromium_config = builder_config.chromium_config(
            config = "chromium",
            apply_configs = [
                "mb",
            ],
            build_config = {{.buildConfig}},
            target_arch = {{.targetArch}},
            target_bits = {{.targetBits}},
            target_platform = {{.targetOs}},
        ),
        {{.additionalConfigs}}
    ),
    targets = targets.bundle(
        targets = [{{.tests}}],
        mixins = [{{.swarmingDimensions}}],
    ),
    console_view_entry = consoles.console_view_entry(
        category = "{{.consoleViewCategory}}",
    ),
)
`
)

// Types used for dependency injection
type checkoutFactory = func(context.Context, string, string) (git.Checkout, error)

func realCheckoutFactory(ctx context.Context, repoUrl, workdir string) (git.Checkout, error) {
	return git.NewCheckout(ctx, repoUrl, workdir)
}

// vfs does not have the concept of directory creation, so we need to have a
// separate way of handling dependency injection for that. In tests, the two
// will likely be backed by the same mock filesystem.
type directoryCreator = func(string, os.FileMode) error

func realDirectoryCreator(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Similarly, vfs does not have the concept of directory removal.
type directoryRemover = func(string) error

func realDirectoryRemover(path string) error {
	return os.RemoveAll(path)
}

// exec.RunIndefinitely() cannot be used as-is for testing like other exec.Run*
// functions since it does not take a context, and it relies directly on
// os/exec.Command behavior (for waiting). It would likely be possible to add
// support for this by abstracting away the os/exec dependency, but for now,
// just use this type for dependency injection.
type concurrentCommandRunner = func(*exec.Command) (exec.Process, <-chan error, error)

func realConcurrentCommandRunner(command *exec.Command) (exec.Process, <-chan error, error) {
	return exec.RunIndefinitely(command)
}

type environmentGetter = func(string) string

func realEnvironmentGetter(key string) string {
	return os.Getenv(key)
}

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

// initImpl is the actual implementation for Init(), broken out to support
// dependency injection.
func (s *ChromiumBuilderService) initImpl(
	ctx context.Context, serviceArgs string, fs vfs.FS, cf checkoutFactory, dc directoryCreator, ccr concurrentCommandRunner) error {
	sklog.Info("Initializing Chromium builder service")
	err := s.parseServiceArgs(serviceArgs)
	if err != nil {
		return err
	}
	sklog.Infof("Parsed args %v", s)

	err = s.handleDepotToolsSetup(ctx, fs, cf, dc)
	if err != nil {
		return err
	}

	err = s.handleChromiumSetup(ctx, fs, cf, dc, ccr)
	if err != nil {
		return err
	}

	sklog.Info("Successfully initialized Chromium builder service")
	return nil
}

// parseServiceArgs parses the string representation of the service's arguments
// and stores the resulting values in the ChromiumBuilderService.
func (s *ChromiumBuilderService) parseServiceArgs(serviceArgs string) error {
	args := strings.Split(serviceArgs, ",")
	for _, pair := range args {
		splitPair := strings.SplitN(pair, "=", 2)
		if len(splitPair) != 2 {
			return skerr.Fmt("Argument %v is not in the expected key=value format", pair)
		}
		key := splitPair[0]
		value := splitPair[1]
		switch key {
		case ArgDepotToolsPath:
			s.depotToolsPath = value
		case ArgChromiumPath:
			s.chromiumPath = value
		default:
			return skerr.Fmt("Unknown argument key %v", key)
		}
	}

	if s.depotToolsPath == "" {
		return skerr.Fmt("Did not receive a %v argument", ArgDepotToolsPath)
	}
	if s.chromiumPath == "" {
		return skerr.Fmt("Did not receive a %v argument", ArgChromiumPath)
	}

	return nil
}

// isNotExistWithUnwraps is a helper function to run os.IsNotExist() on
// possibly wrapped errors.
func isNotExistWithUnwraps(err error) bool {
	for true {
		if err == nil {
			return false
		}
		if os.IsNotExist(err) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

// handleDepotTools ensures that a depot_tools checkout is available at the
// stored path.
func (s *ChromiumBuilderService) handleDepotToolsSetup(ctx context.Context, fs vfs.FS, cf checkoutFactory, dc directoryCreator) error {
	// Check if depot_tools path exists.
	depotToolsDir, err := fs.Open(ctx, s.depotToolsPath)
	if err != nil {
		if isNotExistWithUnwraps(err) {
			return s.handleMissingDepotToolsCheckout(ctx, fs, cf, dc)
		}
		return err
	}
	defer depotToolsDir.Close(ctx)

	return s.handleExistingDepotToolsCheckout(ctx, fs, cf)
}

// handleMissingDepotToolsCheckout sets up a new depot_tools checkout at the
// stored path to handle the case where there is not an existing checkout.
func (s *ChromiumBuilderService) handleMissingDepotToolsCheckout(ctx context.Context, fs vfs.FS, cf checkoutFactory, dc directoryCreator) error {
	sklog.Infof("Did not find existing depot_tools checkout, cloning one at %s", s.depotToolsPath)
	// Ensure the parent directories exist.
	err := dc(filepath.Dir(s.depotToolsPath), 0o750)
	if err != nil {
		return err
	}

	// git.NewCheckout() clones the repo if a checkout doesn't exist at the
	// given directory already, so rely on that behavior.
	err = s.createDepotToolsCheckout(ctx, cf)
	if err != nil {
		return err
	}

	// Creating the checkout creates the repo, but doesn't fetch anything. So,
	// perform an explicit update to pull everything down.
	err = s.updateDepotToolsCheckout(ctx)
	if err != nil {
		return err
	}

	return nil
}

// handleExistingDepotToolsCheckout ensures that an existing depot_tools
// checkout is valid and up to date.
func (s *ChromiumBuilderService) handleExistingDepotToolsCheckout(ctx context.Context, fs vfs.FS, cf checkoutFactory) error {
	sklog.Infof("Found existing depot_tools checkout at %s", s.depotToolsPath)
	// Check that the provided path is actually a directory.
	err := checkIfPathIsDirectory(ctx, fs, s.depotToolsPath)
	if err != nil {
		return err
	}

	// Check that an expected tool exists.
	lucicfgPath := filepath.Join(s.depotToolsPath, "lucicfg")
	lucicfg, err := fs.Open(ctx, lucicfgPath)
	if err != nil {
		return err
	}
	defer lucicfg.Close(ctx)

	// Check that this appears to be an actual git repo.
	dotGitPath := filepath.Join(s.depotToolsPath, ".git")
	err = checkIfPathIsDirectory(ctx, fs, dotGitPath)
	if err != nil {
		return err
	}

	err = s.createDepotToolsCheckout(ctx, cf)
	if err != nil {
		return err
	}

	err = s.updateDepotToolsCheckout(ctx)
	if err != nil {
		return err
	}

	return nil
}

// handleChromiumSetup ensures that a Chromium checkout is available at the
// stored path.
func (s *ChromiumBuilderService) handleChromiumSetup(
	ctx context.Context, fs vfs.FS, cf checkoutFactory, dc directoryCreator, ccr concurrentCommandRunner) error {
	// Check if the Chromium path exists.
	chromiumDir, err := fs.Open(ctx, s.chromiumPath)
	if err != nil {
		if isNotExistWithUnwraps(err) {
			return s.handleMissingChromiumCheckout(ctx, fs, cf, dc, ccr)
		}
		return err
	}
	defer chromiumDir.Close(ctx)

	return s.handleExistingChromiumCheckout(ctx, fs, cf)
}

// handleMissingChromiumCheckout sets up a new Chromium checkout at the stored
// path to handle the case where there is not an existing checkout.
func (s *ChromiumBuilderService) handleMissingChromiumCheckout(
	ctx context.Context, fs vfs.FS, cf checkoutFactory, dc directoryCreator, ccr concurrentCommandRunner) error {
	sklog.Infof("Did not find existing Chromium checkout, fetching one at %s", s.chromiumPath)
	// Ensure the parent directories exist.
	err := dc(filepath.Dir(s.chromiumPath), 0o750)
	if err != nil {
		return err
	}

	err = s.fetchChromium(ccr)
	if err != nil {
		return err
	}

	// Obtain a re-usable checkout.
	err = s.createChromiumCheckout(ctx, cf)
	if err != nil {
		return err
	}

	// The checkout will already be up to date after the fetch, so no need to
	// explicitly update here.

	return nil
}

// handleExistingChromiumCheckout ensures that the existing Chromium checkout is
// valid and up to date.
func (s *ChromiumBuilderService) handleExistingChromiumCheckout(ctx context.Context, fs vfs.FS, cf checkoutFactory) error {
	sklog.Infof("Found existing Chromium checkout at %s", s.chromiumPath)
	// Check that the provided path is actually a directory.
	err := checkIfPathIsDirectory(ctx, fs, s.chromiumPath)
	if err != nil {
		return err
	}

	// Check that this appears to be an actual git repo.
	dotGitPath := filepath.Join(s.chromiumPath, ".git")
	err = checkIfPathIsDirectory(ctx, fs, dotGitPath)
	if err != nil {
		return err
	}

	// Obtain a re-usable checkout and ensure it is up to date.
	err = s.createChromiumCheckout(ctx, cf)
	if err != nil {
		return err
	}
	err = s.updateChromiumCheckout(ctx)
	if err != nil {
		return err
	}

	return nil
}

// checkIfPathIsDirectory is a helper to check if the provided path exists and
// is a directory.
func checkIfPathIsDirectory(ctx context.Context, fs vfs.FS, path string) error {
	// Check if the provided path exists.
	fileHandle, err := fs.Open(ctx, path)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer fileHandle.Close(ctx)

	// Check if the provided path is actually a directory.
	fileInfo, err := fileHandle.Stat(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	if !fileInfo.IsDir() {
		return skerr.Fmt("Path %s exists, but is not a directory.", path)
	}

	return nil
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

// createCiCombinedBuilderHandlerImpl is the actual implementation for
// createCiCombinedBuilderHandler, broken out to support dependency injection.
func (s *ChromiumBuilderService) createCiCombinedBuilderHandlerImpl(
	ctx context.Context, request mcp.CallToolRequest, fs vfs.FS, ccr concurrentCommandRunner, eg environmentGetter) (*mcp.CallToolResult, error) {
	s.handlingToolRequestLock.Lock()
	defer s.handlingToolRequestLock.Unlock()
	sklog.Infof("Calling createCiCombinedBuilderHandlerImpl with %v", s)
	inputs, err := extractCiCombinedBuilderInputs(request)
	if err != nil {
		sklog.Errorf("Error extracting inputs: %v", err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	branchName, err := s.prepareCheckoutsForStarlarkModification(ctx)
	if err != nil {
		return mcp.NewToolResultError("Server had an internal error preparing checkouts. This is not actionable by the client."), nil
	}
	defer s.cleanUpBranchDeferred(ctx, branchName)

	err = s.addNewCiCombinedBuilder(ctx, inputs, fs)
	if err != nil {
		sklog.Errorf("Error adding new builder: %v", err)
		return mcp.NewToolResultError("Server encountered an error while modifying builder files. This is not actionable by the client."), nil
	}

	err = s.handleStarlarkFormattingAndGeneration(ctx, ccr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf(
			"Server encountered an error while cleaning up and generating files using the provided inputs. "+
				"This may be due to invalid inputs, e.g. a typo causing a matching test to not be found. Error: %v",
			err.Error())), nil
	}

	clLink, err := s.handleCommitAndUpload(ctx, inputs.builderName, inputs.builderGroup, ccr, eg)
	if err != nil {
		return mcp.NewToolResultError(
			"Server encountered an error while committing and uploading changes. This is not actionable by the client."), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Created and uploaded CL to %s", clLink)), nil
}

// ciCombinedBuilderInputs stores all MCP arguments for the
// create_ci_combined_builder tool.
type ciCombinedBuilderInputs struct {
	builderGroup        string
	builderName         string
	builderDescription  string
	contactTeamEmail    string
	consoleViewCategory string
	targetOs            string
	targetArch          string
	targetBits          int
	buildConfig         string
	gnArgs              []string
	tests               []string
	swarmingDimensions  []string
}

// extractCiCombinedBuilderInputs extracts all expected arguments for the
// create_ci_combined_builder tool from the given MCP request and stores them
// in a ciCombinedBuilderInputs.
func extractCiCombinedBuilderInputs(request mcp.CallToolRequest) (ciCombinedBuilderInputs, error) {
	inputs := ciCombinedBuilderInputs{}
	var err error

	inputs.builderGroup, err = request.RequireString("builder_group")
	if err != nil {
		return inputs, err
	}

	inputs.builderName, err = request.RequireString("builder_name")
	if err != nil {
		return inputs, err
	}

	inputs.builderDescription, err = request.RequireString("builder_description")
	if err != nil {
		return inputs, err
	}

	inputs.contactTeamEmail, err = request.RequireString("contact_team_email")
	if err != nil {
		return inputs, err
	}

	inputs.consoleViewCategory, err = request.RequireString("console_view_category")
	if err != nil {
		return inputs, err
	}

	inputs.targetOs, err = request.RequireString("target_os")
	if err != nil {
		return inputs, err
	}

	inputs.targetArch, err = request.RequireString("target_arch")
	if err != nil {
		return inputs, err
	}

	inputs.targetBits, err = request.RequireInt("target_bits")
	if err != nil {
		return inputs, err
	}

	inputs.buildConfig, err = request.RequireString("build_config")
	if err != nil {
		return inputs, err
	}

	inputs.gnArgs, err = request.RequireStringSlice("gn_args")
	if err != nil {
		return inputs, err
	}

	inputs.tests, err = request.RequireStringSlice("tests")
	if err != nil {
		return inputs, err
	}

	inputs.swarmingDimensions, err = request.RequireStringSlice("swarming_dimensions")
	if err != nil {
		return inputs, err
	}

	return inputs, nil
}

// createCiChildTesterHandler is the handler for the create_ci_child_tester
// tool, which creates a child tester for an existing parent builder in Chromium
// and uploads the resulting CL.
func (s *ChromiumBuilderService) createCiChildTesterHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.createCiChildTesterHandlerImpl(ctx, request, vfs.Local("/"), realConcurrentCommandRunner, realEnvironmentGetter)
}

// createCiChildTesterHandlerImpl is the actual implementation for
// createCiChildTesterHandler, broken out to support dependency injection.
func (s *ChromiumBuilderService) createCiChildTesterHandlerImpl(
	ctx context.Context, request mcp.CallToolRequest, fs vfs.FS, ccr concurrentCommandRunner, eg environmentGetter) (*mcp.CallToolResult, error) {
	inputs, err := extractCiChildTesterInputs(request)
	if err != nil {
		sklog.Errorf("Error extracting inputs: %v", err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	branchName, err := s.prepareCheckoutsForStarlarkModification(ctx)
	if err != nil {
		return mcp.NewToolResultError("Server had an internal error preparing checkouts. This is not actionable by the client."), nil
	}
	defer s.cleanUpBranchDeferred(ctx, branchName)

	err = s.addNewCiChildTester(ctx, inputs, fs)
	if err != nil {
		sklog.Errorf("Error adding new builder: %v", err)
		return mcp.NewToolResultError("Server encountered an error while modifying builder files. This is not actionable by the client."), nil
	}

	err = s.handleStarlarkFormattingAndGeneration(ctx, ccr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf(
			"Server encountered an error while cleaning up and generating files using the provided inputs. "+
				"This may be due to invalid inputs, e.g. a typo causing a matching test to not be found. Error: %v",
			err.Error())), nil
	}

	clLink, err := s.handleCommitAndUpload(ctx, inputs.builderName, inputs.builderGroup, ccr, eg)
	if err != nil {
		return mcp.NewToolResultError(
			"Server encountered an error while committing and uploading changes. This is not actionable by the client."), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Created and uploaded CL to %s", clLink)), nil
}

// ciChildTesterInputs stores all MCP arguments for the create_ci_child_tester
// tool.
type ciChildTesterInputs struct {
	builderGroup        string
	builderName         string
	builderDescription  string
	contactTeamEmail    string
	consoleViewCategory string
	targetOs            string
	targetArch          string
	targetBits          int
	buildConfig         string
	tests               []string
	swarmingDimensions  []string
	parentBuilder       string
}

// extractCiChildTesterInputs extracts all expected arguments for the
// create_ci_child_tester tool from the given MCP request and stores them in
// a ciChildTesterInputs.
func extractCiChildTesterInputs(request mcp.CallToolRequest) (ciChildTesterInputs, error) {
	inputs := ciChildTesterInputs{}
	var err error

	inputs.builderGroup, err = request.RequireString("builder_group")
	if err != nil {
		return inputs, err
	}

	inputs.builderName, err = request.RequireString("builder_name")
	if err != nil {
		return inputs, err
	}

	inputs.builderDescription, err = request.RequireString("builder_description")
	if err != nil {
		return inputs, err
	}

	inputs.contactTeamEmail, err = request.RequireString("contact_team_email")
	if err != nil {
		return inputs, err
	}

	inputs.consoleViewCategory, err = request.RequireString("console_view_category")
	if err != nil {
		return inputs, err
	}

	inputs.targetOs, err = request.RequireString("target_os")
	if err != nil {
		return inputs, err
	}

	inputs.targetArch, err = request.RequireString("target_arch")
	if err != nil {
		return inputs, err
	}

	inputs.targetBits, err = request.RequireInt("target_bits")
	if err != nil {
		return inputs, err
	}

	inputs.buildConfig, err = request.RequireString("build_config")
	if err != nil {
		return inputs, err
	}

	inputs.tests, err = request.RequireStringSlice("tests")
	if err != nil {
		return inputs, err
	}

	inputs.swarmingDimensions, err = request.RequireStringSlice("swarming_dimensions")
	if err != nil {
		return inputs, err
	}

	inputs.parentBuilder, err = request.RequireString("parent_builder")
	if err != nil {
		return inputs, err
	}

	return inputs, nil
}

// createDepotToolsCheckout creates and stores a re-usable reference to the
// depot_tools checkout.
func (s *ChromiumBuilderService) createDepotToolsCheckout(ctx context.Context, cf checkoutFactory) error {
	s.depotToolsCheckoutLock.Lock()
	defer s.depotToolsCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return skerr.Fmt("Server is shutting down, not proceeding with depot_tools checkout.")
	}
	var err error
	s.depotToolsCheckout, err = cf(ctx, DepotToolsUrl, filepath.Dir(s.depotToolsPath))
	if err != nil {
		return err
	}

	return nil
}

func (s *ChromiumBuilderService) createChromiumCheckout(ctx context.Context, cf checkoutFactory) error {
	s.chromiumCheckoutLock.Lock()
	defer s.chromiumCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return skerr.Fmt("Server is shutting down, not proceeding with Chromium checkout.")
	}
	var err error
	s.chromiumCheckout, err = cf(ctx, ChromiumUrl, filepath.Dir(s.chromiumPath))
	if err != nil {
		return err
	}

	return nil
}

// prepareCheckoutsForStarlarkModification performs all of the checkout-related
// steps that are required between when a tool request starts being handled and
// when local Starlark files are ready for modification. On success, it returns
// the temporary branch name that is currently checked out. Any non-nil errors
// are indicative of a checkout issue which is not something that the MCP client
// can help resolve.
func (s *ChromiumBuilderService) prepareCheckoutsForStarlarkModification(ctx context.Context) (string, error) {
	err := s.updateCheckouts(ctx)
	if err != nil {
		sklog.Errorf("Error updating checkouts: %v", err)
		return "", err
	}

	branchName, err := s.switchToTemporaryBranch(ctx)
	if err != nil {
		sklog.Errorf("Error checking out temporary branch: %v", err)
		return "", err
	}

	return branchName, nil
}

// updateCheckouts ensures that both depot_tools and Chromium are up to date
// with origin/main.
func (s *ChromiumBuilderService) updateCheckouts(ctx context.Context) error {
	err := s.updateDepotToolsCheckout(ctx)
	if err != nil {
		return err
	}

	err = s.updateChromiumCheckout(ctx)
	if err != nil {
		return err
	}

	return nil
}

// updateDepotToolsCheckout ensures that depot_tools is up to date with
// origin/main.
func (s *ChromiumBuilderService) updateDepotToolsCheckout(ctx context.Context) error {
	sklog.Info("Updating depot_tools checkout")
	s.depotToolsCheckoutLock.Lock()
	defer s.depotToolsCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return skerr.Fmt("Server is shutting down, not proceeding with depot_tools update")
	}
	err := s.depotToolsCheckout.Update(ctx)
	if err != nil {
		return err
	}
	return nil
}

// updateChromiumCheckout ensures that Chromium is up to date with origin/main.
// This does *not* interact with gclient, as DEPS should not be needed for
// interacting with //infra/config.
func (s *ChromiumBuilderService) updateChromiumCheckout(ctx context.Context) error {
	sklog.Info("Updating Chromium checkout")
	s.chromiumCheckoutLock.Lock()
	defer s.chromiumCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return skerr.Fmt("Server is shutting down, not proceeding with Chromium update")
	}

	err := s.chromiumCheckout.Update(ctx)
	if err != nil {
		return err
	}

	return nil
}

// switchToTemporaryBranch creates a uniquely named branch and switches to it,
// returning the new branch name.
func (s *ChromiumBuilderService) switchToTemporaryBranch(ctx context.Context) (string, error) {
	s.chromiumCheckoutLock.Lock()
	defer s.chromiumCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return "", skerr.Fmt("Server is shutting down, not proceeding with branch switch")
	}

	branchName := fmt.Sprintf("%d", time.Now().UnixMilli())
	_, err := s.chromiumCheckout.Git(ctx, "checkout", "-b", branchName)
	if err != nil {
		return "", err
	}

	return branchName, nil
}

// cleanUpBranch switches back to the main branch in the Chromium checkout and
// deletes the specified branch.
func (s *ChromiumBuilderService) cleanUpBranch(ctx context.Context, branchName string) error {
	sklog.Infof("Cleaning up branch %s", branchName)
	s.chromiumCheckoutLock.Lock()
	defer s.chromiumCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return skerr.Fmt("Server is shutting down, not proceeding with branch cleanup")
	}

	_, err := s.chromiumCheckout.Git(ctx, "checkout", "main")
	if err != nil {
		return err
	}

	_, err = s.chromiumCheckout.Git(ctx, "branch", "-D", branchName)
	if err != nil {
		return err
	}

	sklog.Infof("Successfully cleaned up branch %s", branchName)
	return nil
}

// cleanUpBranchDeferred is a version of cleanUpBranch that is meant to be used
// via defer. Any errors from cleanUpBranch are logged, but not propagated.
func (s *ChromiumBuilderService) cleanUpBranchDeferred(ctx context.Context, branchName string) {
	err := s.cleanUpBranch(ctx, branchName)
	if err != nil {
		sklog.Errorf("Error when trying to clean up branch %s: %v", branchName, err)
	}
}

// addNewCiCombinedBuilder goes through all the steps necessary to add a new
// combined compile/test builder definition to the relevant Starlark file on
// disk.
func (s *ChromiumBuilderService) addNewCiCombinedBuilder(ctx context.Context, inputs ciCombinedBuilderInputs, fs vfs.FS) error {
	sklog.Infof("Adding combined compile/test builder with inputs %v", inputs)

	starlarkFilename := fmt.Sprintf("%s.star", inputs.builderGroup)
	starlarkFilepath := filepath.Join(s.chromiumPath, BuilderStarlarkSubdirectory, starlarkFilename)
	starlarkFile, err := fs.Open(ctx, starlarkFilepath)
	if err != nil {
		return err
	}
	defer starlarkFile.Close(ctx)

	buildConfig, err := determineBuildConfig(inputs.buildConfig)
	if err != nil {
		return err
	}
	targetArch, err := determineTargetArch(inputs.targetArch)
	if err != nil {
		return err
	}
	targetOs, err := determineTargetOs(inputs.targetOs)
	if err != nil {
		return err
	}
	additionalConfigs, err := determineAdditionalConfigs(inputs.targetOs)
	if err != nil {
		return err
	}
	gnArgs, err := determineGnArgs(inputs.gnArgs)
	if err != nil {
		return err
	}
	tests, err := determineTests(inputs.tests)
	if err != nil {
		return err
	}
	swarmingDimensions, err := determineSwarmingDimensions(inputs.swarmingDimensions)
	if err != nil {
		return err
	}

	formatData := map[string]string{
		"builderName":         inputs.builderName,
		"builderDescription":  inputs.builderDescription,
		"contactTeamEmail":    inputs.contactTeamEmail,
		"buildConfig":         buildConfig,
		"targetArch":          targetArch,
		"targetBits":          fmt.Sprintf("%d", inputs.targetBits),
		"targetOs":            targetOs,
		"additionalConfigs":   additionalConfigs,
		"gnArgs":              gnArgs,
		"tests":               tests,
		"swarmingDimensions":  swarmingDimensions,
		"consoleViewCategory": inputs.consoleViewCategory,
	}

	builderDefinition, err := formatString(CiCombinedBuilderTemplate, formatData)
	if err != nil {
		return err
	}

	wrappedFile := vfs.WithContext(ctx, starlarkFile)
	contentBytes, err := io.ReadAll(wrappedFile)
	if err != nil {
		return err
	}
	wrappedFile.Close()

	contentString := string(contentBytes[:])
	contentString += builderDefinition
	contentBytes = []byte(contentString)
	err = vfs.WriteFile(ctx, fs, starlarkFilepath, contentBytes)
	if err != nil {
		return err
	}

	return nil
}

// addNewCiChildTester goes through all the steps necessary to add a new child
// tester definition to the relevant Starlark file on disk.
func (s *ChromiumBuilderService) addNewCiChildTester(ctx context.Context, inputs ciChildTesterInputs, fs vfs.FS) error {
	sklog.Infof("Adding child tester with inputs %v", inputs)

	starlarkFilename := fmt.Sprintf("%s.star", inputs.builderGroup)
	starlarkFilepath := filepath.Join(s.chromiumPath, BuilderStarlarkSubdirectory, starlarkFilename)
	starlarkFile, err := fs.Open(ctx, starlarkFilepath)
	if err != nil {
		return err
	}
	defer starlarkFile.Close(ctx)

	buildConfig, err := determineBuildConfig(inputs.buildConfig)
	if err != nil {
		return err
	}
	targetArch, err := determineTargetArch(inputs.targetArch)
	if err != nil {
		return err
	}
	targetOs, err := determineTargetOs(inputs.targetOs)
	if err != nil {
		return err
	}
	additionalConfigs, err := determineAdditionalConfigs(inputs.targetOs)
	if err != nil {
		return err
	}
	tests, err := determineTests(inputs.tests)
	if err != nil {
		return err
	}
	swarmingDimensions, err := determineSwarmingDimensions(inputs.swarmingDimensions)
	if err != nil {
		return err
	}

	formatData := map[string]string{
		"builderName":         inputs.builderName,
		"builderDescription":  inputs.builderDescription,
		"contactTeamEmail":    inputs.contactTeamEmail,
		"buildConfig":         buildConfig,
		"targetArch":          targetArch,
		"targetBits":          fmt.Sprintf("%d", inputs.targetBits),
		"targetOs":            targetOs,
		"additionalConfigs":   additionalConfigs,
		"tests":               tests,
		"swarmingDimensions":  swarmingDimensions,
		"consoleViewCategory": inputs.consoleViewCategory,
		"parentBuilder":       inputs.parentBuilder,
	}

	builderDefinition, err := formatString(CiChildTesterTemplate, formatData)
	if err != nil {
		return err
	}

	wrappedFile := vfs.WithContext(ctx, starlarkFile)
	contentBytes, err := io.ReadAll(wrappedFile)
	if err != nil {
		return err
	}
	wrappedFile.Close()

	contentString := string(contentBytes[:])
	contentString += builderDefinition
	contentBytes = []byte(contentString)
	err = vfs.WriteFile(ctx, fs, starlarkFilepath, contentBytes)
	if err != nil {
		return err
	}

	return nil
}

// determineBuildConfig translates the provided string to the corresponding
// Starlark constant.
func determineBuildConfig(buildConfig string) (string, error) {
	switch buildConfig {
	case BuilderConfigDebug:
		return "builder_config.build_config.DEBUG", nil
	case BuilderConfigRelease:
		return "builder_config.build_config.RELEASE", nil
	default:
		return "", skerr.Fmt("Unhandled builder config %s", buildConfig)
	}
}

// determineTargetArch translates the provided string to the corresponding
// Starlark constant.
func determineTargetArch(targetArch string) (string, error) {
	switch targetArch {
	case TargetArchArm:
		return "builder_config.target_arch.ARM", nil
	case TargetArchIntel:
		return "builder_config.target_arch.INTEL", nil
	default:
		return "", skerr.Fmt("Unhandled target architecture %s", targetArch)
	}
}

// determineTargetOs translates the provided string to the corresponding
// Starlark constant.
func determineTargetOs(targetOs string) (string, error) {
	switch targetOs {
	case TargetOsAndroid:
		return "builder_config.target_platform.ANDROID", nil
	case TargetOsLinux:
		return "builder_config.target_platform.LINUX", nil
	case TargetOsMac:
		return "builder_config.target_platform.MAC", nil
	case TargetOsWin:
		return "builder_config.target_platform.WIN", nil
	default:
		return "", skerr.Fmt("Unhandled target OS %s", targetOs)
	}
}

// determineAdditionalConfigs returns a string containing any additional
// Starlark configs that should be added to the builder's builder_spec entry
// based on the provided inputs.
func determineAdditionalConfigs(targetOs string) (string, error) {
	additionalConfigs := ""
	if targetOs == TargetOsAndroid {
		additionalConfigs += `android_config = builder_config.android_config(config = "base_config"),`
	}
	return additionalConfigs, nil
}

// quoteAndCommaSeparate is a helper to wrap each element in the provided string
// slice in double quotes then join them with commas.
func quoteAndCommaSeparate(stringSlice []string) string {
	quotedStrings := []string{}
	for _, s := range stringSlice {
		quotedStrings = append(quotedStrings, fmt.Sprintf(`"%s"`, s))
	}
	return strings.Join(quotedStrings, ", ")
}

// determineGnArgs translates the provided string slice containing GN arg
// configs to a string usable as the contents for a Starlark list.
func determineGnArgs(gnArgs []string) (string, error) {
	return quoteAndCommaSeparate(gnArgs), nil
}

// determineTests translates the provided string slice containing tests to a
// string usable as the contents for a Starlark list.
func determineTests(tests []string) (string, error) {
	return quoteAndCommaSeparate(tests), nil
}

// determineSwarmingDimensions translates the provided string slice containing
// Swarming mixins to a string usable as the contents for a Starlark list.
func determineSwarmingDimensions(swarmingDimensions []string) (string, error) {
	return quoteAndCommaSeparate(swarmingDimensions), nil
}

// formatString is a helper to format a given string using the provided
// key-value pairs.
func formatString(format string, data map[string]string) (string, error) {
	tmpl, err := template.New("format").Parse(format)
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, data)
	if err != nil {
		return "", err
	}
	return buffer.String(), nil
}

// handleStarlarkFormattingAndGeneration performs all of the Starlark-related
// steps that are required between when Starlark files are modified and when
// files are ready to be committed/uploaded. Non-nil errors may or may not be
// fixable by the MCP client. An example of a fixable issue would be a a typo
// in one of the inputs which causes Starlark file generation to fail. An
// example of a non-fixable issue would be Starlark file generation failing due
// to the used template being out of date.
func (s *ChromiumBuilderService) handleStarlarkFormattingAndGeneration(ctx context.Context, ccr concurrentCommandRunner) error {
	err := s.formatStarlark(ctx, ccr)
	if err != nil {
		sklog.Errorf("Error formatting Starlark: %v", err)
		return err
	}

	err = s.generateFilesFromStarlark(ctx, ccr)
	if err != nil {
		sklog.Errorf("Error generating files from Starlark: %v", err)
		return err
	}

	return nil
}

// formatStarlark runs lucicfg to format the Starlark files contained within
// the Chromium checkout.
func (s *ChromiumBuilderService) formatStarlark(ctx context.Context, ccr concurrentCommandRunner) error {
	lucicfgPath := filepath.Join(s.depotToolsPath, "lucicfg")
	infraConfigPath := filepath.Join(s.chromiumPath, InfraConfigSubdirectory)

	output := bytes.Buffer{}
	err := s.runSafeCancellableCommand(&exec.Command{
		Name:           lucicfgPath,
		Args:           []string{"fmt", infraConfigPath},
		CombinedOutput: &output,
	}, ccr)
	if err != nil {
		return skerr.Fmt("Failed to format Starlark. Original error: %v Stdout: %s", err, output.String())
	}

	return nil
}

// generateFilesFromStarlark runs Chromium's main Starlark file to generate any
// JSON/pyl/etc. files based on any changes to Starlark files.
func (s *ChromiumBuilderService) generateFilesFromStarlark(ctx context.Context, ccr concurrentCommandRunner) error {
	starlarkMainPath := filepath.Join(s.chromiumPath, InfraConfigSubdirectory, "main.star")

	output := bytes.Buffer{}
	err := s.runSafeCancellableCommand(&exec.Command{
		Name:           starlarkMainPath,
		Args:           []string{},
		CombinedOutput: &output,
	}, ccr)
	if err != nil {
		return skerr.Fmt("Failed to generate files from Starlark. Original error: %v Stdout: %s", err, output.String())
	}

	return nil
}

// handleCommitAndUpload commits all local changes in the Chromium checkout and
// uploads them as a new CL on Gerrit. On success, it returns the link to the
// uploaded CL. Any non-nil errors are indicative of a checkout/network/Gerrit
// issue which is not something that the MCP client can help resolve.
func (s *ChromiumBuilderService) handleCommitAndUpload(
	ctx context.Context, builderName, builderGroup string, ccr concurrentCommandRunner, eg environmentGetter) (string, error) {
	err := s.addAndCommitFiles(ctx, builderName, builderGroup)
	if err != nil {
		sklog.Errorf("Error adding and committing files: %v", err)
		return "", err
	}

	clLink, err := s.uploadCl(ctx, ccr, eg)
	if err != nil {
		sklog.Errorf("Error uploading CL: %v", err)
		return "", err
	}

	return clLink, nil
}

// addAndCommitFiles adds all files under Chromium's //infra/config directory to
// git then commits them.
func (s *ChromiumBuilderService) addAndCommitFiles(ctx context.Context, builderName, builderGroup string) error {
	s.chromiumCheckoutLock.Lock()
	defer s.chromiumCheckoutLock.Unlock()

	if s.shuttingDown.Load() {
		return skerr.Fmt("Server is shutting down, not proceeding with adding/committing files.")
	}

	infraConfigPath := filepath.Join(s.chromiumPath, InfraConfigSubdirectory)
	_, err := s.chromiumCheckout.Git(ctx, "add", infraConfigPath)
	if err != nil {
		return err
	}

	clTitle := fmt.Sprintf("Add new builder %s", builderName)
	clDescription := fmt.Sprintf("Adds a new builder %s in the %s group. This CL was auto-generated.", builderName, builderGroup)
	_, err = s.chromiumCheckout.Git(ctx, "commit", "-m", clTitle, "-m", clDescription)
	if err != nil {
		return err
	}

	return nil
}

// uploadCl uploads committed changes to Gerrit and returns the uploaded CL's
// link.
func (s *ChromiumBuilderService) uploadCl(ctx context.Context, ccr concurrentCommandRunner, eg environmentGetter) (string, error) {
	gitClPath := filepath.Join(s.depotToolsPath, "git_cl.py")

	// git_cl.py relies on some additional tools within depot_tools, most
	// notably vpython3. So, add depot_tools to PATH for this command. Other
	// environment variables are inherited as-is due to InheritEnv.
	envPath := fmt.Sprintf("PATH=%s", s.depotToolsPath)
	existingPath := eg("PATH")
	if existingPath != "" {
		envPath = fmt.Sprintf("%s:%s", envPath, existingPath)
	}

	output := bytes.Buffer{}
	err := s.runSafeCancellableCommand(&exec.Command{
		Name:           gitClPath,
		Args:           []string{"upload", "--skip-title", "--bypass-hooks", "--force"},
		Env:            []string{envPath},
		InheritEnv:     true,
		Dir:            s.chromiumPath,
		CombinedOutput: &output,
		Timeout:        5 * time.Minute,
	}, ccr)
	if err != nil {
		return "", skerr.Fmt("Failed to upload CL to Gerrit. Original error: %v Stdout: %s", err, output.String())
	}

	outputString := output.String()
	sklog.Errorf("Uploaded with output: %s", outputString)

	clLink := ""
	for _, f := range strings.Fields(outputString) {
		if strings.HasPrefix(f, GerritUrlPrefix) {
			clLink = f
			break
		}
	}
	if clLink == "" {
		return clLink, skerr.Fmt("Unable to extract Gerrit link from git cl upload output")
	}

	return clLink, nil
}

// fetchChromium fetches a Chromium checkout using the stored path.
func (s *ChromiumBuilderService) fetchChromium(ccr concurrentCommandRunner) error {
	// If we end up cancelling the fetch command mid-run, we will have to
	// perform additional cleanup in order to ensure that the checkout is not
	// in a bad state. Hence, we have our own lock and cannot use
	// runSafeCancellableCommand().
	sklog.Infof("Fetching Chromium checkout into %s. This will take a while.", s.chromiumPath)
	fetchPath := filepath.Join(s.depotToolsPath, "fetch")
	output := bytes.Buffer{}
	cmd := exec.Command{
		Name:           fetchPath,
		Args:           []string{"--nohooks", "chromium"},
		CombinedOutput: &output,
		Dir:            filepath.Dir(s.chromiumPath),
	}
	err := s.runCancellableCommand(&cmd, ccr, &(s.chromiumFetchLock))
	if err != nil {
		return skerr.Fmt("Failed to fetch Chromium. Original error: %v Stdout: %s", err, output.String())
	}
	sklog.Info("Successfully fetched Chromium checkout")

	return nil
}

// runSafeCancellableCommand runs the provided Command in such a way that it can
// be cancelled mid-run. Any commands run this way must not result in bad state
// being left on disk in the event of the command being cancelled.
func (s *ChromiumBuilderService) runSafeCancellableCommand(cmd *exec.Command, ccr concurrentCommandRunner) error {
	return s.runCancellableCommand(cmd, ccr, &(s.safeCancellableCommandLock))
}

// runCancellableCommand runs the provided Command in such a way that it can be
// cancelled mid-run. The sync.Mutex argument will be locked for the duration
// of the function to signal that some cancellable command is being run.
func (s *ChromiumBuilderService) runCancellableCommand(cmd *exec.Command, ccr concurrentCommandRunner, lock *sync.Mutex) error {
	lock.Lock()
	defer lock.Unlock()
	// This is manually unlocked later so we can release it sooner.
	s.currentProcessLock.Lock()

	if s.shuttingDown.Load() {
		s.currentProcessLock.Unlock()
		return skerr.Fmt("Server is shutting down, not starting cancellable command.")
	}

	process, doneChan, err := ccr(cmd)
	s.currentProcess = process
	s.currentProcessLock.Unlock()
	if err != nil {
		return err
	}
	err = <-doneChan
	s.currentProcessLock.Lock()
	s.currentProcess = nil
	s.currentProcessLock.Unlock()
	if err != nil {
		return err
	}

	return nil
}

// Shutdown cleanly shuts down the service. This primarly involves ensuring that
// git operations are either allowed to finish (if they are expected to be fast)
// or are forcibly killed and cleaned up so that no bad state is left on disk.
func (s *ChromiumBuilderService) Shutdown() error {
	return s.shutdownImpl(realDirectoryRemover)
}

// shutdownImpl is the actual implementation for Shutdown(), broken out to
// support dependency injection.
func (s *ChromiumBuilderService) shutdownImpl(dr directoryRemover) error {
	sklog.Infof("Shutting down Chromium Builder service")
	s.shuttingDown.Store(true)

	err := s.ensureDepotToolsCheckoutNotInUse()
	if err != nil {
		return err
	}

	err = s.ensureChromiumCheckoutNotInUse()
	if err != nil {
		return err
	}

	err = s.cancelSafeCommands()
	if err != nil {
		return err
	}

	err = s.cancelChromiumFetch(dr)
	if err != nil {
		return err
	}

	return nil
}

// ensureDepotToolsCheckoutNotInUse ensures that the depot_tools checkout is not
// actively being used before continuing with shutdown. Killing the server while
// it is in use, e.g. mid-update, could leave the checkout in an unusable state
// which would affect the server the next time it is deployed.
func (s *ChromiumBuilderService) ensureDepotToolsCheckoutNotInUse() error {
	if !s.shuttingDown.Load() {
		return skerr.Fmt("ensureDepotToolsCheckoutNotInUse() must only be called during shutdown.")
	}

	s.depotToolsCheckoutLock.Lock()
	defer s.depotToolsCheckoutLock.Unlock()

	// Both the initial checkout and updating of depot_tools is very quick, so
	// just let them run their course. Both are handled via the git package
	// rather than the exec package anyways, so we would not be able to cancel
	// them mid-run.
	return nil
}

// ensureChromiumCheckoutNotInUse ensures that the Chromium checkout is not
// actively being used before continuing with shutdown. Killing the server while
// it is in use, e.g. mid-update, could leave the checkout in an unusable state
// which would affect the server the next time it is deployed.
func (s *ChromiumBuilderService) ensureChromiumCheckoutNotInUse() error {
	if !s.shuttingDown.Load() {
		return skerr.Fmt("ensureChromiumCheckoutNotInUse() must only be called during shutdown.")
	}

	s.chromiumCheckoutLock.Lock()
	defer s.chromiumCheckoutLock.Unlock()

	// Initial checkout setup is handled via fetch, which can be cancelled
	// in another shutdown helper. Updating the Chromium checkout should not
	// take too long, and isn't cancellable anyways due to use of the git
	// package instead of the exec package.
	return nil
}

// cancelSafeCommands cancels any in-progress commands which are safe to cancel
// without any additional cleanup.
func (s *ChromiumBuilderService) cancelSafeCommands() error {
	if !s.shuttingDown.Load() {
		return skerr.Fmt("cancelSafeCommands() must only be called during shutdown.")
	}

	notCurrentlyRunning := s.safeCancellableCommandLock.TryLock()
	if notCurrentlyRunning {
		s.safeCancellableCommandLock.Unlock()
		return nil
	}

	s.currentProcessLock.Lock()
	defer s.currentProcessLock.Unlock()

	if s.currentProcess == nil {
		// This can happen in one of two ways:
		//   1. We tried to acquire safeCallableCommandLock just as it was
		//      acquired by the function running the command. In this case, we
		//      can safely assume that the current process won't be set later
		//      since that function will detect that the server is shutting down
		//      and not start the process.
		//   2. We tried to acquire safeCallableCommandLock as the function
		//      running the command was finishing. In this case, the process has
		//      already finished.
		// In both cases, it is safe to not do anything else.
		return nil
	}

	err := s.currentProcess.Kill()
	if err != nil {
		// We don't return this error since we want shutdown to continue. It
		// seems likely that we are going to hit this during normal operation
		// anyways if the process is already finished by the time we try to kill
		// it.
		sklog.Errorf("Got the following error when trying to kill the current running safe command: %v", err)
	}
	return nil
}

// cancelChromiumFetch cancels the in-progress Chromium fetch, if there is one.
// In the event that there is an in-progress fetch, the directories potentially
// containing checkout data will be wiped in order to ensure it is not left
// in a bad state that will affect future deployments.
func (s *ChromiumBuilderService) cancelChromiumFetch(dr directoryRemover) error {
	if !s.shuttingDown.Load() {
		return skerr.Fmt("cancelChromiumFetch() must only be called during shutdown.")
	}

	notCurrentlyFetching := s.chromiumFetchLock.TryLock()
	if notCurrentlyFetching {
		s.chromiumFetchLock.Unlock()
		return nil
	}

	s.currentProcessLock.Lock()
	defer s.currentProcessLock.Unlock()

	if s.currentProcess == nil {
		// See cancelSafeCommands for explanation on why we can safely do
		// nothing here.
		return nil
	}

	err := s.currentProcess.Kill()
	if err != nil {
		sklog.Errorf("Got the following error when trying to kill the Chromium fetch process: %v", err)
	}

	// We remove the parent directory since the stored path is to the src
	// directory, but gclient information is stored in the directory above that.
	// We want to wipe any gclient information as well so that the next
	// deployment will have a clean slate.
	err = dr(filepath.Dir(s.chromiumPath))
	if err != nil {
		sklog.Errorf(("Failed to delete in-progress Chromium checkout, future deployments will likely fail until " +
			"this is cleaned up. Error: %v"), err)
		return err
	}

	return nil
}
