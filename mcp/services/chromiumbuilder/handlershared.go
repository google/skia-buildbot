package chromiumbuilder

// Code used by multiple tool handlers of ChromiumBuilderService

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
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

var (
	InfraConfigSubdirectory     string = filepath.Join("infra", "config")
	BuilderStarlarkSubdirectory string = filepath.Join(InfraConfigSubdirectory, "subprojects", "chromium", "ci")
)

const (
	GerritUrlPrefix string = "https://chromium-review.googlesource.com"
)

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
