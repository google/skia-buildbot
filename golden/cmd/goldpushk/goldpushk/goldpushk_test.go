package goldpushk

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNew(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Gather some DeployableUnits to pass to New() as parameters.
	s := ProductionDeployableUnits()
	deployableUnits := []DeployableUnit{}
	deployableUnits = appendUnit(t, deployableUnits, s, Skia, DiffCalculator) // Regular deployment.
	deployableUnits = appendUnit(t, deployableUnits, s, SkiaPublic, Frontend) // Public deployment with non-templated ConfigMap.
	canariedDeployableUnits := []DeployableUnit{}
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Skia, IngestionBT)       // Regular deployment with templated ConfigMap.
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Fuchsia, DiffCalculator) // Internal deployment.
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Fuchsia, IngestionBT)    // Internal deployment with templated ConfigMap.

	// Call code under test.
	g := New(deployableUnits, canariedDeployableUnits, "path/to/buildbot", true /* =dryRun */, true /* =noCommit */, 30 /* =minUptimeSeconds */, 3 /* =uptimePollFrequencySeconds */, "http://k8s-config.com", true /* =verbose */)

	expected := &Goldpushk{
		deployableUnits:            deployableUnits,
		canariedDeployableUnits:    canariedDeployableUnits,
		goldSrcDir:                 "path/to/buildbot/golden",
		dryRun:                     true,
		noCommit:                   true,
		minUptimeSeconds:           30,
		uptimePollFrequencySeconds: 3,
		k8sConfigRepoUrl:           "http://k8s-config.com",
		verbose:                    true,
	}
	require.Equal(t, expected, g)
}

func TestGoldpushk_CheckOutK8sConfigGitRepository_Success(t *testing.T) {
	unittest.MediumTest(t)
	unittest.LinuxOnlyTest(t)

	ctx := context.Background()

	// Create a fake k8s-config repository (i.e. "git init" a temp directory).
	fakeK8sConfig := createFakeK8sConfigRepo(t, ctx)
	defer fakeK8sConfig.Cleanup()

	// Create the goldpushk instance under test. We pass it the file://... URL to the Git repository
	// created earlier.
	g := Goldpushk{
		k8sConfigRepoUrl: fakeK8sConfig.RepoUrl(),
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake k8s-config repository created earlier by running "git clone file://...".
	err := g.checkOutK8sConfigRepo(ctx)

	// Assert that no errors occurred and that we have a git.TempCheckout for the cloned repo.
	require.NoError(t, err)
	require.NotNil(t, g.k8sConfigCheckout)

	// Clean up the checkout after the test finishes.
	defer g.k8sConfigCheckout.Delete()

	// Assert that the local path to the checkout is not the same as the local path to the fake
	// k8s-config repo created earlier. This is just a basic correctness check to ensure that we're
	// actually dealing with a clone of the original repo, as opposed to the original repo itself.
	require.NotEqual(t, g.k8sConfigCheckout.GitDir, fakeK8sConfig.Dir())

	// Read README.md from the checkout.
	k8sConfigReadmeMdBytes, err := ioutil.ReadFile(filepath.Join(string(g.k8sConfigCheckout.GitDir), "README.md"))
	require.NoError(t, err)

	// Assert that file README.md has the expected contents.
	require.Equal(t, "This is repo k8s-config!", string(k8sConfigReadmeMdBytes))
}

func TestGoldpushk_GetDeploymentFilePath_Success(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Create the goldpushk instance under test.
	g := Goldpushk{}
	addFakeK8sConfigRepoCheckout(&g)

	// Gather the DeployableUnits we will call Goldpushk.getDeploymentFilePath() with.
	s := ProductionDeployableUnits()
	publicUnit, _ := s.Get(makeID(Skia, DiffCalculator))
	internalUnit, _ := s.Get(makeID(Fuchsia, DiffCalculator))

	require.Equal(t, filepath.Join(g.k8sConfigCheckout.Dir(), "skia-public", "gold-skia-diffcalculator.yaml"), g.getDeploymentFilePath(publicUnit))
	require.Equal(t, filepath.Join(g.k8sConfigCheckout.Dir(), "skia-corp", "gold-fuchsia-diffcalculator.yaml"), g.getDeploymentFilePath(internalUnit))
}

func TestGoldpushk_RegenerateConfigFiles_Success(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Test on a good combination of different types of deployments.
	s := ProductionDeployableUnits()
	deployableUnits := []DeployableUnit{}
	deployableUnits = appendUnit(t, deployableUnits, s, Skia, DiffCalculator) // Regular deployment.
	deployableUnits = appendUnit(t, deployableUnits, s, SkiaPublic, Frontend) // Public deployment with non-templated ConfigMap.
	canariedDeployableUnits := []DeployableUnit{}
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Skia, IngestionBT)       // Regular deployment with templated ConfigMap.
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Fuchsia, DiffCalculator) // Internal deployment.
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Fuchsia, IngestionBT)    // Internal deployment with templated ConfigMap.

	// Create the goldpushk instance under test.
	g := Goldpushk{
		deployableUnits:         deployableUnits,
		canariedDeployableUnits: canariedDeployableUnits,
		goldSrcDir:              "/path/to/buildbot/golden",

		// Fake out the copy
		disableCopyingConfigsToCheckout: true,

		fakeNow: time.Date(2020, time.July, 6, 5, 4, 3, 0, time.UTC),
	}
	addFakeK8sConfigRepoCheckout(&g)

	// Set up mocks.
	commandCollector := exec.CommandCollector{}
	commandCollectorCtx := exec.NewContext(context.Background(), commandCollector.Run)

	// Call code under test.
	err := g.regenerateConfigFiles(commandCollectorCtx)
	require.NoError(t, err)

	// Expected commands.
	expected := []string{
		// Skia DiffCalculator
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/skia/skia.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/skia/skia-diffcalculator.json5 " +
			"-extra INSTANCE_ID:skia " +
			"-extra NOW:2020-07-06T05_04_03Z_00 " +
			"-t /path/to/buildbot/golden/k8s-config-templates/gold-diffcalculator-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + g.k8sConfigCheckout.Dir() + "/skia-public/gold-skia-diffcalculator.yaml",

		// SkiaPublic Frontend
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/skia-public/skia-public.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/skia-public/skia-public-frontend.json5 " +
			"-extra INSTANCE_ID:skia-public " +
			"-extra NOW:2020-07-06T05_04_03Z_00 " +
			"-t /path/to/buildbot/golden/k8s-config-templates/gold-frontend-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + g.k8sConfigCheckout.Dir() + "/skia-public/gold-skia-public-frontend.yaml",

		// Skia IngestionBT
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/skia/skia.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/skia/skia-ingestion-bt.json5 " +
			"-extra INSTANCE_ID:skia " +
			"-extra NOW:2020-07-06T05_04_03Z_00 " +
			"-t /path/to/buildbot/golden/k8s-config-templates/gold-ingestion-bt-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + g.k8sConfigCheckout.Dir() + "/skia-public/gold-skia-ingestion-bt.yaml",

		// Fuchsia DiffCalculator
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/fuchsia/fuchsia.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/fuchsia/fuchsia-diffcalculator.json5 " +
			"-extra INSTANCE_ID:fuchsia " +
			"-extra NOW:2020-07-06T05_04_03Z_00 " +
			"-t /path/to/buildbot/golden/k8s-config-templates/gold-diffcalculator-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + g.k8sConfigCheckout.Dir() + "/skia-corp/gold-fuchsia-diffcalculator.yaml",

		// Fuchsia IngestionBT
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/fuchsia/fuchsia.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/fuchsia/fuchsia-ingestion-bt.json5 " +
			"-extra INSTANCE_ID:fuchsia " +
			"-extra NOW:2020-07-06T05_04_03Z_00 " +
			"-t /path/to/buildbot/golden/k8s-config-templates/gold-ingestion-bt-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + g.k8sConfigCheckout.Dir() + "/skia-corp/gold-fuchsia-ingestion-bt.yaml",
	}

	for i, e := range expected {
		assert.Equal(t, e, exec.DebugString(commandCollector.Commands()[i]))
	}
}

func TestGoldpushk_CommitConfigFiles_Success(t *testing.T) {
	unittest.MediumTest(t)
	unittest.LinuxOnlyTest(t)

	ctx := context.Background()

	// Create a fake k8s-config repository (i.e. "git init" a temp directory).
	fakeK8sConfig := createFakeK8sConfigRepo(t, ctx)
	defer fakeK8sConfig.Cleanup()
	assertNumCommits(t, ctx, fakeK8sConfig, 1)

	// Create the goldpushk instance under test. We pass it the file://... URL to the Git repository
	// created earlier.
	g := Goldpushk{
		k8sConfigRepoUrl: fakeK8sConfig.RepoUrl(),
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake k8s-config repository created earlier by running "git clone file://...".
	err := g.checkOutK8sConfigRepo(ctx)
	require.NoError(t, err)
	defer g.k8sConfigCheckout.Delete()

	// Add changes to the k8s-config repository checkout.
	writeFileIntoRepo(t, g.k8sConfigCheckout, "foo.yaml", "I'm a change in k8s-config.")

	// Pretend that the user confirms the commit step.
	cleanup := fakeStdin(t, "y\n")
	defer cleanup()

	// Call the function under test, which will try to commit and push the changes.
	ok, err := g.commitConfigFiles(ctx)
	require.NoError(t, err)

	// Assert that the user confirmed the commit step.
	require.True(t, ok)

	// Assert that the changes were pushed to the fake k8s-config repository.
	assertNumCommits(t, ctx, fakeK8sConfig, 2)
	assertRepositoryContainsFileWithContents(t, ctx, fakeK8sConfig, "foo.yaml", "I'm a change in k8s-config.")
}

func TestGoldpushk_CommitConfigFiles_UserAborts_DoesNotCommit(t *testing.T) {
	unittest.MediumTest(t)
	unittest.LinuxOnlyTest(t)

	ctx := context.Background()

	// Create a fake k8s-config repository (i.e. "git init" a temp directory).
	fakeK8sConfig := createFakeK8sConfigRepo(t, ctx)
	defer fakeK8sConfig.Cleanup()
	assertNumCommits(t, ctx, fakeK8sConfig, 1)

	// Create the goldpushk instance under test. We pass it the file://... URL to the Git repository
	// created earlier.
	g := Goldpushk{
		k8sConfigRepoUrl: fakeK8sConfig.RepoUrl(),
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake k8s-config repository created earlier by running "git clone file://...".
	err := g.checkOutK8sConfigRepo(ctx)
	require.NoError(t, err)
	defer g.k8sConfigCheckout.Delete()

	// Add changes to the k8s-config repository checkout.
	writeFileIntoRepo(t, g.k8sConfigCheckout, "foo.yaml", "I'm a change in k8s-config.")

	// Pretend that the user aborts the commit step.
	restoreStdin := fakeStdin(t, "n\n")
	defer restoreStdin()

	// Call the function under test, which will try to commit and push the changes.
	ok, err := g.commitConfigFiles(ctx)
	require.NoError(t, err)

	// Assert that the user aborted the commit step.
	require.False(t, ok)

	// Assert that no changes were pushed to the fake k8s-config repository.
	assertNumCommits(t, ctx, fakeK8sConfig, 1)
}

func TestGoldpushk_CommitConfigFiles_FlagNoCommitSet_DoesNotCommit(t *testing.T) {
	unittest.MediumTest(t)
	unittest.LinuxOnlyTest(t)

	ctx := context.Background()

	// Create a fake k8s-config repository (i.e. "git init" a temp directory).
	fakeK8sConfig := createFakeK8sConfigRepo(t, ctx)
	defer fakeK8sConfig.Cleanup()
	assertNumCommits(t, ctx, fakeK8sConfig, 1)

	// Create the goldpushk instance under test. We pass it the file://... URL to the Git repository
	// created earlier.
	g := Goldpushk{
		k8sConfigRepoUrl: fakeK8sConfig.RepoUrl(),
		noCommit:         true,
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake k8s-config repository created earlier by running "git clone file://...".
	err := g.checkOutK8sConfigRepo(ctx)
	require.NoError(t, err)
	defer g.k8sConfigCheckout.Delete()

	// Add changes to the k8s-config repository checkout.
	writeFileIntoRepo(t, g.k8sConfigCheckout, "foo.yaml", "I'm a change in k8s-config.")

	// Call the function under test, which should not commit nor push any changes.
	ok, err := g.commitConfigFiles(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	// Assert that no changes were pushed to the fake k8s-config repository.
	assertNumCommits(t, ctx, fakeK8sConfig, 1)
}

func TestGoldpushk_CommitConfigFiles_FlagDryRunSet_DoesNotCommit(t *testing.T) {
	unittest.MediumTest(t)
	unittest.LinuxOnlyTest(t)

	ctx := context.Background()

	// Create a fake k8s-config repository (i.e. "git init" a temp directory).
	fakeK8sConfig := createFakeK8sConfigRepo(t, ctx)
	defer fakeK8sConfig.Cleanup()
	assertNumCommits(t, ctx, fakeK8sConfig, 1)

	// Create the goldpushk instance under test. We pass it the file://... URL to the Git repository
	// created earlier.
	g := Goldpushk{
		k8sConfigRepoUrl: fakeK8sConfig.RepoUrl(),
		dryRun:           true,
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake k8s-config repository created earlier by running "git clone file://...".
	err := g.checkOutK8sConfigRepo(ctx)
	require.NoError(t, err)
	defer g.k8sConfigCheckout.Delete()

	// Add changes to the k8s-config repository checkout.
	writeFileIntoRepo(t, g.k8sConfigCheckout, "foo.yaml", "I'm a change in k8s-config.")

	// Call the function under test, which should not commit nor push any changes.
	ok, err := g.commitConfigFiles(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	// Assert that no changes were pushed to the fake k8s-config repository.
	assertNumCommits(t, ctx, fakeK8sConfig, 1)
}

func TestGoldpushk_SwitchClusters_Success(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Create the goldpushk instance under test.
	g := Goldpushk{}

	// Set up mocks.
	commandCollector := exec.CommandCollector{}
	commandCollectorCtx := exec.NewContext(context.Background(), commandCollector.Run)

	// Test cases.
	testCases := []struct {
		cluster     cluster
		expectedCmd string
	}{
		{
			cluster:     clusterSkiaPublic,
			expectedCmd: "gcloud container clusters get-credentials skia-public --zone us-central1-a --project skia-public",
		},
		{
			cluster:     clusterSkiaCorp,
			expectedCmd: "gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp",
		},
	}

	for i, tc := range testCases {
		err := g.switchClusters(commandCollectorCtx, tc.cluster)
		require.NoError(t, err)
		require.Equal(t, tc.expectedCmd, exec.DebugString(commandCollector.Commands()[i]))
	}
}

func TestGoldpushk_PushCanaries_Success(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Skia, DiffCalculator)    // Public.
	units = appendUnit(t, units, s, Skia, IngestionBT)       // Public, with config map.
	units = appendUnit(t, units, s, Fuchsia, DiffCalculator) // Internal.
	units = appendUnit(t, units, s, Fuchsia, IngestionBT)    // Internal, with config map.

	// Create the goldpushk instance under test.
	g := &Goldpushk{
		canariedDeployableUnits: units,
		goldSrcDir:              "/infra/golden",
	}
	addFakeK8sConfigRepoCheckout(g)

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Set up mocks.
	commandCollector := exec.CommandCollector{}
	commandCollectorCtx := exec.NewContext(context.Background(), commandCollector.Run)

	// Call code under test.
	err := g.pushCanaries(commandCollectorCtx)
	require.NoError(t, err)

	// Assert that the correct kubectl and gcloud commands were executed.
	expectedCommands := []string{
		"gcloud container clusters get-credentials skia-public --zone us-central1-a --project skia-public",
		"kubectl delete configmap gold-skia-config",
		"kubectl create configmap gold-skia-config --from-file /infra/golden/k8s-instances/skia",
		"kubectl apply -f /path/to/k8s-config/skia-public/gold-skia-diffcalculator.yaml",
		"kubectl apply -f /path/to/k8s-config/skia-public/gold-skia-ingestion-bt.yaml",
		"gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp",
		"kubectl delete configmap gold-fuchsia-config",
		"kubectl create configmap gold-fuchsia-config --from-file /infra/golden/k8s-instances/fuchsia",
		"kubectl apply -f /path/to/k8s-config/skia-corp/gold-fuchsia-diffcalculator.yaml",
		"kubectl apply -f /path/to/k8s-config/skia-corp/gold-fuchsia-ingestion-bt.yaml",
	}
	assert.Len(t, commandCollector.Commands(), len(expectedCommands))
	for i, command := range expectedCommands {
		assert.Equal(t, command, exec.DebugString(commandCollector.Commands()[i]))
	}
}

func TestGoldpushk_PushCanaries_FlagDryRunSet_DoesNotPush(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Skia, DiffCalculator)    // Public.
	units = appendUnit(t, units, s, Skia, IngestionBT)       // Public, with config map.
	units = appendUnit(t, units, s, Fuchsia, DiffCalculator) // Internal.
	units = appendUnit(t, units, s, Fuchsia, IngestionBT)    // Internal, with config map.

	// Create the goldpushk instance under test.
	g := &Goldpushk{
		canariedDeployableUnits: units,
		dryRun:                  true,
	}

	// Capture and hide goldpushk output to stdout.
	fakeStdout, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Set up mocks.
	commandCollector := exec.CommandCollector{}
	commandCollectorCtx := exec.NewContext(context.Background(), commandCollector.Run)

	// Call code under test.
	err := g.pushCanaries(commandCollectorCtx)
	require.NoError(t, err)

	// Assert that no commands were executed.
	require.Len(t, commandCollector.Commands(), 0)

	// Assert that the expected output was written to stdout.
	expectedStdout := `
Pushing canaried services.

Skipping push step (dry run).
`
	require.Equal(t, expectedStdout, readFakeStdout(t, fakeStdout))
}

func TestGoldpushk_PushServices_Success(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Skia, DiffCalculator)    // Public.
	units = appendUnit(t, units, s, Skia, IngestionBT)       // Public, with config map.
	units = appendUnit(t, units, s, Fuchsia, DiffCalculator) // Internal.
	units = appendUnit(t, units, s, Fuchsia, IngestionBT)    // Internal, with config map.

	// Create the goldpushk instance under test.
	g := &Goldpushk{
		deployableUnits: units,
		goldSrcDir:      "/infra/golden",
	}
	addFakeK8sConfigRepoCheckout(g)

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Set up mocks.
	commandCollector := exec.CommandCollector{}
	commandCollectorCtx := exec.NewContext(context.Background(), commandCollector.Run)

	// Call code under test.
	err := g.pushServices(commandCollectorCtx)
	require.NoError(t, err)

	// Assert that the correct kubectl and gcloud commands were executed.
	expectedCommands := []string{
		"gcloud container clusters get-credentials skia-public --zone us-central1-a --project skia-public",
		"kubectl delete configmap gold-skia-config",
		"kubectl create configmap gold-skia-config --from-file /infra/golden/k8s-instances/skia",
		"kubectl apply -f /path/to/k8s-config/skia-public/gold-skia-diffcalculator.yaml",
		"kubectl apply -f /path/to/k8s-config/skia-public/gold-skia-ingestion-bt.yaml",
		"gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp",
		"kubectl delete configmap gold-fuchsia-config",
		"kubectl create configmap gold-fuchsia-config --from-file /infra/golden/k8s-instances/fuchsia",
		"kubectl apply -f /path/to/k8s-config/skia-corp/gold-fuchsia-diffcalculator.yaml",
		"kubectl apply -f /path/to/k8s-config/skia-corp/gold-fuchsia-ingestion-bt.yaml",
	}
	assert.Len(t, commandCollector.Commands(), len(expectedCommands))
	for i, command := range expectedCommands {
		assert.Equal(t, command, exec.DebugString(commandCollector.Commands()[i]))
	}
}

func TestGoldpushk_PushServices_FlagDryRunSet_DoesNotPush(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Skia, DiffCalculator)    // Public.
	units = appendUnit(t, units, s, Skia, IngestionBT)       // Public, with config map.
	units = appendUnit(t, units, s, Fuchsia, DiffCalculator) // Internal.
	units = appendUnit(t, units, s, Fuchsia, IngestionBT)    // Internal, with config map.

	// Create the goldpushk instance under test.
	g := &Goldpushk{
		deployableUnits: units,
		dryRun:          true,
	}

	// Capture and hide goldpushk output to stdout.
	fakeStdout, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Set up mocks.
	commandCollector := exec.CommandCollector{}
	commandCollectorCtx := exec.NewContext(context.Background(), commandCollector.Run)

	// Call code under test.
	err := g.pushServices(commandCollectorCtx)
	require.NoError(t, err)

	// Assert that no commands were executed.
	require.Len(t, commandCollector.Commands(), 0)

	// Assert that the expected output was written to stdout.
	expectedStdout := `
Pushing services.

Skipping push step (dry run).
`
	require.Equal(t, expectedStdout, readFakeStdout(t, fakeStdout))
}

func TestGoldpushk_GetUptimesSingleCluster_Success(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Chrome, BaselineServer)
	units = appendUnit(t, units, s, Chrome, DiffCalculator)
	units = appendUnit(t, units, s, Flutter, BaselineServer)
	units = appendUnit(t, units, s, Flutter, DiffCalculator)

	// Create the goldpushk instance under test.
	g := &Goldpushk{}

	// Set up mocks.
	commandCollector := exec.CommandCollector{}
	commandCollector.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		n, err := cmd.CombinedOutput.Write([]byte(kubectlGetPodsOutput))
		require.NoError(t, err)
		require.Equal(t, len(kubectlGetPodsOutput), n)
		return nil
	})
	commandCollectorCtx := exec.NewContext(context.Background(), commandCollector.Run)

	// Fake time.
	now := time.Date(2019, 10, 01, 17, 30, 0, 0, time.UTC) // 2019-10-01T17:30:00Z

	// Call code under test.
	uptime, err := g.getUptimesSingleCluster(commandCollectorCtx, units, now)
	require.NoError(t, err)

	// Assert that we get the expected uptimes.
	require.Len(t, uptime, 2)
	require.Equal(t, 8*time.Minute, uptime[makeID(Chrome, BaselineServer)]) // 17:30:00 - 17:22:00
	require.Equal(t, 5*time.Minute, uptime[makeID(Chrome, DiffCalculator)]) // 17:30:00 - 17:25:00

	// One of its pods is not yet ready.
	require.NotContains(t, uptime, makeID(Flutter, BaselineServer))

	// Its only pod does not even show up in the kubectl output.
	require.NotContains(t, uptime, makeID(Flutter, DiffCalculator))
}

func TestGoldpushk_GetUptimes_Success(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Chrome, DiffCalculator)  // Public instance (skia-public).
	units = appendUnit(t, units, s, Fuchsia, DiffCalculator) // Internal instance (skia-corp).

	// Create the goldpushk instance under test.
	g := &Goldpushk{}

	// Fake kubectl outputs.
	kubectlPublicOutput := "app:gold-chrome-diffcalculator  podName:gold-chrome-diffcalculator-5ffc99f584-lxw98  ready:True  readyLastTransitionTime:2019-09-24T17:57:02Z"
	kubectlCorpOutput := "app:gold-fuchsia-diffcalculator  podName:gold-fuchsia-diffcalculator-8647b8f966-v7s2g  ready:True  readyLastTransitionTime:2019-09-24T17:56:32Z"

	// Set up mocks.
	numTimesKubectlGet := 0
	commandCollector := exec.CommandCollector{}
	commandCollector.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if cmd.Name == "kubectl" && cmd.Args[0] == "get" {
			numTimesKubectlGet++

			// First call corresponds to the skia-public cluster, second call corresponds to skia-corp.
			output := kubectlPublicOutput
			if numTimesKubectlGet == 2 {
				output = kubectlCorpOutput
			}

			n, err := cmd.CombinedOutput.Write([]byte(output))
			require.NoError(t, err)
			require.Equal(t, len(output), n)
		}
		return nil
	})
	commandCollectorCtx := exec.NewContext(context.Background(), commandCollector.Run)

	// Fake time.
	now := time.Date(2019, 9, 24, 17, 58, 2, 0, time.UTC) // 2019-09-24T17:58:02Z

	// Call code under test.
	uptime, err := g.getUptimes(commandCollectorCtx, units, now)
	require.NoError(t, err)

	// Assert that the correct kubectl and gcloud commands were executed.
	expectedCommands := []string{
		"gcloud container clusters get-credentials skia-public --zone us-central1-a --project skia-public",
		"kubectl get pods -o jsonpath={range .items[*]}{'app:'}{.metadata.labels.app}{'  podName:'}{.metadata.name}{'  ready:'}{.status.conditions[?(@.type == 'Ready')].status}{'  readyLastTransitionTime:'}{.status.conditions[?(@.type == 'Ready')].lastTransitionTime}{'\\n'}{end}",
		"gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp",
		"kubectl get pods -o jsonpath={range .items[*]}{'app:'}{.metadata.labels.app}{'  podName:'}{.metadata.name}{'  ready:'}{.status.conditions[?(@.type == 'Ready')].status}{'  readyLastTransitionTime:'}{.status.conditions[?(@.type == 'Ready')].lastTransitionTime}{'\\n'}{end}",
	}
	require.Len(t, commandCollector.Commands(), len(expectedCommands))
	for i, command := range expectedCommands {
		require.Equal(t, command, exec.DebugString(commandCollector.Commands()[i]))
	}

	// Assert that we get the expected uptimes.
	require.Len(t, uptime, 2)
	require.Equal(t, 60*time.Second, uptime[makeID(Chrome, DiffCalculator)])  // 17:58:02 - 17:57:02
	require.Equal(t, 90*time.Second, uptime[makeID(Fuchsia, DiffCalculator)]) // 17:58:02 - 17:56:32
}

func TestGoldpushk_Monitor_Success(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Gather the DeployableUnits to monitor.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Chrome, BaselineServer)
	units = appendUnit(t, units, s, Chrome, DiffCalculator)
	units = appendUnit(t, units, s, Chrome, IngestionBT)

	// Create the goldpushk instance under test.
	g := &Goldpushk{
		minUptimeSeconds:           30,
		uptimePollFrequencySeconds: 5,
	}

	// Capture and hide goldpushk output to stdout.
	fakeStdout, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Mock uptimes. This array holds the return values of the mock uptimesFn, ordered
	// chronologically.
	mockUptimes := []map[DeployableUnitID]time.Duration{
		// At t=0, no services are running.
		{},
		// At t=5s, gold-chrome-baselineserver is up.
		{
			makeID(Chrome, BaselineServer): 0 * time.Second,
		},
		// At t=10s, gold-chrome-diffcalculator is up.
		{
			makeID(Chrome, BaselineServer): 5 * time.Second,
			makeID(Chrome, DiffCalculator): 2 * time.Second,
		},
		// At t=15s, gold-chrome-ingestion-bt is up.
		{
			makeID(Chrome, BaselineServer): 10 * time.Second,
			makeID(Chrome, DiffCalculator): 7 * time.Second,
			makeID(Chrome, IngestionBT):    1 * time.Second,
		},
		// t=20s.
		{
			makeID(Chrome, BaselineServer): 15 * time.Second,
			makeID(Chrome, DiffCalculator): 12 * time.Second,
			makeID(Chrome, IngestionBT):    6 * time.Second,
		},
		// t=25s.
		{
			makeID(Chrome, BaselineServer): 20 * time.Second,
			makeID(Chrome, DiffCalculator): 17 * time.Second,
			makeID(Chrome, IngestionBT):    11 * time.Second,
		},
		// t=30s.
		{
			makeID(Chrome, BaselineServer): 25 * time.Second,
			makeID(Chrome, DiffCalculator): 22 * time.Second,
			makeID(Chrome, IngestionBT):    16 * time.Second,
		},
		// At t=35s, gold-chrome-baselineserver has been running for at least 30s.
		{
			makeID(Chrome, BaselineServer): 30 * time.Second,
			makeID(Chrome, DiffCalculator): 27 * time.Second,
			makeID(Chrome, IngestionBT):    21 * time.Second,
		},
		// At t=40s, gold-chrome-diffcalculator has been running for at least 30s.
		{
			makeID(Chrome, BaselineServer): 35 * time.Second,
			makeID(Chrome, DiffCalculator): 32 * time.Second,
			makeID(Chrome, IngestionBT):    26 * time.Second,
		},
		// At t=45s, gold-chrome-ingestion-bt has been running for at least 30s. Monitoring should end.
		{
			makeID(Chrome, BaselineServer): 40 * time.Second,
			makeID(Chrome, DiffCalculator): 37 * time.Second,
			makeID(Chrome, IngestionBT):    31 * time.Second,
		},
	}

	// Keep track of how many times the mock uptimesFn was called.
	numTimesMockUptimesFnWasCalled := 0

	// Mock uptimesFn.
	mockUptimesFn := func(_ context.Context, uptimesFnUnits []DeployableUnit, _ time.Time) (map[DeployableUnitID]time.Duration, error) {
		require.Equal(t, units, uptimesFnUnits)
		uptimes := mockUptimes[numTimesMockUptimesFnWasCalled]
		numTimesMockUptimesFnWasCalled += 1
		return uptimes, nil
	}

	// Keep track of how many times the mock sleepFn was called.
	numTimesMockSleepFnWasCalled := 0

	// Mock sleepFn.
	mockSleepFn := func(d time.Duration) {
		numTimesMockSleepFnWasCalled++

		// First call to sleep happens before entering the monitoring loop.
		if numTimesMockSleepFnWasCalled == 1 {
			require.Equal(t, 10*time.Second, d) // delayBetweenPushAndMonitoring.
		} else {
			require.Equal(t, 5*time.Second, d) // Goldpushk.uptimePollFrequencySeconds.
		}
	}

	// Call code under test.
	err := g.monitor(context.Background(), units, mockUptimesFn, mockSleepFn)
	require.NoError(t, err)

	// Assert that the mock uptimesFn was called the expected number of times, which means that
	// monitoring ends as soon as all services have been running for the required amount of time
	// (which in this case is 30s).
	require.Equal(t, 10, numTimesMockUptimesFnWasCalled)

	// Similarly, assert that sleepFn was called the expected number of times.
	require.Equal(t, 10, numTimesMockSleepFnWasCalled)

	// Assert that monitoring produced the expected output on stdout.
	require.Equal(t, expectedMonitorStdout, readFakeStdout(t, fakeStdout))
}

func TestGoldpushk_Monitor_FlagDryRunSet_DoesNotMonitor(t *testing.T) {
	unittest.SmallTest(t)
	unittest.LinuxOnlyTest(t)

	// Gather the DeployableUnits to monitor.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Chrome, BaselineServer)
	units = appendUnit(t, units, s, Chrome, DiffCalculator)
	units = appendUnit(t, units, s, Chrome, IngestionBT)

	// Create the goldpushk instance under test.
	g := &Goldpushk{
		dryRun:                     true,
		minUptimeSeconds:           30,
		uptimePollFrequencySeconds: 5,
	}

	// Capture and hide goldpushk output to stdout.
	fakeStdout, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Mock uptimesFn.
	mockUptimesFn := func(_ context.Context, _ []DeployableUnit, _ time.Time) (map[DeployableUnitID]time.Duration, error) {
		require.Fail(t, "uptimesFn should never be called in dry mode")
		return nil, nil
	}

	// Mock sleepFn.
	mockSleepFn := func(d time.Duration) {
		require.Fail(t, "sleepFn should never be called in dry mode")
	}

	// Call code under test.
	err := g.monitor(context.Background(), units, mockUptimesFn, mockSleepFn)
	require.NoError(t, err)

	// Assert that monitoring produced the expected output on stdout.
	require.Equal(t, expectedMonitorDryRunStdout, readFakeStdout(t, fakeStdout))
}

// appendUnit will retrieve a DeployableUnit from the given DeployableUnitSet using the given
// Instance and Service and append it to the given DeployableUnit slice.
func appendUnit(t *testing.T, units []DeployableUnit, s DeployableUnitSet, instance Instance, service Service) []DeployableUnit {
	unit, ok := s.Get(DeployableUnitID{Instance: instance, Service: service})
	require.True(t, ok, "Instance: %s, Service: %s", instance, service)
	return append(units, unit)
}

// makeID is a convenience method to create a DeployableUnitID.
func makeID(instance Instance, service Service) DeployableUnitID {
	return DeployableUnitID{
		Instance: instance,
		Service:  service,
	}
}

// createFakeK8sConfigRepo initializes a Git repository in local temporary directory which can be
// used as fake k8s-config repository in tests.
func createFakeK8sConfigRepo(t *testing.T, ctx context.Context) *testutils.GitBuilder {
	// "git init" a temporary directory.
	fakeK8sConfig := testutils.GitInit(t, ctx)

	// Populate fake repository with a file that will make it easier to identify it later on.
	fakeK8sConfig.Add(ctx, "README.md", "This is repo k8s-config!")
	fakeK8sConfig.Commit(ctx)

	// Allow repository to receive pushes.
	fakeK8sConfig.AcceptPushes(ctx)

	return fakeK8sConfig
}

// This is intended to be used in tests that do not need to write to disk, but need a
// git.TempCheckout instance to e.g. compute a path into a checkout.
func addFakeK8sConfigRepoCheckout(g *Goldpushk) {
	fakeK8sConfigCheckout := &git.TempCheckout{
		GitDir: "/path/to/k8s-config",
	}
	g.k8sConfigCheckout = fakeK8sConfigCheckout
}

// writeFileIntoRepo creates a file with the given name and contents into a *git.TempCheckout.
func writeFileIntoRepo(t *testing.T, repo *git.TempCheckout, name, contents string) {
	bytes := []byte(contents)
	path := filepath.Join(string(repo.GitDir), name)
	err := ioutil.WriteFile(path, bytes, os.ModePerm)
	require.NoError(t, err)
}

// hideStdout replaces os.Stdout with a temp file. This hides any output generated by the code under
// test and leads to a less noisy "go test" output.
func hideStdout(t *testing.T) (fakeStdout *os.File, cleanup func()) {
	// Back up the real stdout.
	stdout := os.Stdout
	cleanup = func() {
		os.Stdout = stdout
	}

	// Replace os.Stdout with a temporary file.
	fakeStdout, err := ioutil.TempFile("", "fake-stdout")
	require.NoError(t, err)
	os.Stdout = fakeStdout

	return fakeStdout, cleanup
}

// readFakeStdout takes the *os.File returned by hideStdout(), and reads and returns its contents.
func readFakeStdout(t *testing.T, fakeStdout *os.File) string {
	// Read the captured stdout.
	_, err := fakeStdout.Seek(0, 0)
	require.NoError(t, err)
	stdoutBytes, err := ioutil.ReadAll(fakeStdout)
	require.NoError(t, err)
	return string(stdoutBytes)
}

// fakeStdin fakes user input via stdin. It replaces stdin with a temporary file with the given fake
// input. The returned function should be called at the end of a test to restore the original stdin.
func fakeStdin(t *testing.T, userInput string) (cleanup func()) {
	// Back up stdin and provide a function to restore it later.
	realStdin := os.Stdin
	cleanup = func() {
		os.Stdin = realStdin
	}

	// Create new file to be used as a fake stdin.
	fakeStdin, err := ioutil.TempFile("", "fake-stdin")
	require.NoError(t, err)

	// Write fake user input.
	_, err = fakeStdin.WriteString(userInput)
	require.NoError(t, err)

	// Rewind stdin file so that fmt.Scanf() will pick up what we just wrote.
	_, err = fakeStdin.Seek(0, 0)
	require.NoError(t, err)

	// Replace real stdout with the fake one.
	os.Stdin = fakeStdin

	return cleanup
}

// assertNumCommits asserts that the given Git repository has the given number of commits.
func assertNumCommits(t *testing.T, ctx context.Context, repo *testutils.GitBuilder, n int64) {
	clone, err := git.NewTempCheckout(ctx, repo.RepoUrl())
	defer clone.Delete()
	require.NoError(t, err)
	actualN, err := clone.NumCommits(ctx)
	require.NoError(t, err)
	require.Equal(t, n, actualN)
}

// assertRepositoryContainsFileWithContents asserts the presence of a file with the given contents
// in a git repo.
func assertRepositoryContainsFileWithContents(t *testing.T, ctx context.Context, repo *testutils.GitBuilder, filename, expectedContents string) {
	clone, err := git.NewTempCheckout(ctx, repo.RepoUrl())
	require.NoError(t, err)
	commits, err := clone.RevList(ctx, git.DefaultBranch)
	require.NoError(t, err)
	lastCommit := commits[0]
	actualContents, err := clone.GetFile(ctx, filename, lastCommit)
	require.NoError(t, err)
	require.Equal(t, expectedContents, actualContents)
}

// This fake kubectl output simulates the following situation:
//   - gold-chrome-baselineserver:     three replicas, all of them ready.
//   - gold-chrome-diffcalculator:         one single replica, ready.
//   - gold-chrome-gpu-baselineserver: three replicas, all of them ready.
//   - gold-chrome-gpu-diffcalculator:     one single replica, which is NOT ready.
//   - gold-flutter-baselineserver:    three replicas; two of them are ready, one is not.
//
// Generated by running the following command, and editing the output to fit the story above:
// $ kubectl get pods -o jsonpath="{range .items[*]}{'app:'}{.metadata.labels.app}{'  podName:'}{.metadata.name}{'  ready:'}{.status.conditions[?(@.type == 'Ready')].status}{'  readyLastTransitionTime:'}{.status.conditions[?(@.type == 'Ready')].lastTransitionTime}{'\n'}{end}"
const kubectlGetPodsOutput = `app:gold-chrome-baselineserver  podName:gold-chrome-baselineserver-5ffc99f584-2v2vm  ready:True  readyLastTransitionTime:2019-10-01T17:20:00Z
app:gold-chrome-baselineserver  podName:gold-chrome-baselineserver-5ffc99f584-lxw98  ready:True  readyLastTransitionTime:2019-10-01T17:21:00Z
app:gold-chrome-baselineserver  podName:gold-chrome-baselineserver-5ffc99f584-ndzc8  ready:True  readyLastTransitionTime:2019-10-01T17:22:00Z
app:gold-chrome-diffcalculator  podName:gold-chrome-diffcalculator-0  ready:True  readyLastTransitionTime:2019-10-01T17:25:00Z
app:gold-chrome-gpu-baselineserver  podName:gold-chrome-gpu-baselineserver-5dfd8b65cb-9krrl  ready:True  readyLastTransitionTime:2019-10-01T17:10:00Z
app:gold-chrome-gpu-baselineserver  podName:gold-chrome-gpu-baselineserver-5dfd8b65cb-hr86n  ready:True  readyLastTransitionTime:2019-10-01T17:11:00Z
app:gold-chrome-gpu-baselineserver  podName:gold-chrome-gpu-baselineserver-5dfd8b65cb-l4lt5  ready:True  readyLastTransitionTime:2019-10-01T17:12:00Z
app:gold-chrome-gpu-diffcalculator  podName:gold-chrome-gpu-diffcalculator-0  ready:False  readyLastTransitionTime:2019-09-09T18:43:08Z
app:gold-flutter-baselineserver  podName:gold-flutter-baselineserver-8647b8f966-2fqw5  ready:True  readyLastTransitionTime:2019-09-03T16:49:23Z
app:gold-flutter-baselineserver  podName:gold-flutter-baselineserver-8647b8f966-qx2f8  ready:False  readyLastTransitionTime:2019-09-03T16:44:49Z
app:gold-flutter-baselineserver  podName:gold-flutter-baselineserver-8647b8f966-v7s2g  ready:True  readyLastTransitionTime:2019-09-30T13:58:37Z
`

// Generated by printing out the stdout captured in TestMonitor above.
const expectedMonitorStdout = `
Monitoring the following services until they all reach 30 seconds of uptime (polling every 5 seconds):
  gold-chrome-baselineserver
  gold-chrome-diffcalculator
  gold-chrome-ingestion-bt

Waiting 10 seconds before starting the monitoring step.

UPTIME    READY     NAME
<None>    No        gold-chrome-baselineserver
<None>    No        gold-chrome-diffcalculator
<None>    No        gold-chrome-ingestion-bt
----------------------------------------------
0s        No        gold-chrome-baselineserver
<None>    No        gold-chrome-diffcalculator
<None>    No        gold-chrome-ingestion-bt
----------------------------------------------
5s        No        gold-chrome-baselineserver
2s        No        gold-chrome-diffcalculator
<None>    No        gold-chrome-ingestion-bt
----------------------------------------------
10s       No        gold-chrome-baselineserver
7s        No        gold-chrome-diffcalculator
1s        No        gold-chrome-ingestion-bt
----------------------------------------------
15s       No        gold-chrome-baselineserver
12s       No        gold-chrome-diffcalculator
6s        No        gold-chrome-ingestion-bt
----------------------------------------------
20s       No        gold-chrome-baselineserver
17s       No        gold-chrome-diffcalculator
11s       No        gold-chrome-ingestion-bt
----------------------------------------------
25s       No        gold-chrome-baselineserver
22s       No        gold-chrome-diffcalculator
16s       No        gold-chrome-ingestion-bt
----------------------------------------------
30s       Yes       gold-chrome-baselineserver
27s       No        gold-chrome-diffcalculator
21s       No        gold-chrome-ingestion-bt
----------------------------------------------
35s       Yes       gold-chrome-baselineserver
32s       Yes       gold-chrome-diffcalculator
26s       No        gold-chrome-ingestion-bt
----------------------------------------------
40s       Yes       gold-chrome-baselineserver
37s       Yes       gold-chrome-diffcalculator
31s       Yes       gold-chrome-ingestion-bt
`

// Generated by printing out the stdout captured in TestMonitorDryRun above.
const expectedMonitorDryRunStdout = `
Monitoring the following services until they all reach 30 seconds of uptime (polling every 5 seconds):
  gold-chrome-baselineserver
  gold-chrome-diffcalculator
  gold-chrome-ingestion-bt

Skipping monitoring step (dry run).
`
