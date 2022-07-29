// Package genpromcrd implements all the functionality for the genpromcrd
// command line application.
package genpromcrd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

var alertTarget = AlertTarget{
	AppGroup:  "perf",
	Namespace: "perfns",
	Directory: "/some/sub-directory/in/the/git/checkout/",
}

func TestAlertTarget_TargetFilename_Success(t *testing.T) {
	unittest.SmallTest(t)
	require.Equal(t, "/some/sub-directory/in/the/git/checkout/perf_perfns_appgroup_alerts.yml", alertTarget.TargetFilename())
}

func TestAlertTarget_PodMonitoring_Success(t *testing.T) {
	unittest.SmallTest(t)
	expected := `apiVersion: monitoring.googleapis.com/v1
kind: PodMonitoring
metadata:
 name: perf-perfns
spec:
 selector:
   matchLabels:
      appgroup: perf
 endpoints:
   - port: prom
     interval: 15s
 targetLabels:
   fromPod:
     - from: app
     - from: appgroup
`
	got, err := alertTarget.PodMonitoring()
	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestNameSpaceOrDefault_NoNamespaceProvided_ReturnsDefault(t *testing.T) {
	unittest.SmallTest(t)

	require.Equal(t, "default", NamespaceOrDefault(""))
}

func TestNameSpaceOrDefault_NamespaceProvided_ReturnsGivenNamespace(t *testing.T) {
	unittest.SmallTest(t)

	require.Equal(t, "foo", NamespaceOrDefault("foo"))
}

func TestGetAlertTargetsFromFilename_ContainsOneDeploymentInDefaultNamespace_Success(t *testing.T) {
	unittest.MediumTest(t)

	got, err := getAlertTargetsFromFilename(filepath.Join(testutils.TestDataDir(t), "deployment.yaml"))
	require.NoError(t, err)
	require.Len(t, got, 1)

	// The key into alertTarget contains alertTarget.Directory, which will
	// change based on where the code is being run, so iternate over the map to
	// test the members.
	for alertTarget := range got {
		require.Equal(t, "perf", alertTarget.AppGroup)
		require.Equal(t, "default", alertTarget.Namespace)
		require.Contains(t, alertTarget.Directory, "/promk/go/genpromcrd/genpromcrd/testdata")
	}
}

func TestGetAlertTargetsFromFilename_ContainsOneStatefulSetInNonDefaultNamespace_Success(t *testing.T) {
	unittest.MediumTest(t)

	got, err := getAlertTargetsFromFilename(filepath.Join(testutils.TestDataDir(t), "statefulset.yml"))
	require.NoError(t, err)
	require.Len(t, got, 1)

	// The key into alertTarget contains alertTarget.Directory, which will
	// change based on where the code is being run, so iternate over the map to
	// test the members.
	for alertTarget := range got {
		require.Equal(t, "prometheus", alertTarget.AppGroup)
		require.Equal(t, "prometheus", alertTarget.Namespace)
		require.Contains(t, alertTarget.Directory, "/promk/go/genpromcrd/genpromcrd/testdata")
	}
}

func TestGetAlertTargetsFromFilename_FileDoesNotExist_ReturnsError(t *testing.T) {
	unittest.MediumTest(t)

	_, err := getAlertTargetsFromFilename(filepath.Join(testutils.TestDataDir(t), "the-name-of-a-file-that-does-not-exist.yml"))
	require.Error(t, err)
}

func TestGetAllAlertTargetsUnderDir_DirContainsYAMLFilesThatShouldBeSkipped_OnlyTheOneValidFileIsRead(t *testing.T) {
	unittest.MediumTest(t)

	// The 'fake-checkout' directory has deployment files in this tree:
	//
	//  fake-checkout/
	//  ├── monitoring
	//  │   └── appgroups
	//  │       └── perf.yml
	//  ├── skia-infra-public
	//  │   ├── deployment.yml
	//  │   └── this-deployment-is-ignored.yml
	//  ├── templates
	//  │   └── this-deployment-is-ignored.yml
	//  └── this-deployment-is-ignored.yaml
	//
	// Only the two files under skia-infra-public should be read as
	// getAllAlertTargetsUnderDir only looks at files under directories that
	// correspond to cluster names.

	alertTargets, err := getAllAlertTargetsUnderDir(filepath.Join(testutils.TestDataDir(t), "fake-checkout"))
	require.NoError(t, err)
	require.Len(t, alertTargets, 2)
}

func TestAppMain_NoDirectoryFlagSupplied_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	require.Error(t, NewApp().Main([]string{"path/to/exe/goes/here"}))
}

func TestAppMain_DryRunOverFakeCheckout_PrintsListOfFilesWritten(t *testing.T) {
	unittest.MediumTest(t)

	// Setup to capture stdout.
	backup := os.Stdout
	defer func() {
		os.Stdout = backup
	}()
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run Main with the --dryrun flag which only prints the names of the files it would write.
	require.NoError(t, NewApp().Main(
		[]string{
			"path/to/exe/goes/here",
			"--dryrun",
			"--directory", filepath.Join(testutils.TestDataDir(t), "fake-checkout"),
		}))

	err := w.Close()
	require.NoError(t, err)
	out, err := ioutil.ReadAll(r)
	require.NoError(t, err)

	// We only expect a single file to be written.
	parts := strings.Split(string(out), "\n")
	require.Len(t, parts, 2)
	require.Contains(t, parts[0], "/testdata/fake-checkout/skia-infra-public/perf_mytestnamespace_appgroup_alerts.yml")
	require.Equal(t, "", parts[1])
}

func TestAppMain_RunOverFakeCheckout_CorrectFileContentsAreWritten(t *testing.T) {
	unittest.MediumTest(t)

	tmpDir := t.TempDir()
	err := copy.Copy(filepath.Join(testutils.TestDataDir(t), "fake-checkout"), tmpDir)
	require.NoError(t, err)

	require.NoError(t, NewApp().Main(
		[]string{
			"path/to/exe/goes/here",
			"--directory", tmpDir,
		}))

	newlyWrittenFilename := filepath.Join(tmpDir, "skia-infra-public/perf_mytestnamespace_appgroup_alerts.yml")
	require.FileExists(t, newlyWrittenFilename)
	b, err := ioutil.ReadFile(newlyWrittenFilename)
	require.NoError(t, err)

	expected := `# File is generated by genpromcrd. DO NOT EDIT.
apiVersion: monitoring.googleapis.com/v1
kind: Rules
metadata:
  name: perf
  namespace: mytestnamespace
spec:
  groups:
  - name: perf
    interval: 30s
    rules:
    - alert: AlwaysFiringAlertToSeeIfAlertsAreWorking
      expr: vector(1)
      labels: {}
      annotations: {}
  - name: absent-perf
    interval: 30s
    rules: []

---
apiVersion: monitoring.googleapis.com/v1
kind: PodMonitoring
metadata:
 name: perf-mytestnamespace
spec:
 selector:
   matchLabels:
      appgroup: perf
 endpoints:
   - port: prom
     interval: 15s
 targetLabels:
   fromPod:
     - from: app
     - from: appgroup
`
	require.Equal(t, expected, string(b))
}

func TestAppMain_UnknownFlagPassedIn_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	require.Equal(t, ErrFlagsParse, NewApp().Main(
		[]string{
			"path/to/exe/goes/here",
			"--unknown-flag",
		}))
}
