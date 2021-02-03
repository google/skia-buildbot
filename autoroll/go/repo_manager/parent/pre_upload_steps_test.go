package parent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/autoroll/go/proto"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetPreUploadStep(t *testing.T) {
	unittest.SmallTest(t)

	// Test for existing steps.
	infraStep, err := GetPreUploadStep(proto.PreUploadStep_TRAIN_INFRA)
	assert.NoError(t, err)
	assert.NotNil(t, infraStep)
	flutterStep, err := GetPreUploadStep(proto.PreUploadStep_FLUTTER_LICENSE_SCRIPTS)
	assert.NoError(t, err)
	assert.NotNil(t, flutterStep)

	// Test for missing step.
	missingStep, err := GetPreUploadStep(proto.PreUploadStep(9999))
	assert.Error(t, err)
	assert.Equal(t, "No such pre-upload step: 9999", err.Error())
	assert.Nil(t, missingStep)
}

func TestFlutterLicenseScripts(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	pubErr := error(nil)
	dartErr := error(nil)
	gitErr := error(nil)

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		pubCmd := "get"
		dartCmd := "lib/main.dart --src ../../.. --out testing/out/licenses --golden testing/dir/ci/licenses_golden"
		if cmd.Name == "testing/third_party/dart/tools/sdks/dart-sdk/bin/pub" && strings.Join(cmd.Args, " ") == pubCmd {
			return pubErr
		} else if cmd.Name == "testing/third_party/dart/tools/sdks/dart-sdk/bin/dart" && strings.Join(cmd.Args, " ") == dartCmd {
			return dartErr
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
	err := FlutterLicenseScripts(ctx, nil, nil, "testing/dir")
	assert.NoError(t, err)

	// Now test for errors.
	pubErr = errors.New("pub error")
	err = FlutterLicenseScripts(ctx, nil, nil, "testing/dir")
	assert.Error(t, err)
	assert.Equal(t, "Error when running pub get: pub error; Stdout+Stderr:\n", err.Error())

	pubErr = error(nil)
	dartErr = errors.New("dart error")
	err = FlutterLicenseScripts(ctx, nil, nil, "testing/dir")
	assert.Error(t, err)
	assert.Equal(t, "Error when running dart license script: dart error", err.Error())

	pubErr = error(nil)
	dartErr = error(nil)
	err = FlutterLicenseScripts(ctx, nil, nil, "testing/dir")
	assert.NoError(t, err)
}
