package parent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetPreUploadStep(t *testing.T) {

	// Test for existing steps.
	infraStep, err := GetPreUploadStep(config.PreUploadStep_TRAIN_INFRA)
	assert.NoError(t, err)
	assert.NotNil(t, infraStep)
	flutterStep, err := GetPreUploadStep(config.PreUploadStep_FLUTTER_LICENSE_SCRIPTS)
	assert.NoError(t, err)
	assert.NotNil(t, flutterStep)

	// Test for missing step.
	missingStep, err := GetPreUploadStep(config.PreUploadStep(9999))
	assert.Error(t, err)
	assert.Equal(t, "No such pre-upload step: 9999", err.Error())
	assert.Nil(t, missingStep)
}

func TestFlutterLicenseScripts(t *testing.T) {
	unittest.LinuxOnlyTest(t)

	wd, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	pubErr := error(nil)
	mainDartErr := error(nil)
	gitErr := error(nil)

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		dartBinary := fmt.Sprintf("%s/engine/src/flutter/third_party/dart/tools/sdks/dart-sdk/bin/dart", wd)
		pubDartCmd := "pub get"
		mainDartCmd := fmt.Sprintf("--interpret_irregexp lib/main.dart --src ../../.. --out %s/engine/src/out/licenses --golden %s/engine/src/flutter/ci/licenses_golden", wd, wd)
		releaseDartCmd := fmt.Sprintf("--interpret_irregexp lib/main.dart --release --src ../../.. --quiet --out %s/engine/src/out/licenses", wd)
		cmdArgs := strings.Join(cmd.Args, " ")
		if cmd.Name == dartBinary && cmdArgs == pubDartCmd {
			return pubErr
		} else if cmd.Name == dartBinary && (cmdArgs == mainDartCmd || cmdArgs == releaseDartCmd) {
			return mainDartErr
		} else if strings.Contains(cmd.Name, "git") {
			expectedCheckoutArgs := "checkout -- pubspec.lock"
			expectedCommitArgs := "commit -a --amend --no-edit"
			if strings.Join(cmd.Args, " ") == expectedCheckoutArgs || strings.Join(cmd.Args, " ") == expectedCommitArgs {
				return gitErr
			}
		}
		return exec.DefaultRun(ctx, cmd)
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)

	// No errors should be throw.
	err = FlutterLicenseScripts(ctx, nil, nil, wd, nil, nil)
	assert.NoError(t, err)

	// Now test for errors.
	pubErr = errors.New("pub error")
	err = FlutterLicenseScripts(ctx, nil, nil, wd, nil, nil)
	assert.Error(t, err)
	assert.Equal(t, "Error when running pub get: pub error; Stdout+Stderr:\n", err.Error())

	pubErr = error(nil)
	mainDartErr = errors.New("dart error")
	err = FlutterLicenseScripts(ctx, nil, nil, wd, nil, nil)
	assert.Error(t, err)
	assert.Equal(t, "Error when running dart license script: dart error", err.Error())

	pubErr = error(nil)
	mainDartErr = error(nil)
	err = FlutterLicenseScripts(ctx, nil, nil, wd, nil, nil)
	assert.NoError(t, err)
}

func TestPlaceholders_String(t *testing.T) {
	p := Placeholders{
		ParentRepoDir: "/path/to/repo",
		RollingFromID: "abc",
		RollingToID:   "def",
	}
	require.Equal(t, "/path/to/repo/foo", p.String("${parent_dir}/foo"))
	require.Equal(t, "rolling from abc to def", p.String("rolling from ${rolling_from} to ${rolling_to}"))
	require.Equal(t, "no placeholders", p.String("no placeholders"))
	require.Equal(t, "${unknown}", p.String("${unknown}"))
}

func TestPlaceholders_Strings(t *testing.T) {
	p := Placeholders{
		ParentRepoDir: "/path/to/repo",
		RollingFromID: "abc",
		RollingToID:   "def",
	}
	input := []string{
		"${parent_dir}/foo",
		"rolling from ${rolling_from} to ${rolling_to}",
		"no placeholders",
	}
	expected := []string{
		"/path/to/repo/foo",
		"rolling from abc to def",
		"no placeholders",
	}
	require.Equal(t, expected, p.Strings(input))
}

func TestPlaceholders_CIPDPackage(t *testing.T) {
	p := Placeholders{
		ParentRepoDir: "/path/to/repo",
		RollingFromID: "abc",
		RollingToID:   "def",
	}

	// Test with placeholder substitution.
	pkgCfg := &config.PreUploadCIPDPackageConfig{
		Name:    "some/package/${rolling_to}",
		Path:    "${parent_dir}/cipd_pkgs",
		Version: "version:${rolling_from}",
	}
	expected := &cipd.Package{
		Name:    "some/package/def",
		Path:    "/path/to/repo/cipd_pkgs",
		Version: "version:abc",
	}
	actual, err := p.CIPDPackage(pkgCfg)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	// Test with use_pinned_version.
	pkgCfg = &config.PreUploadCIPDPackageConfig{
		Name:    "skia/bots/go",
		Path:    ".",
		Version: "${use_pinned_version}",
	}
	_, err = p.CIPDPackage(pkgCfg)
	require.NoError(t, err)

	// Test with use_pinned_version for a package that doesn't exist.
	pkgCfg = &config.PreUploadCIPDPackageConfig{
		Name:    "does/not/exist",
		Path:    ".",
		Version: "${use_pinned_version}",
	}
	_, err = p.CIPDPackage(pkgCfg)
	require.Error(t, err)
}

func TestPlaceholders_CIPDPackages(t *testing.T) {
	p := Placeholders{
		ParentRepoDir: "/path/to/repo",
		RollingFromID: "abc",
		RollingToID:   "def",
	}
	pkgCfg1 := &config.PreUploadCIPDPackageConfig{
		Name:    "some/package/${rolling_to}",
		Path:    "${parent_dir}/cipd_pkgs",
		Version: "version:${rolling_from}",
	}
	pkgCfg2 := &config.PreUploadCIPDPackageConfig{
		Name:    "another/package",
		Path:    ".",
		Version: "stable",
	}
	pkgs, err := p.CIPDPackages([]*config.PreUploadCIPDPackageConfig{pkgCfg1, pkgCfg2})
	require.NoError(t, err)
	require.Len(t, pkgs, 2)
	require.Equal(t, "some/package/def", pkgs[0].Name)
	require.Equal(t, "/path/to/repo/cipd_pkgs", pkgs[0].Path)
	require.Equal(t, "version:abc", pkgs[0].Version)
	require.Equal(t, "another/package", pkgs[1].Name)
	require.Equal(t, ".", pkgs[1].Path)
	require.Equal(t, "stable", pkgs[1].Version)
}

func TestPlaceholders_Command(t *testing.T) {
	p := Placeholders{
		ParentRepoDir: "/path/to/repo",
		RollingFromID: "abc",
		RollingToID:   "def",
	}
	cmd, err := p.Command(t.Context(), []string{"echo", "hello", "${rolling_to}"}, "${parent_dir}", []string{"VAR=${rolling_from}"})
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(cmd.Name, "echo"))
	require.Equal(t, []string{"hello", "def"}, cmd.Args)
	require.Equal(t, "/path/to/repo", cmd.Dir)
	require.Equal(t, []string{"VAR=abc"}, cmd.Env)
}

func TestPlaceholders_PreUploadConfig(t *testing.T) {
	p := Placeholders{
		ParentRepoDir: "/path/to/repo",
		RollingFromID: "abc",
		RollingToID:   "def",
	}
	cfg := &config.PreUploadConfig{
		CipdPackage: []*config.PreUploadCIPDPackageConfig{
			{
				Name:    "some/package/${rolling_to}",
				Path:    "${parent_dir}/cipd_pkgs",
				Version: "version:${rolling_from}",
			},
		},
		Command: []*config.PreUploadCommandConfig{
			{
				Command: "echo hello ${rolling_to}",
				Cwd:     "${parent_dir}",
				Env:     []string{"VAR=${rolling_from}"},
			},
		},
	}
	cipdPkgs, cmds, err := p.PreUploadConfig(t.Context(), []string{}, cfg)
	require.NoError(t, err)
	require.Len(t, cipdPkgs, 1)
	require.Equal(t, "some/package/def", cipdPkgs[0].Name)
	require.Len(t, cmds, 1)
	require.True(t, strings.HasSuffix(cmds[0].Name, "echo"))
	require.Equal(t, []string{"hello", "def"}, cmds[0].Args)
	require.Equal(t, "/path/to/repo", cmds[0].Dir)
	require.Equal(t, []string{"VAR=abc"}, cmds[0].Env)
}
