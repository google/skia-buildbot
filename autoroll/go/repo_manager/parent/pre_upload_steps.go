package parent

/*
   This file contains canned pre-upload steps for RepoManagers to use.
*/

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/android_skia_checkout"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/go_install"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/metrics2"
)

var cipdRoot = path.Join(os.TempDir(), "cipd")

// PreUploadStep is a function to be run after the roll is performed but before
// a CL is uploaded. The string slice parameter is the set of environment
// variables which should be used by the pre-upload step. The http.Client should
// be authenticated. The string parameter is the absolute path to the directory
// of the parent repo.
type PreUploadStep func(context.Context, []string, *http.Client, string, *revision.Revision, *revision.Revision) error

var (
	// preUploadSteps is the registry of known PreUploadStep instances.
	preUploadSteps = map[config.PreUploadStep]PreUploadStep{
		config.PreUploadStep_ANGLE_CODE_GENERATION:               ANGLECodeGeneration,
		config.PreUploadStep_ANGLE_ROLL_CHROMIUM:                 ANGLERollChromium,
		config.PreUploadStep_GO_GENERATE_CIPD:                    GoGenerateCipd,
		config.PreUploadStep_TRAIN_INFRA:                         TrainInfra,
		config.PreUploadStep_FLUTTER_LICENSE_SCRIPTS:             FlutterLicenseScripts,
		config.PreUploadStep_FLUTTER_LICENSE_SCRIPTS_FOR_DART:    FlutterLicenseScriptsForDart,
		config.PreUploadStep_FLUTTER_LICENSE_SCRIPTS_FOR_FUCHSIA: FlutterLicenseScriptsForFuchsia,
		config.PreUploadStep_ANGLE_GN_TO_BP:                      AngleGnToBp,
		config.PreUploadStep_SKIA_GN_TO_BP:                       SkiaGnToBp,
		config.PreUploadStep_UPDATE_FLUTTER_DEPS_FOR_DART:        UpdateFlutterDepsForDart,
		config.PreUploadStep_VULKAN_DEPS_UPDATE_COMMIT_MESSAGE:   VulkanDepsUpdateCommitMessage,
		config.PreUploadStep_UPDATE_BORINGSSL:                    UpdateBoringSSL,
		config.PreUploadStep_CHROMIUM_ROLL_WEBGPU_CTS:            ChromiumRollWebGPUCTS,
	}
)

// GetPreUploadStep returns the PreUploadStep with the given name.
func GetPreUploadStep(s config.PreUploadStep) (PreUploadStep, error) {
	rv, ok := preUploadSteps[s]
	if !ok {
		return nil, fmt.Errorf("No such pre-upload step: %s", s)
	}
	return rv, nil
}

// GetPreUploadSteps returns the PreUploadSteps with the given names.
func GetPreUploadSteps(steps []config.PreUploadStep) ([]PreUploadStep, error) {
	rv := make([]PreUploadStep, 0, len(steps))
	for _, s := range steps {
		step, err := GetPreUploadStep(s)
		if err != nil {
			return nil, err
		}
		rv = append(rv, step)
	}
	return rv, nil
}

// AddPreUploadStepForTesting adds the given PreUploadStep to the global
// registry to be used for testing. Returns a unique name for the step. Not safe
// to be used concurrently with GetPreUploadStep(s).
func AddPreUploadStepForTesting(s PreUploadStep) config.PreUploadStep {
	id := config.PreUploadStep(len(preUploadSteps))
	preUploadSteps[id] = s
	// Add to the config package maps.
	stepName := fmt.Sprintf("Test-Step-%d", id)
	config.PreUploadStep_name[int32(id)] = stepName
	config.PreUploadStep_value[stepName] = int32(id)
	return id
}

// TrainInfra trains the infra expectations.
func TrainInfra(ctx context.Context, env []string, client *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	// TODO(borenet): Should we plumb through --local and --workdir?
	sklog.Info("Installing Go...")
	_, goEnv, err := go_install.EnsureGo(ctx, client, cipdRoot)
	if err != nil {
		return err
	}
	envMap := make(map[string]string, len(goEnv)+len(env))
	for _, v := range env {
		split := strings.SplitN(v, "=", 2)
		if len(split) != 2 {
			return fmt.Errorf("Invalid environment variable: %q", v)
		}
		envMap[split[0]] = split[1]
	}
	for k, v := range goEnv {
		if k == "PATH" {
			oldPath := envMap["PATH"]
			if oldPath != "" {
				v += ":" + oldPath
			}
		}
		envMap[k] = v
	}
	envSlice := make([]string, 0, len(envMap))
	for k, v := range envMap {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}
	sklog.Info("Training infra expectations...")
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Name: "make",
		Args: []string{"train"},
		Dir:  path.Join(parentRepoDir, "infra", "bots"),
		Env:  envSlice,
	}); err != nil {
		return err
	}
	return nil
}

// UpdateFlutterDepsForDart runs the create_updated_flutter_deps.py for Dart in
// Flutter.
// See https://bugs.chromium.org/p/skia/issues/detail?id=8437#c18 and
// https://bugs.chromium.org/p/skia/issues/detail?id=8437#c21 for context.
func UpdateFlutterDepsForDart(ctx context.Context, env []string, _ *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	sklog.Info("Running create_updated_flutter_deps.py and then \"gclient sync\"")

	scriptDir := filepath.Join(parentRepoDir, "..", "tools", "dart")
	scriptName := "create_updated_flutter_deps.py"
	if _, err := exec.RunCwd(ctx, scriptDir, "python", scriptName); err != nil {
		return fmt.Errorf("Error when running python %s: %s", scriptName, err)
	}

	// Do "gclient sync" after the script runs.
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  parentRepoDir,
		Env:  env,
		Name: "python",
		Args: []string{filepath.Join(parentRepoDir, "..", "..", "depot_tools", GClient), "sync", "--delete_unversioned_trees", "--force"},
	}); err != nil {
		return fmt.Errorf("Error when running \"gclient sync\" in %s: %s", parentRepoDir, err)
	}

	return nil
}

// FlutterLicenseScripts runs the flutter license scripts as described in
// https://bugs.chromium.org/p/skia/issues/detail?id=7730#c6 and in
// https://github.com/flutter/engine/blob/master/tools/licenses/README.md
func FlutterLicenseScripts(ctx context.Context, _ []string, _ *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	return flutterLicenseScripts(ctx, parentRepoDir, "licenses_skia")
}

// FlutterLicenseScriptsForFuchsia runs the flutter license scripts for fuchsia.
func FlutterLicenseScriptsForFuchsia(ctx context.Context, _ []string, _ *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	return flutterLicenseScripts(ctx, parentRepoDir, "licenses_fuchsia")
}

// FlutterLicenseScriptsForDart runs the flutter license scripts for dart.
func FlutterLicenseScriptsForDart(ctx context.Context, _ []string, _ *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	return flutterLicenseScripts(ctx, parentRepoDir, "licenses_third_party")
}

func flutterLicenseScripts(ctx context.Context, parentRepoDir, licenseFileName string) error {
	licenseScriptFailure := int64(1)
	defer func() {
		metrics2.GetInt64Metric("flutter_license_script_failure", nil).Update(licenseScriptFailure)
	}()

	sklog.Info("Running flutter license scripts.")
	binariesPath := filepath.Join(parentRepoDir, "..", "third_party", "dart", "tools", "sdks", "dart-sdk", "bin")

	// Step1: Run pub get.
	licenseToolsDir := filepath.Join(parentRepoDir, "tools", "licenses")
	pubBinary := filepath.Join(binariesPath, "pub")
	sklog.Info("Running put get.")
	if _, err := exec.RunCwd(ctx, licenseToolsDir, pubBinary, "get"); err != nil {
		return fmt.Errorf("Error when running pub get: %s", err)
	}

	// Step2: Clean out out/licenses. Things fail without this step.
	licensesOutDir := filepath.Join(parentRepoDir, "..", "out", "licenses")
	if err := os.RemoveAll(licensesOutDir); err != nil {
		return fmt.Errorf("Error when cleaning out/licenses: %s", err)
	}

	// Step3: Run dart license script to create new licenses.
	dartBinary := filepath.Join(binariesPath, "dart")
	licensesGoldenDir := filepath.Join(parentRepoDir, "ci", "licenses_golden")
	licenseCmd := []string{dartBinary, "lib/main.dart", "--src", "../../..", "--out", licensesOutDir, "--golden", licensesGoldenDir}
	sklog.Infof("Running %s", licenseCmd)
	if err := exec.Run(ctx, &exec.Command{
		Dir:            licenseToolsDir,
		Name:           licenseCmd[0],
		Args:           licenseCmd[1:],
		CombinedOutput: os.Stdout,
	}); err != nil {
		return fmt.Errorf("Error when running dart license script: %s", err)
	}

	// Step4: Check to see if the target license file was created in the out dir.
	//        It will be created if the hash changes.
	if _, err := os.Stat(filepath.Join(licensesOutDir, licenseFileName)); err == nil {
		sklog.Infof("Found %s", licenseFileName)

		// Step5: Copy from out dir to goldens dir. This is required for updating
		//        the release file in sky_engine/LICENSE.
		if _, err := exec.RunCwd(ctx, licenseToolsDir, "cp", filepath.Join(licensesOutDir, licenseFileName), filepath.Join(licensesGoldenDir, licenseFileName)); err != nil {
			return fmt.Errorf("Error when copying %s from out to golden dir: %s", licenseFileName, err)
		}
		// Step6: Capture diff of licenses_golden/${licenseFileName}.
		licensesDiffOutput, err := git.GitDir(licenseToolsDir).Git(ctx, "diff", "--no-ext-diff", filepath.Join(licensesGoldenDir, licenseFileName))
		if err != nil {
			return fmt.Errorf("Error when seeing diff of golden %s: %s", licenseFileName, err)
		}
		sklog.Infof("The licenses diff output is:\n%s", licensesDiffOutput)

		// Step7: Update sky_engine/LICENSE. It should always be run
		// according to https://github.com/flutter/engine/pull/4959#issuecomment-380222322
		updateLicenseCmd := []string{dartBinary, "lib/main.dart", "--release", "--src", "../../..", "--out", licensesOutDir}
		sklog.Infof("Running %s", updateLicenseCmd)
		releasesLicensePath := filepath.Join(parentRepoDir, "sky", "packages", "sky_engine", "LICENSE")
		outFile, err := os.Create(releasesLicensePath)
		if err != nil {
			return fmt.Errorf("Could not open %s: %s", releasesLicensePath, err)
		}
		if err := exec.Run(ctx, &exec.Command{
			Dir:    licenseToolsDir,
			Name:   updateLicenseCmd[0],
			Args:   updateLicenseCmd[1:],
			Stdout: outFile,
			Stderr: os.Stderr,
		}); err != nil {
			return fmt.Errorf("Error when running dart license script: %s", err)
		}
	}

	sklog.Info("Done running flutter license scripts.")
	licenseScriptFailure = 0
	return nil
}

// GoGenerateCipd runs "go generate" in go/cipd.
func GoGenerateCipd(ctx context.Context, _ []string, client *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	// TODO(borenet): Should we plumb through --local and --workdir?
	sklog.Info("Installing Go...")
	goExc, goEnv, err := go_install.EnsureGo(ctx, client, cipdRoot)
	if err != nil {
		return err
	}

	// Also install the protoc asset. Use a different CIPD root dir to
	// prevent conflicts with the Go packages.
	protocRoot := path.Join(os.TempDir(), "cipd_protoc")
	if err := cipd.Ensure(ctx, client, protocRoot, cipd.PkgProtoc); err != nil {
		return err
	}

	envSlice := make([]string, 0, len(goEnv))
	for k, v := range goEnv {
		if k == "PATH" {
			// Construct PATH by adding protoc and the required PATH
			// entries for Go on to the existing PATH.
			v = path.Join(protocRoot, cipd.PkgProtoc.Path, "bin") + ":" + v + ":" + os.Getenv("PATH")
		}
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}

	// Run go generate.
	generateDir := filepath.Join(parentRepoDir, "go", "cipd")
	sklog.Infof("Running 'go generate' in %s", generateDir)
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Name: goExc,
		Args: []string{"generate"},
		Dir:  generateDir,
		Env:  envSlice,
	}); err != nil {
		return err
	}
	return nil
}

// AngleGnToBp runs Angle's scripts to roll into Android.
func AngleGnToBp(ctx context.Context, env []string, client *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	angleDir := filepath.Join(parentRepoDir, "external", "angle")
	if _, scriptErr := exec.RunCwd(ctx, angleDir, "./scripts/roll_aosp.sh"); scriptErr != nil {
		return fmt.Errorf("./scripts/roll_aosp.sh error: %s", scriptErr)
	}
	return nil
}

// SkiaGnToBp runs Skia's gn_to_bp.py script to roll into Android.
func SkiaGnToBp(ctx context.Context, env []string, client *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	skiaDir := filepath.Join(parentRepoDir, "external", "skia")
	if err := android_skia_checkout.RunGnToBp(ctx, skiaDir); err != nil {
		return fmt.Errorf("Error when running gn_to_bp: %s", err)
	}
	for _, genFile := range android_skia_checkout.FilesGeneratedByGnToGp {
		if _, err := git.GitDir(skiaDir).Git(ctx, "add", genFile); err != nil {
			return fmt.Errorf("Could not git add %s: %s", genFile, err)
		}
	}
	if _, err := git.GitDir(skiaDir).Git(ctx, "add", android_skia_checkout.LibGifRelPath); err != nil {
		return fmt.Errorf("Could not git add %s: %s", android_skia_checkout.LibGifRelPath, err)
	}
	return nil
}

// ANGLECodeGeneration runs the ANGLE code generation script.
func ANGLECodeGeneration(ctx context.Context, env []string, client *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	sklog.Info("Running code generation script...")
	out, err := exec.RunCommand(ctx, &exec.Command{
		Name: "python",
		Args: []string{filepath.Join("scripts", "run_code_generation.py")},
		Dir:  parentRepoDir,
		Env:  env,
	})
	sklog.Infof("Output from run_code_generation:\n%s", out)
	return skerr.Wrap(err)
}

// ANGLERollChromium runs the ANGLE roll_chromium_deps.py script.
func ANGLERollChromium(ctx context.Context, env []string, _ *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	sklog.Info("Running roll_chromium_deps script...")
	contents, err := ioutil.ReadFile(filepath.Join(parentRepoDir, deps_parser.DepsFileName))
	if err != nil {
		return skerr.Wrap(err)
	}
	dep, err := deps_parser.GetDep(string(contents), common.REPO_CHROMIUM)
	if err != nil {
		return skerr.Wrap(err)
	}
	out, err := exec.RunCommand(ctx, &exec.Command{
		Name: "python",
		Args: []string{filepath.Join("scripts", "roll_chromium_deps.py"), fmt.Sprintf("--revision=%s", dep.Version), "--ignore-unclean-workdir", "--autoroll", "-v"},
		Dir:  parentRepoDir,
		Env:  env,
	})
	sklog.Infof("Output from roll_chromium_deps.py:\n%s", out)
	return skerr.Wrap(err)
}

// VulkanDepsUpdateCommitMessage runs a script to produce a more usful commit message.
func VulkanDepsUpdateCommitMessage(ctx context.Context, env []string, _ *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	sklog.Info("Running update-commit-message script...")
	out, err := exec.RunCommand(ctx, &exec.Command{
		Name: "python3",
		Args: []string{filepath.Join("third_party", "vulkan-deps", "update-commit-message.py"), "--old-revision", from.Id},
		Dir:  parentRepoDir,
		Env:  env,
	})
	sklog.Infof("Output from update-commit-message.py:\n%s", out)
	return skerr.Wrap(err)
}

// UpdateBoringSSL runs a script to generate files for BoringSSL.
func UpdateBoringSSL(ctx context.Context, env []string, client *http.Client, parentRepoDir string, _, _ *revision.Revision) error {
	sklog.Info("Installing Go...")
	_, goEnv, err := go_install.EnsureGo(ctx, client, cipdRoot)
	if err != nil {
		return err
	}
	envSlice := make([]string, 0, len(goEnv)+len(env))
	for _, kv := range env {
		split := strings.SplitN(kv, "=", 2)
		if len(split) == 2 && split[0] == "PATH" {
			kv = fmt.Sprintf("PATH=%s", goEnv["PATH"]+":"+split[1])
		}
		envSlice = append(envSlice, kv)
	}
	for k, v := range goEnv {
		if k != "PATH" {
			envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
		}
	}
	sklog.Info("Running update_boringssl script...")
	out, err := exec.RunCommand(ctx, &exec.Command{
		Name: "python3",
		Args: []string{filepath.Join("third_party", "boringssl", "update_boringssl.py")},
		Dir:  parentRepoDir,
		Env:  envSlice,
	})
	sklog.Infof("Output from update_boringssl.py:\n%s", out)
	return skerr.Wrap(err)
}

// ChromiumRollWebGPUCTS updates the list of Typescript sources used to compile
// the CTS, and it regenerates the list of WPT variants.
func ChromiumRollWebGPUCTS(ctx context.Context, env []string, client *http.Client, parentRepoDir string, from *revision.Revision, to *revision.Revision) error {
	sklog.Info("Running gen_ts_dep_lists.py...")
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Name: "python3",
		Args: []string{filepath.Join("third_party", "webgpu-cts", "scripts", "gen_ts_dep_lists.py")},
		Dir:  parentRepoDir,
		Env:  env,
	}); err != nil {
		// Log, but don't return the error. The Chromium presubmit will fail if this does not succeed.
		sklog.Errorf("%s", err)
	}

	sklog.Info("Running regenerate_internal_cts_html.py...")
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Name: "vpython",
		Args: []string{filepath.Join("third_party", "webgpu-cts", "scripts", "regenerate_internal_cts_html.py")},
		Dir:  parentRepoDir,
		Env:  env,
	}); err != nil {
		// Log, but don't return the error. The Chromium presubmit will fail if this does not succeed.
		sklog.Errorf("%s", err)
	}
	return nil
}
