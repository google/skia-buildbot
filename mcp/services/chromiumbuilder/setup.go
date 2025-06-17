package chromiumbuilder

// Code related to ChromiumBuilderService.Init().

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vfs"
)

const (
	ArgDepotToolsPath string = "depot_tools_path"
	ArgChromiumPath   string = "chromium_path"
)

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
