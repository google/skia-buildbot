package chromiumbuilder

// Code related to the create_ci_combined_builder tool of ChromiumBuilderService.

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vfs"
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
