package chromiumbuilder

// TODO(bsheedy): Refactor things so that Init and the handler just call another
// function that takes the relevant interfaces to better facilitate testing.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
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
)

type checkoutFactory = func(context.Context, string, string) (git.Checkout, error)

func realCheckoutFactory(ctx context.Context, repoUrl, workdir string) (git.Checkout, error) {
	return git.NewCheckout(ctx, repoUrl, workdir)
}

// ChromiumBuilderService is an MCP service which is capable of generating CLs
// to add new LUCI builders to chromium/src.
type ChromiumBuilderService struct {
	chromiumPath       string
	depotToolsPath     string
	chromiumCheckout   git.Checkout
	depotToolsCheckout git.Checkout
}

// Init initializes the service with the provided arguments. serviceArgs is
// expected to be a comma-separated list of key-value pairs in the form
// key=value.
func (s *ChromiumBuilderService) Init(serviceArgs string) error {
	ctx := context.Background()
	err := s.parseServiceArgs(serviceArgs)
	if err != nil {
		return err
	}
	sklog.Errorf("Parsed args %v", s)

	err = s.handleDepotToolsSetup(ctx, vfs.Local("/"), realCheckoutFactory)
	if err != nil {
		return err
	}

	err = s.handleChromiumSetup(ctx, vfs.Local("/"), realCheckoutFactory)
	if err != nil {
		return err
	}

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

// handleDepotTools ensures that a depot_tools checkout is available at the
// stored path.
func (s *ChromiumBuilderService) handleDepotToolsSetup(ctx context.Context, fs vfs.FS, factory checkoutFactory) error {
	// Check if depot_tools path exists.
	depotToolsDir, err := fs.Open(ctx, s.depotToolsPath)
	if err != nil {
		return s.handleMissingDepotToolsCheckout(ctx, fs, factory)
	}
	defer depotToolsDir.Close(ctx)

	return s.handleExistingDepotToolsCheckout(ctx, fs, factory)
}

// handleMissingDepotToolsCheckout sets up a new depot_tools checkout at the
// stored path to handle the case where there is not an existing checkout.
func (s *ChromiumBuilderService) handleMissingDepotToolsCheckout(ctx context.Context, fs vfs.FS, factory checkoutFactory) error {
	return skerr.Fmt("depot_tools path %s does not currently exist. This service does not yet support automatically getting a copy",
		s.depotToolsPath)
}

// handleExistingDepotToolsCheckout ensures that an existing depot_tools
// checkout is valid and up to date.
func (s *ChromiumBuilderService) handleExistingDepotToolsCheckout(ctx context.Context, fs vfs.FS, factory checkoutFactory) error {
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

	// Obtain a re-usable checkout and ensure it is up to date.
	s.depotToolsCheckout, err = factory(ctx, DepotToolsUrl, filepath.Dir(s.depotToolsPath))
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
func (s *ChromiumBuilderService) handleChromiumSetup(ctx context.Context, fs vfs.FS, factory checkoutFactory) error {
	// Check if the Chromium path exists.
	chromiumDir, err := fs.Open(ctx, s.chromiumPath)
	if err != nil {
		return s.handleMissingChromiumCheckout(ctx, fs, factory)
	}
	defer chromiumDir.Close(ctx)

	return s.handleExistingChromiumCheckout(ctx, fs, factory)
}

// handleMissingChromiumCheckout sets up a new Chromium checkout at the stored
// path to handle the case where there is not an existing checkout.
func (s *ChromiumBuilderService) handleMissingChromiumCheckout(ctx context.Context, fs vfs.FS, factory checkoutFactory) error {
	return skerr.Fmt("Chromium path %s does not currently exist. This service does not yet support automatically getting a copy",
		s.chromiumPath)
}

// handleExistingChromiumCheckout ensures that the existing Chromium checkout is
// valid and up to date.
func (s *ChromiumBuilderService) handleExistingChromiumCheckout(ctx context.Context, fs vfs.FS, factory checkoutFactory) error {
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
	s.chromiumCheckout, err = factory(ctx, ChromiumUrl, filepath.Dir(s.chromiumPath))
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
	sklog.Infof("Calling GetTools with service %v", s)
	return []common.Tool{
		{
			Name:        "create_ci_combined_builder",
			Description: "Creates a combined compile/test LUCI builder for Chromium",
			Arguments: []common.ToolArgument{
				{
					Name:        "builder_group",
					Description: "The builder group the builder will be a part of, e.g. chromium.fyi",
					Required:    true,
				},
				{
					Name:        "builder_name",
					Description: "The name of the new builder",
					Required:    true,
				},
				{
					Name:        "builder_description",
					Description: "A human-readable description of the builder. Supports HTML tags.",
					Required:    true,
				},
				{
					Name:        "contact_team_email",
					Description: "A valid email address for the team that will own the new builder.",
					Required:    true,
				},
				{
					Name: "console_view_category",
					Description: ("One or more categories used to group similar builders together. Each category is separated " +
						"by '|', with each level being progressively more nested. For example 'Linux|Asan' will " +
						"group the builder first with all other 'Linux' machines, then with all 'Asan' machines " +
						"are under 'Linux'."),
					Required: true,
				},
				{
					Name:        "target_os",
					Description: "The OS the builder is compiling for, e.g. 'Linux' or 'Android'",
					Required:    true,
					EnumValues:  []string{TargetOsAndroid, TargetOsLinux, TargetOsMac, TargetOsWin},
				},
				{
					Name:        "target_arch",
					Description: "The architecture the builder is compiling for, e.g. 'Arm'",
					Required:    true,
					EnumValues:  []string{TargetArchArm, TargetArchIntel},
				},
				{
					Name:         "target_bits",
					Description:  "The target bitness the builder is compiling for, e.g. 32 or 64",
					Required:     true,
					ArgumentType: common.NumberArgument,
				},
				{
					Name:        "build_config",
					Description: "The target config the builder is compiling for, e.g. 'Debug' or 'Release'",
					Required:    true,
					EnumValues:  []string{BuilderConfigDebug, BuilderConfigRelease},
				},
				{
					Name: "gn_args",
					Description: ("The GN arg configs for the builder to use when compiling. " +
						"Can be any number of valid configs from " +
						"https://source.chromium.org/chromium/chromium/src/+/main:infra/config/gn_args/gn_args.star"),
					Required:     true,
					ArgumentType: common.ArrayArgument,
					ArraySchema:  map[string]any{"type": "string"},
				},
				{
					Name: "tests",
					Description: ("The names of individual tests or bundles for the builder to compile and run. " +
						"Can be any number of individual tests from " +
						"https://source.chromium.org/chromium/chromium/src/+/main:infra/config/targets/tests.star " +
						"or bundles from " +
						"https://source.chromium.org/chromium/chromium/src/+/main:infra/config/targets/bundles.star"),
					Required:     true,
					ArgumentType: common.ArrayArgument,
					ArraySchema:  map[string]any{"type": "string"},
				},
				{
					Name: "swarming_dimensions",
					Description: ("The names of Swarming mixins to use when triggering tests. " +
						"Can be any number of mixins from " +
						"https://source.chromium.org/chromium/chromium/src/+/main:infra/config/targets/mixins.star"),
					Required:     true,
					ArgumentType: common.ArrayArgument,
					ArraySchema:  map[string]any{"type": "string"},
				},
			},
			Handler: s.createCiCombinedBuilderHandler,
		},
	}
}

// createCiCombinedBuilderHandler is the handler the create_ci_combined_builder
// tool, which creates a combined compile + test builder in Chromium and uploads
// the resulting CL.
func (s *ChromiumBuilderService) createCiCombinedBuilderHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sklog.Infof("calling handler with data %v", s)
	inputs, err := extractCiCombinedBuilderInputs(request)
	if err != nil {
		sklog.Errorf("Error extracting inputs: %v", err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = s.updateCheckouts(ctx)
	if err != nil {
		sklog.Errorf("Error updating checkouts: %v", err)
		return mcp.NewToolResultError("Server had an internal error updating checkout. This is not actionable by the client."), nil
	}

	_, err = s.switchToTemporaryBranch(ctx)
	if err != nil {
		sklog.Errorf("Error checking out temporary branch: %v", err)
		return mcp.NewToolResultError("Server failed to check out temporary branch. This is not actionable by the client."), nil
	}
	// TODO(bsheedy): Clean up the created branch

	err = s.addNewBuilder(ctx, inputs, vfs.Local("/"))
	if err != nil {
		sklog.Errorf("Error adding new builder: %v", err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = s.formatStarlark(ctx)
	if err != nil {
		sklog.Errorf("Error formatting Starlark: %v", err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = s.generateFilesFromStarlark(ctx)
	if err != nil {
		sklog.Errorf("Error generating files from Starlark: %v", err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = s.addAndCommitFiles(ctx, inputs)
	if err != nil {
		sklog.Errorf("Error adding and committing files: %v", err)
		return mcp.NewToolResultError("Server failed to commit changes for upload. This is not actionable by the client."), nil
	}

	clLink, err := s.uploadCl(ctx)
	if err != nil {
		sklog.Errorf("Error uploading CL: %v", err)
		return mcp.NewToolResultError("Server failed to upload generated CL to Gerrit. This is not actionable by the client."), nil
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

// updateCheckouts ensures that both depot_tools and Chromium are up to date
// with origin/main.
func (s ChromiumBuilderService) updateCheckouts(ctx context.Context) error {
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
func (s ChromiumBuilderService) updateDepotToolsCheckout(ctx context.Context) error {
	err := s.depotToolsCheckout.Update(ctx)
	if err != nil {
		return err
	}
	return nil
}

// updateChromiumCheckout ensures that Chromium is up to date with origin/main.
// This does *not* interact with gclient, as DEPS should not be needed for
// interacting with //infra/config.
func (s ChromiumBuilderService) updateChromiumCheckout(ctx context.Context) error {
	err := s.chromiumCheckout.Update(ctx)
	if err != nil {
		return err
	}

	return nil
}

// switchToTemporaryBranch creates a uniquely named branch and switches to it,
// returning the new branch name.
func (s ChromiumBuilderService) switchToTemporaryBranch(ctx context.Context) (string, error) {
	branchName := fmt.Sprintf("%d", time.Now().UnixMilli())
	_, err := s.chromiumCheckout.Git(ctx, "checkout", "-b", branchName)
	if err != nil {
		return "", err
	}

	return branchName, nil
}

// addNewBuilder goes through all the steps necessary to add a new builder
// definition to the relevant Starlark file on disk.
func (s ChromiumBuilderService) addNewBuilder(ctx context.Context, inputs ciCombinedBuilderInputs, fs vfs.FS) error {
	sklog.Errorf("Adding new builder with inputs %v", inputs)

	starlarkFilename := fmt.Sprintf("%s.star", inputs.builderGroup)
	starlarkFilepath := filepath.Join(s.chromiumPath, BuilderStarlarkSubdirectory, starlarkFilename)
	starlarkFile, err := fs.Open(ctx, starlarkFilepath)
	if err != nil {
		return err
	}
	defer starlarkFile.Close(ctx)

	buildConfig, err := determineBuildConfig(inputs)
	if err != nil {
		return err
	}
	targetArch, err := determineTargetArch(inputs)
	if err != nil {
		return err
	}
	targetOs, err := determineTargetOs(inputs)
	if err != nil {
		return err
	}
	additionalConfigs, err := determineAdditionalConfigs(inputs)
	if err != nil {
		return err
	}
	gnArgs, err := determineGnArgs(inputs)
	if err != nil {
		return err
	}
	tests, err := determineTests(inputs)
	if err != nil {
		return err
	}
	swarmingDimensions, err := determineSwarmingDimensions(inputs)
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

// determineBuildConfig translates the string contained within
// inputs.buildConfig to the corresponding Starlark constant.
func determineBuildConfig(inputs ciCombinedBuilderInputs) (string, error) {
	switch inputs.buildConfig {
	case BuilderConfigDebug:
		return "builder_config.build_config.DEBUG", nil
	case BuilderConfigRelease:
		return "builder_config.build_config.RELEASE", nil
	default:
		return "", skerr.Fmt("Unhandled builder config %s", inputs.buildConfig)
	}
}

// determineTargetArch translates the string contained within
// inputs.targetArch to the corresponding Starlark constant.
func determineTargetArch(inputs ciCombinedBuilderInputs) (string, error) {
	switch inputs.targetArch {
	case TargetArchArm:
		return "builder_config.target_arch.ARM", nil
	case TargetArchIntel:
		return "builder_config.target_arch.INTEL", nil
	default:
		return "", skerr.Fmt("Unhandled target architecture %s", inputs.targetArch)
	}
}

// determineTargetOs translates the string contained within
// inputs.targetOs to the corresponding Starlark constant.
func determineTargetOs(inputs ciCombinedBuilderInputs) (string, error) {
	switch inputs.targetOs {
	case TargetOsAndroid:
		return "builder_config.target_platform.ANDROID", nil
	case TargetOsLinux:
		return "builder_config.target_platform.LINUX", nil
	case TargetOsMac:
		return "builder_config.target_platform.MAC", nil
	case TargetOsWin:
		return "builder_config.target_platform.WIN", nil
	default:
		return "", skerr.Fmt("Unhandled target OS %s", inputs.targetOs)
	}
}

// determineAdditionalConfigs returns a string containing any additional
// Starlark configs that should be added to the builder's builder_spec entry
// based on the contents of the provided inputs.
func determineAdditionalConfigs(inputs ciCombinedBuilderInputs) (string, error) {
	additionalConfigs := ""
	if inputs.targetOs == TargetOsAndroid {
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

// determineGnArgs translates the string slice contained within inputs.gnArgs
// to a string usable as the contents for a Starlark list.
func determineGnArgs(inputs ciCombinedBuilderInputs) (string, error) {
	return quoteAndCommaSeparate(inputs.gnArgs), nil
}

// determineTests translates the string slice contained within inputs.tests
// to a string usable as the contents for a Starlark list.
func determineTests(inputs ciCombinedBuilderInputs) (string, error) {
	return quoteAndCommaSeparate(inputs.tests), nil
}

// determineSwarmingDimensions translates the string slice contained within
// inputs.swarmingDimensions to a string usable as the contents for a Starlark
// list.
func determineSwarmingDimensions(inputs ciCombinedBuilderInputs) (string, error) {
	return quoteAndCommaSeparate(inputs.swarmingDimensions), nil
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

// formatStarlark runs lucicfg to format the Starlark files contained within
// the Chromium checkout.
func (s ChromiumBuilderService) formatStarlark(ctx context.Context) error {
	lucicfgPath := filepath.Join(s.depotToolsPath, "lucicfg")
	infraConfigPath := filepath.Join(s.chromiumPath, InfraConfigSubdirectory)

	output := bytes.Buffer{}
	err := exec.Run(ctx, &exec.Command{
		Name:           lucicfgPath,
		Args:           []string{"fmt", infraConfigPath},
		CombinedOutput: &output,
	})
	if err != nil {
		return skerr.Fmt("Failed to format Starlark. Original error: %v Stdout: %s", err, output.String())
	}

	return nil
}

// generateFilesFromStarlark runs Chromium's main Starlark file to generate any
// JSON/pyl/etc. files based on any changes to Starlark files.
func (s ChromiumBuilderService) generateFilesFromStarlark(ctx context.Context) error {
	starlarkMainPath := filepath.Join(s.chromiumPath, InfraConfigSubdirectory, "main.star")

	output := bytes.Buffer{}
	err := exec.Run(ctx, &exec.Command{
		Name:           starlarkMainPath,
		Args:           []string{},
		CombinedOutput: &output,
	})
	if err != nil {
		return skerr.Fmt("Failed to generate files from Starlark. Original error: %v Stdout: %s", err, output.String())
	}

	return nil
}

// addAndCommitFiles adds all files under Chromium's //infra/config directory to
// git then commits them.
func (s ChromiumBuilderService) addAndCommitFiles(ctx context.Context, inputs ciCombinedBuilderInputs) error {
	infraConfigPath := filepath.Join(s.chromiumPath, InfraConfigSubdirectory)
	_, err := s.chromiumCheckout.Git(ctx, "add", infraConfigPath)
	if err != nil {
		return err
	}

	clTitle := fmt.Sprintf("Add new builder %s", inputs.builderName)
	clDescription := fmt.Sprintf("Adds a new builder %s in the %s group. This CL was auto-generated.", inputs.builderName, inputs.builderGroup)
	_, err = s.chromiumCheckout.Git(ctx, "commit", "-m", clTitle, "-m", clDescription)
	if err != nil {
		return err
	}

	return nil
}

// uploadCl uploads committed changes to Gerrit and returns the uploaded CL's
// link.
func (s ChromiumBuilderService) uploadCl(ctx context.Context) (string, error) {
	gitClPath := filepath.Join(s.depotToolsPath, "git_cl.py")

	// TODO(bsheedy): Figure out what the best way to handle this on k8s is. It
	// works locally, but is likely relying on depot_tools already being in PATH.
	// Setting PATH manually via the Env argument of Run breaks authentication
	// since git_cl doesn't have access to the SSO information anymore.
	output := bytes.Buffer{}
	err := exec.Run(ctx, &exec.Command{
		Name:           gitClPath,
		Args:           []string{"upload", "--skip-title", "--bypass-hooks", "--force"},
		Dir:            s.chromiumPath,
		CombinedOutput: &output,
		Timeout:        5 * time.Minute,
	})
	if err != nil {
		return "", err
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

func (s *ChromiumBuilderService) Shutdown() error {
	// TODO(bsheedy): Implement the shutdown process.
	sklog.Infof("Shutting down Chromium Builder service")
	return nil
}
