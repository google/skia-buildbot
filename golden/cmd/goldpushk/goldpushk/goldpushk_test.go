package goldpushk

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNew(t *testing.T) {
	unittest.SmallTest(t)

	// Gather some DeployableUnits to pass to New() as parameters.
	s := ProductionDeployableUnits()
	deployableUnits := []DeployableUnit{}
	deployableUnits = appendUnit(t, deployableUnits, s, Skia, DiffServer)            // Regular deployment.
	deployableUnits = appendUnit(t, deployableUnits, s, SkiaPublic, SkiaCorrectness) // Public deployment with non-templated ConfigMap.
	canariedDeployableUnits := []DeployableUnit{}
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Skia, IngestionBT)    // Regular deployment with templated ConfigMap.
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Fuchsia, DiffServer)  // Internal deployment.
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Fuchsia, IngestionBT) // Internal deployment with templated ConfigMap.

	// Call code under test.
	g := New(deployableUnits, canariedDeployableUnits, "path/to/buildbot", true, true, 30, 3, "http://skia-public.com", "http://skia-corp.com")

	expected := &Goldpushk{
		deployableUnits:            deployableUnits,
		canariedDeployableUnits:    canariedDeployableUnits,
		rootPath:                   "path/to/buildbot",
		dryRun:                     true,
		noCommit:                   true,
		minUptimeSeconds:           30,
		uptimePollFrequencySeconds: 3,
		skiaPublicConfigRepoUrl:    "http://skia-public.com",
		skiaCorpConfigRepoUrl:      "http://skia-corp.com",
	}
	require.Equal(t, expected, g)
}

func TestGoldpushkCheckOutGitRepositories(t *testing.T) {
	unittest.MediumTest(t)

	ctx := context.Background()

	// Create two fake skia-{public,corp}-config repositories (i.e. "git init" two temp directories).
	fakeSkiaPublicConfig, fakeSkiaCorpConfig := createFakeConfigRepos(t, ctx)
	defer fakeSkiaPublicConfig.Cleanup()
	defer fakeSkiaCorpConfig.Cleanup()

	// Create the goldpushk instance under test. We pass it the file://... URLs to
	// the two Git repositories created earlier.
	g := Goldpushk{
		skiaPublicConfigRepoUrl: fakeSkiaPublicConfig.RepoUrl(),
		skiaCorpConfigRepoUrl:   fakeSkiaCorpConfig.RepoUrl(),
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake "skia-public-config" and "skia-corp-config"
	// repositories. This will clone the repositories created earlier by running
	// "git clone file://...".
	err := g.checkOutGitRepositories(ctx)

	// Assert that no errors occurred and that we have a git.TempCheckout instance
	// for each cloned repo.
	require.NoError(t, err)
	require.NotNil(t, g.skiaPublicConfigCheckout)
	require.NotNil(t, g.skiaCorpConfigCheckout)

	// Clean up the checkouts after the test finishes.
	defer g.skiaPublicConfigCheckout.Delete()
	defer g.skiaCorpConfigCheckout.Delete()

	// Assert that the local path to the checkouts is not the same as the local
	// path to the fake "skia-public-config" and "skia-corp-config" repos created
	// earlier. This is just a basic sanity check that ensures that we're actually
	// dealing with clones of the original repos, as opposed to the original repos
	// themselves.
	require.NotEqual(t, g.skiaPublicConfigCheckout.GitDir, fakeSkiaPublicConfig.Dir())
	require.NotEqual(t, g.skiaCorpConfigCheckout.GitDir, fakeSkiaCorpConfig.Dir())

	// Read files from the checkouts.
	publicWhichRepoTxtBytes, err := ioutil.ReadFile(filepath.Join(string(g.skiaPublicConfigCheckout.GitDir), "which-repo.txt"))
	require.NoError(t, err)
	corpWhichRepoTxtBytes, err := ioutil.ReadFile(filepath.Join(string(g.skiaCorpConfigCheckout.GitDir), "which-repo.txt"))
	require.NoError(t, err)

	// Assert that the contents of file "which-repo.txt" on each checkout matches
	// the contents of the same file on the corresponding origin repository.
	require.Equal(t, "This is repo skia-public-config!", string(publicWhichRepoTxtBytes))
	require.Equal(t, "This is repo skia-corp-config!", string(corpWhichRepoTxtBytes))
}

func TestGoldpushkGetDeploymentFilePath(t *testing.T) {
	unittest.SmallTest(t)

	// Create the goldpushk instance under test.
	g := Goldpushk{}
	addFakeConfigRepoCheckouts(&g)

	// Gather the DeployableUnits we will call Goldpushk.getDeploymentFilePath() with.
	s := ProductionDeployableUnits()
	publicUnit, _ := s.Get(makeID(Skia, DiffServer))
	internalUnit, _ := s.Get(makeID(Fuchsia, DiffServer))

	require.Equal(t, filepath.Join(g.skiaPublicConfigCheckout.Dir(), "gold-skia-diffserver.yaml"), g.getDeploymentFilePath(publicUnit))
	require.Equal(t, filepath.Join(g.skiaCorpConfigCheckout.Dir(), "gold-fuchsia-diffserver.yaml"), g.getDeploymentFilePath(internalUnit))
}

func TestGoldpushkGetConfigMapFilePath(t *testing.T) {
	unittest.SmallTest(t)

	// Create the goldpushk instance under test.
	skiaInfraRoot := "/path/to/buildbot"
	g := Goldpushk{
		rootPath: skiaInfraRoot,
	}
	addFakeConfigRepoCheckouts(&g)

	// Gather the DeployableUnits we will call Goldpushk.getConfigMapFilePath() with.
	s := ProductionDeployableUnits()
	publicUnitWithoutConfigMap, _ := s.Get(makeID(Skia, DiffServer))
	publicUnitWithConfigMapTemplate, _ := s.Get(makeID(Skia, IngestionBT))
	publicUnitWithConfigMapFile, _ := s.Get(makeID(SkiaPublic, SkiaCorrectness))
	internalUnitWithoutConfigMap, _ := s.Get(makeID(Fuchsia, DiffServer))
	internalUnitWithConfigMapTemplate, _ := s.Get(makeID(Fuchsia, IngestionBT))

	// Helper functions to write more concise assertions.
	assertNoConfigMap := func(unit DeployableUnit) {
		_, ok := g.getConfigMapFilePath(unit)
		require.False(t, ok, unit.CanonicalName())
	}
	assertConfigMapFileEquals := func(unit DeployableUnit, expectedPath ...string) {
		path, ok := g.getConfigMapFilePath(unit)
		require.True(t, ok, unit.CanonicalName())
		require.Equal(t, filepath.Join(expectedPath...), path, unit.CanonicalName())
	}

	// Get the paths to the checked out repositories.
	skiaPublicConfigPath := g.skiaPublicConfigCheckout.Dir()
	skiaCorpConfigPath := g.skiaCorpConfigCheckout.Dir()

	// Assert that we get the correct ConfigMap file path for each DeployableUnit.
	assertNoConfigMap(publicUnitWithoutConfigMap)
	assertConfigMapFileEquals(publicUnitWithConfigMapTemplate, skiaPublicConfigPath, "gold-skia-ingestion-config-bt.json5")
	assertConfigMapFileEquals(publicUnitWithConfigMapFile, skiaInfraRoot, "golden/k8s-instances/skia-public/authorized-params.json5")
	assertNoConfigMap(internalUnitWithoutConfigMap)
	assertConfigMapFileEquals(internalUnitWithConfigMapTemplate, skiaCorpConfigPath, "gold-fuchsia-ingestion-config-bt.json5")
}

func TestRegenerateConfigFiles(t *testing.T) {
	unittest.SmallTest(t)

	// Test on a good combination of different types of deployments.
	s := ProductionDeployableUnits()
	deployableUnits := []DeployableUnit{}
	deployableUnits = appendUnit(t, deployableUnits, s, Skia, DiffServer)            // Regular deployment.
	deployableUnits = appendUnit(t, deployableUnits, s, SkiaPublic, SkiaCorrectness) // Public deployment with non-templated ConfigMap.
	canariedDeployableUnits := []DeployableUnit{}
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Skia, IngestionBT)    // Regular deployment with templated ConfigMap.
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Fuchsia, DiffServer)  // Internal deployment.
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Fuchsia, IngestionBT) // Internal deployment with templated ConfigMap.

	// Create the goldpushk instance under test.
	g := Goldpushk{
		deployableUnits:         deployableUnits,
		canariedDeployableUnits: canariedDeployableUnits,
		rootPath:                "/path/to/buildbot",
	}
	addFakeConfigRepoCheckouts(&g)

	// Get the paths to the checked out repositories, ending with a separator.
	skiaPublicConfigPath := g.skiaPublicConfigCheckout.Dir() + string(filepath.Separator)
	skiaCorpConfigPath := g.skiaCorpConfigCheckout.Dir() + string(filepath.Separator)

	// Set up mocks.
	commandCollector := exec.CommandCollector{}
	commandCollectorCtx := exec.NewContext(context.Background(), commandCollector.Run)

	// Call code under test.
	err := g.regenerateConfigFiles(commandCollectorCtx)
	require.NoError(t, err)

	// Expected commands.
	expected := []string{
		// Skia DiffServer
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/skia-instance.json5 " +
			"-extra INSTANCE_ID:skia " +
			"-t /path/to/buildbot/golden/k8s-config-templates/gold-diffserver-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + skiaPublicConfigPath + "gold-skia-diffserver.yaml",

		// SkiaPublic SkiaCorrectness
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/skia-public-instance.json5 " +
			"-extra INSTANCE_ID:skia-public " +
			"-t /path/to/buildbot/golden/k8s-config-templates/gold-skiacorrectness-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + skiaPublicConfigPath + "gold-skia-public-skiacorrectness.yaml",

		// Skia IngestionBT
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/skia-instance.json5 " +
			"-extra INSTANCE_ID:skia " +
			"-t /path/to/buildbot/golden/k8s-config-templates/gold-ingestion-bt-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + skiaPublicConfigPath + "gold-skia-ingestion-bt.yaml",

		// Skia IngestionBT ConfigMap
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/skia-instance.json5 " +
			"-extra INSTANCE_ID:skia " +
			"-t /path/to/buildbot/golden/k8s-config-templates/ingest-config-template.json5 " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + skiaPublicConfigPath + "gold-skia-ingestion-config-bt.json5",

		// Fuchsia DiffServer
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/fuchsia-instance.json5 " +
			"-extra INSTANCE_ID:fuchsia " +
			"-t /path/to/buildbot/golden/k8s-config-templates/gold-diffserver-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + skiaCorpConfigPath + "gold-fuchsia-diffserver.yaml",

		// Fuchsia IngestionBT
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/fuchsia-instance.json5 " +
			"-extra INSTANCE_ID:fuchsia " +
			"-t /path/to/buildbot/golden/k8s-config-templates/gold-ingestion-bt-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + skiaCorpConfigPath + "gold-fuchsia-ingestion-bt.yaml",

		// Fuchsia IngestionBT ConfigMap
		"kube-conf-gen " +
			"-c /path/to/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /path/to/buildbot/golden/k8s-instances/fuchsia-instance.json5 " +
			"-extra INSTANCE_ID:fuchsia " +
			"-t /path/to/buildbot/golden/k8s-config-templates/ingest-config-template.json5 " +
			"-parse_conf=false " +
			"-strict " +
			"-o " + skiaCorpConfigPath + "gold-fuchsia-ingestion-config-bt.json5",
	}

	for i, e := range expected {
		require.Equal(t, e, exec.DebugString(commandCollector.Commands()[i]))
	}
}

func TestCommitConfigFiles(t *testing.T) {
	unittest.MediumTest(t)

	ctx := context.Background()

	// Create two fake skia-{public,corp}-config repositories (i.e. "git init" two temp directories).
	fakeSkiaPublicConfig, fakeSkiaCorpConfig := createFakeConfigRepos(t, ctx)
	defer fakeSkiaPublicConfig.Cleanup()
	defer fakeSkiaCorpConfig.Cleanup()

	// Assert that there is just one commit on both repositories.
	assertNumCommits(t, ctx, fakeSkiaPublicConfig, 1)
	assertNumCommits(t, ctx, fakeSkiaCorpConfig, 1)

	// Create the goldpushk instance under test. We pass it the file://... URLs to the two Git
	// repositories created earlier.
	g := Goldpushk{
		skiaPublicConfigRepoUrl: fakeSkiaPublicConfig.RepoUrl(),
		skiaCorpConfigRepoUrl:   fakeSkiaCorpConfig.RepoUrl(),
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake "skia-public-config" and "skia-corp-config" repositories created earlier.
	// This will run "git clone file://..." for each repository.
	err := g.checkOutGitRepositories(ctx)
	require.NoError(t, err)
	defer g.skiaPublicConfigCheckout.Delete()
	defer g.skiaCorpConfigCheckout.Delete()

	// Add changes to skia-public-config and skia-corp-config.
	writeFileIntoRepo(t, g.skiaPublicConfigCheckout, "foo.yaml", "I'm a change in skia-public-config.")
	writeFileIntoRepo(t, g.skiaCorpConfigCheckout, "bar.yaml", "I'm a change in skia-corp-config.")

	// Pretend that the user confirms the commit step.
	cleanup := fakeStdin(t, "y\n")
	defer cleanup()

	// Call the function under test, which will try to commit and push the changes.
	ok, err := g.commitConfigFiles(ctx)
	require.NoError(t, err)

	// Assert that the user confirmed the commit step.
	require.True(t, ok)

	// Assert that the changes were pushed to the fake skia-{public,corp}-config repositories.
	assertNumCommits(t, ctx, fakeSkiaPublicConfig, 2)
	assertNumCommits(t, ctx, fakeSkiaCorpConfig, 2)
	assertRepositoryContainsFileWithContents(t, ctx, fakeSkiaPublicConfig, "foo.yaml", "I'm a change in skia-public-config.")
	assertRepositoryContainsFileWithContents(t, ctx, fakeSkiaCorpConfig, "bar.yaml", "I'm a change in skia-corp-config.")
}

func TestCommitConfigFilesOnlyOneRepositoryIsDirty(t *testing.T) {
	unittest.MediumTest(t)

	ctx := context.Background()

	// Create two fake skia-{public,corp}-config repositories (i.e. "git init" two temp directories).
	fakeSkiaPublicConfig, fakeSkiaCorpConfig := createFakeConfigRepos(t, ctx)
	defer fakeSkiaPublicConfig.Cleanup()
	defer fakeSkiaCorpConfig.Cleanup()

	// Assert that there is just one commit on both repositories.
	assertNumCommits(t, ctx, fakeSkiaPublicConfig, 1)
	assertNumCommits(t, ctx, fakeSkiaCorpConfig, 1)

	// Create the goldpushk instance under test. We pass it the file://... URLs to the two Git
	// repositories created earlier.
	g := Goldpushk{
		skiaPublicConfigRepoUrl: fakeSkiaPublicConfig.RepoUrl(),
		skiaCorpConfigRepoUrl:   fakeSkiaCorpConfig.RepoUrl(),
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake "skia-public-config" and "skia-corp-config" repositories created earlier.
	// This will run "git clone file://..." for each repository.
	err := g.checkOutGitRepositories(ctx)
	require.NoError(t, err)
	defer g.skiaPublicConfigCheckout.Delete()
	defer g.skiaCorpConfigCheckout.Delete()

	// Add changes to skia-corp-config only. Repository skia-public-config remains clean.
	writeFileIntoRepo(t, g.skiaCorpConfigCheckout, "foo.yaml", "I'm a change in skia-corp-config.")

	// Pretend that the user confirms the commit step.
	cleanup := fakeStdin(t, "y\n")
	defer cleanup()

	// Call the function under test, which will try to commit and push the changes.
	ok, err := g.commitConfigFiles(ctx)
	require.NoError(t, err)

	// Assert that the user confirmed the commit step.
	require.True(t, ok)

	// Assert that the skia-public-config repository remains unchanged.
	assertNumCommits(t, ctx, fakeSkiaPublicConfig, 1)

	// Assert that changes were pushed to the fake skia-corp-config repository.
	assertNumCommits(t, ctx, fakeSkiaCorpConfig, 2)
	assertRepositoryContainsFileWithContents(t, ctx, fakeSkiaCorpConfig, "foo.yaml", "I'm a change in skia-corp-config.")
}

func TestCommitConfigFilesAbortedByUser(t *testing.T) {
	unittest.MediumTest(t)

	ctx := context.Background()

	// Create two fake skia-{public,corp}-config repositories (i.e. "git init" two temp directories).
	fakeSkiaPublicConfig, fakeSkiaCorpConfig := createFakeConfigRepos(t, ctx)
	defer fakeSkiaPublicConfig.Cleanup()
	defer fakeSkiaCorpConfig.Cleanup()

	// Assert that there is just one commit on both repositories.
	assertNumCommits(t, ctx, fakeSkiaPublicConfig, 1)
	assertNumCommits(t, ctx, fakeSkiaCorpConfig, 1)

	// Create the goldpushk instance under test. We pass it the file://... URLs to the two Git
	// repositories created earlier.
	g := Goldpushk{
		skiaPublicConfigRepoUrl: fakeSkiaPublicConfig.RepoUrl(),
		skiaCorpConfigRepoUrl:   fakeSkiaCorpConfig.RepoUrl(),
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake "skia-public-config" and "skia-corp-config" repositories created earlier.
	// This will run "git clone file://..." for each repository.
	err := g.checkOutGitRepositories(ctx)
	require.NoError(t, err)
	defer g.skiaPublicConfigCheckout.Delete()
	defer g.skiaCorpConfigCheckout.Delete()

	// Add changes to skia-public-config and skia-corp-config.
	writeFileIntoRepo(t, g.skiaPublicConfigCheckout, "foo.yaml", "I'm a change in skia-public-config.")
	writeFileIntoRepo(t, g.skiaCorpConfigCheckout, "bar.yaml", "I'm a change in skia-corp-config.")

	// Pretend that the user aborts the commit step.
	restoreStdin := fakeStdin(t, "n\n")
	defer restoreStdin()

	// Call the function under test, which will try to commit and push the changes.
	ok, err := g.commitConfigFiles(ctx)
	require.NoError(t, err)

	// Assert that the user aborted the commit step.
	require.False(t, ok)

	// Assert that no changes were pushed to skia-public-config or skia-corp-config.
	assertNumCommits(t, ctx, fakeSkiaPublicConfig, 1)
	assertNumCommits(t, ctx, fakeSkiaCorpConfig, 1)
}

func TestCommitConfigFilesSkippedWithFlagNoCommit(t *testing.T) {
	unittest.MediumTest(t)

	ctx := context.Background()

	// Create two fake skia-{public,corp}-config repositories (i.e. "git init" two temp directories).
	fakeSkiaPublicConfig, fakeSkiaCorpConfig := createFakeConfigRepos(t, ctx)
	defer fakeSkiaPublicConfig.Cleanup()
	defer fakeSkiaCorpConfig.Cleanup()

	// Assert that there is just one commit on both repositories.
	assertNumCommits(t, ctx, fakeSkiaPublicConfig, 1)
	assertNumCommits(t, ctx, fakeSkiaCorpConfig, 1)

	// Create the goldpushk instance under test. We pass it the file://... URLs to the two Git
	// repositories created earlier.
	g := Goldpushk{
		skiaPublicConfigRepoUrl: fakeSkiaPublicConfig.RepoUrl(),
		skiaCorpConfigRepoUrl:   fakeSkiaCorpConfig.RepoUrl(),
		noCommit:                true,
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake "skia-public-config" and "skia-corp-config" repositories created earlier.
	// This will run "git clone file://..." for each repository.
	err := g.checkOutGitRepositories(ctx)
	require.NoError(t, err)
	defer g.skiaPublicConfigCheckout.Delete()
	defer g.skiaCorpConfigCheckout.Delete()

	// Add changes to skia-public-config and skia-corp-config.
	writeFileIntoRepo(t, g.skiaPublicConfigCheckout, "foo.yaml", "I'm a change in skia-public-config.")
	writeFileIntoRepo(t, g.skiaCorpConfigCheckout, "bar.yaml", "I'm a change in skia-corp-config.")

	// Call the function under test, which should not commit nor push any changes.
	ok, err := g.commitConfigFiles(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	// Assert that no changes were pushed to skia-public-config or skia-corp-config.
	assertNumCommits(t, ctx, fakeSkiaPublicConfig, 1)
	assertNumCommits(t, ctx, fakeSkiaCorpConfig, 1)
}

func TestCommitConfigFilesSkippedWithFlagDryRun(t *testing.T) {
	unittest.MediumTest(t)

	ctx := context.Background()

	// Create two fake skia-{public,corp}-config repositories (i.e. "git init" two temp directories).
	fakeSkiaPublicConfig, fakeSkiaCorpConfig := createFakeConfigRepos(t, ctx)
	defer fakeSkiaPublicConfig.Cleanup()
	defer fakeSkiaCorpConfig.Cleanup()

	// Assert that there is just one commit on both repositories.
	assertNumCommits(t, ctx, fakeSkiaPublicConfig, 1)
	assertNumCommits(t, ctx, fakeSkiaCorpConfig, 1)

	// Create the goldpushk instance under test. We pass it the file://... URLs to the two Git
	// repositories created earlier.
	g := Goldpushk{
		skiaPublicConfigRepoUrl: fakeSkiaPublicConfig.RepoUrl(),
		skiaCorpConfigRepoUrl:   fakeSkiaCorpConfig.RepoUrl(),
		dryRun:                  true,
	}

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Check out the fake "skia-public-config" and "skia-corp-config" repositories created earlier.
	// This will run "git clone file://..." for each repository.
	err := g.checkOutGitRepositories(ctx)
	require.NoError(t, err)
	defer g.skiaPublicConfigCheckout.Delete()
	defer g.skiaCorpConfigCheckout.Delete()

	// Add changes to skia-public-config and skia-corp-config.
	writeFileIntoRepo(t, g.skiaPublicConfigCheckout, "foo.yaml", "I'm a change in skia-public-config.")
	writeFileIntoRepo(t, g.skiaCorpConfigCheckout, "bar.yaml", "I'm a change in skia-corp-config.")

	// Call the function under test, which should not commit nor push any changes.
	ok, err := g.commitConfigFiles(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	// Assert that no changes were pushed to skia-public-config or skia-corp-config.
	assertNumCommits(t, ctx, fakeSkiaPublicConfig, 1)
	assertNumCommits(t, ctx, fakeSkiaCorpConfig, 1)
}

func TestSwitchClusters(t *testing.T) {
	unittest.SmallTest(t)

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

func TestPushSingleDeployableUnitDeleteNonexistentConfigMap(t *testing.T) {
	unittest.SmallTest(t)

	// Gather the DeployableUnit to deploy.
	s := ProductionDeployableUnits()
	unit, ok := s.Get(makeID(Skia, IngestionBT))
	require.True(t, ok)

	// Create the goldpushk instance under test.
	g := &Goldpushk{}
	addFakeConfigRepoCheckouts(g)

	// Hide goldpushk output to stdout.
	_, restoreStdout := hideStdout(t)
	defer restoreStdout()

	// Set up mocks.
	commandCollector := exec.CommandCollector{}
	commandCollector.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if cmd.Name == "kubectl" && cmd.Args[0] == "delete" {
			// This is the actual error message that is returned when the command exits with status 1.
			return errors.New("Command exited with exit status 1: kubectl delete configmap gold-skia-ingestion-config-bt")
		}
		return nil
	})
	commandCollectorCtx := exec.NewContext(context.Background(), commandCollector.Run)

	// Call code under test.
	err := g.pushSingleDeployableUnit(commandCollectorCtx, unit)
	require.NoError(t, err)

	// Assert that the correct kubectl and gcloud commands were executed.
	expectedCommands := []string{
		"gcloud container clusters get-credentials skia-public --zone us-central1-a --project skia-public",
		"kubectl delete configmap gold-skia-ingestion-config-bt",
		"kubectl create configmap gold-skia-ingestion-config-bt --from-file /path/to/skia-public-config/gold-skia-ingestion-config-bt.json5",
		"kubectl apply -f /path/to/skia-public-config/gold-skia-ingestion-bt.yaml",
	}
	require.Len(t, commandCollector.Commands(), len(expectedCommands))
	for i, command := range expectedCommands {
		require.Equal(t, command, exec.DebugString(commandCollector.Commands()[i]))
	}
}

func TestPushCanaries(t *testing.T) {
	unittest.SmallTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Skia, DiffServer)     // Public.
	units = appendUnit(t, units, s, Skia, IngestionBT)    // Public, with config map.
	units = appendUnit(t, units, s, Fuchsia, DiffServer)  // Internal.
	units = appendUnit(t, units, s, Fuchsia, IngestionBT) // Internal, with config map.

	// Create the goldpushk instance under test.
	g := &Goldpushk{
		canariedDeployableUnits: units,
	}
	addFakeConfigRepoCheckouts(g)

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
		"kubectl apply -f /path/to/skia-public-config/gold-skia-diffserver.yaml",
		"kubectl delete configmap gold-skia-ingestion-config-bt",
		"kubectl create configmap gold-skia-ingestion-config-bt --from-file /path/to/skia-public-config/gold-skia-ingestion-config-bt.json5",
		"kubectl apply -f /path/to/skia-public-config/gold-skia-ingestion-bt.yaml",
		"gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp",
		"kubectl apply -f /path/to/skia-corp-config/gold-fuchsia-diffserver.yaml",
		"kubectl delete configmap gold-fuchsia-ingestion-config-bt",
		"kubectl create configmap gold-fuchsia-ingestion-config-bt --from-file /path/to/skia-corp-config/gold-fuchsia-ingestion-config-bt.json5",
		"kubectl apply -f /path/to/skia-corp-config/gold-fuchsia-ingestion-bt.yaml",
	}
	require.Len(t, commandCollector.Commands(), len(expectedCommands))
	for i, command := range expectedCommands {
		require.Equal(t, command, exec.DebugString(commandCollector.Commands()[i]))
	}
}

func TestPushCanariesDryRun(t *testing.T) {
	unittest.SmallTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Skia, DiffServer)     // Public.
	units = appendUnit(t, units, s, Skia, IngestionBT)    // Public, with config map.
	units = appendUnit(t, units, s, Fuchsia, DiffServer)  // Internal.
	units = appendUnit(t, units, s, Fuchsia, IngestionBT) // Internal, with config map.

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

func TestPushServices(t *testing.T) {
	unittest.SmallTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Skia, DiffServer)     // Public.
	units = appendUnit(t, units, s, Skia, IngestionBT)    // Public, with config map.
	units = appendUnit(t, units, s, Fuchsia, DiffServer)  // Internal.
	units = appendUnit(t, units, s, Fuchsia, IngestionBT) // Internal, with config map.

	// Create the goldpushk instance under test.
	g := &Goldpushk{
		deployableUnits: units,
	}
	addFakeConfigRepoCheckouts(g)

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
		"kubectl apply -f /path/to/skia-public-config/gold-skia-diffserver.yaml",
		"kubectl delete configmap gold-skia-ingestion-config-bt",
		"kubectl create configmap gold-skia-ingestion-config-bt --from-file /path/to/skia-public-config/gold-skia-ingestion-config-bt.json5",
		"kubectl apply -f /path/to/skia-public-config/gold-skia-ingestion-bt.yaml",
		"gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp",
		"kubectl apply -f /path/to/skia-corp-config/gold-fuchsia-diffserver.yaml",
		"kubectl delete configmap gold-fuchsia-ingestion-config-bt",
		"kubectl create configmap gold-fuchsia-ingestion-config-bt --from-file /path/to/skia-corp-config/gold-fuchsia-ingestion-config-bt.json5",
		"kubectl apply -f /path/to/skia-corp-config/gold-fuchsia-ingestion-bt.yaml",
	}
	require.Len(t, commandCollector.Commands(), len(expectedCommands))
	for i, command := range expectedCommands {
		require.Equal(t, command, exec.DebugString(commandCollector.Commands()[i]))
	}
}

func TestPushServicesDryRun(t *testing.T) {
	unittest.SmallTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Skia, DiffServer)     // Public.
	units = appendUnit(t, units, s, Skia, IngestionBT)    // Public, with config map.
	units = appendUnit(t, units, s, Fuchsia, DiffServer)  // Internal.
	units = appendUnit(t, units, s, Fuchsia, IngestionBT) // Internal, with config map.

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

func TestGetUptimesSingleCluster(t *testing.T) {
	unittest.SmallTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Chrome, BaselineServer)
	units = appendUnit(t, units, s, ChromeGPU, BaselineServer)
	units = appendUnit(t, units, s, Flutter, BaselineServer)
	units = appendUnit(t, units, s, Flutter, DiffServer)

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
	now := time.Date(2019, 9, 24, 17, 58, 2, 0, time.UTC) // 2019-09-24T17:58:02Z

	// Call code under test.
	uptime, err := g.getUptimesSingleCluster(commandCollectorCtx, units, now)
	require.NoError(t, err)

	// Assert that we get the expected uptimes.
	require.Len(t, uptime, 2)
	require.Equal(t, 29*time.Second, uptime[makeID(Chrome, BaselineServer)])     // 17:58:02 - 17:57:33
	require.Equal(t, 159*time.Second, uptime[makeID(ChromeGPU, BaselineServer)]) // 17:58:02 - 17:55:23

	// One of its containers is not running (see line "gold-flutter-baselineserver ... <none>" above).
	require.NotContains(t, uptime, makeID(Flutter, BaselineServer))

	// Its only container is not running (see line "gold-flutter-diffserver ... <none>" above).
	require.NotContains(t, uptime, makeID(Flutter, DiffServer))
}

func TestGetUptimes(t *testing.T) {
	unittest.SmallTest(t)

	// Gather the DeployableUnits to deploy.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Chrome, DiffServer)  // Public instance (skia-public).
	units = appendUnit(t, units, s, Fuchsia, DiffServer) // Internal instance (skia-corp).

	// Create the goldpushk instance under test.
	g := &Goldpushk{}

	// Fake kubectl outputs.
	kubectlPublicOutput := "NAME  RUNNING_SINCE\ngold-chrome-diffserver  2019-09-24T17:57:02Z\n"
	kubectlCorpOutput := "NAME  RUNNING_SINCE\ngold-fuchsia-diffserver  2019-09-24T17:56:32Z\n"

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
		"kubectl get pods -o custom-columns=NAME:.metadata.labels.app,RUNNING_SINCE:.status.containerStatuses[0].state.running.startedAt",
		"gcloud container clusters get-credentials skia-corp --zone us-central1-a --project google.com:skia-corp",
		"kubectl get pods -o custom-columns=NAME:.metadata.labels.app,RUNNING_SINCE:.status.containerStatuses[0].state.running.startedAt",
	}
	require.Len(t, commandCollector.Commands(), len(expectedCommands))
	for i, command := range expectedCommands {
		require.Equal(t, command, exec.DebugString(commandCollector.Commands()[i]))
	}

	// Assert that we get the expected uptimes.
	require.Len(t, uptime, 2)
	require.Equal(t, 60*time.Second, uptime[makeID(Chrome, DiffServer)])  // 17:58:02 - 17:57:02
	require.Equal(t, 90*time.Second, uptime[makeID(Fuchsia, DiffServer)]) // 17:58:02 - 17:56:32
}

func TestMonitor(t *testing.T) {
	unittest.SmallTest(t)

	// Gather the DeployableUnits to monitor.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Chrome, BaselineServer)
	units = appendUnit(t, units, s, Chrome, DiffServer)
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
		// At t=10s, gold-chrome-diffserver is up.
		{
			makeID(Chrome, BaselineServer): 5 * time.Second,
			makeID(Chrome, DiffServer):     2 * time.Second,
		},
		// At t=15s, gold-chrome-ingestion-bt is up.
		{
			makeID(Chrome, BaselineServer): 10 * time.Second,
			makeID(Chrome, DiffServer):     7 * time.Second,
			makeID(Chrome, IngestionBT):    1 * time.Second,
		},
		// t=20s.
		{
			makeID(Chrome, BaselineServer): 15 * time.Second,
			makeID(Chrome, DiffServer):     12 * time.Second,
			makeID(Chrome, IngestionBT):    6 * time.Second,
		},
		// t=25s.
		{
			makeID(Chrome, BaselineServer): 20 * time.Second,
			makeID(Chrome, DiffServer):     17 * time.Second,
			makeID(Chrome, IngestionBT):    11 * time.Second,
		},
		// t=30s.
		{
			makeID(Chrome, BaselineServer): 25 * time.Second,
			makeID(Chrome, DiffServer):     22 * time.Second,
			makeID(Chrome, IngestionBT):    16 * time.Second,
		},
		// At t=35s, gold-chrome-baselineserver has been running for at least 30s.
		{
			makeID(Chrome, BaselineServer): 30 * time.Second,
			makeID(Chrome, DiffServer):     27 * time.Second,
			makeID(Chrome, IngestionBT):    21 * time.Second,
		},
		// At t=40s, gold-chrome-diffserver has been running for at least 30s.
		{
			makeID(Chrome, BaselineServer): 35 * time.Second,
			makeID(Chrome, DiffServer):     32 * time.Second,
			makeID(Chrome, IngestionBT):    26 * time.Second,
		},
		// At t=45s, gold-chrome-ingestion-bt has been running for at least 30s. Monitoring should end.
		{
			makeID(Chrome, BaselineServer): 40 * time.Second,
			makeID(Chrome, DiffServer):     37 * time.Second,
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

func TestMonitorDryRun(t *testing.T) {
	unittest.SmallTest(t)

	// Gather the DeployableUnits to monitor.
	s := ProductionDeployableUnits()
	units := []DeployableUnit{}
	units = appendUnit(t, units, s, Chrome, BaselineServer)
	units = appendUnit(t, units, s, Chrome, DiffServer)
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

// createFakeConfigRepos initializes two Git repositories in local temporary directories, which can
// be used as fake skia-{public,corp}-config repositories in tests.
func createFakeConfigRepos(t *testing.T, ctx context.Context) (fakeSkiaPublicConfig, fakeSkiaCorpConfig *testutils.GitBuilder) {
	// Create two fake "skia-public-config" and "skia-corp-config" Git repos on the local file system
	// (i.e. "git init" two temporary directories).
	fakeSkiaPublicConfig = testutils.GitInit(t, ctx)
	fakeSkiaCorpConfig = testutils.GitInit(t, ctx)

	// Populate fake repositories with a file that will make it easier to tell them apart later on.
	fakeSkiaPublicConfig.Add(ctx, "which-repo.txt", "This is repo skia-public-config!")
	fakeSkiaPublicConfig.Commit(ctx)
	fakeSkiaCorpConfig.Add(ctx, "which-repo.txt", "This is repo skia-corp-config!")
	fakeSkiaCorpConfig.Commit(ctx)

	// Allow repositories to receive pushes.
	fakeSkiaPublicConfig.AcceptPushes(ctx)
	fakeSkiaCorpConfig.AcceptPushes(ctx)

	return
}

// This is intended to be used in tests that do not need to write to disk, but need a
// git.TempCheckout instance to e.g. compute a path into a checkout.
func addFakeConfigRepoCheckouts(g *Goldpushk) {
	fakeSkiaPublicConfigCheckout := &git.TempCheckout{
		GitDir: "/path/to/skia-public-config",
	}
	fakeSkiaCorpConfigCheckout := &git.TempCheckout{
		GitDir: "/path/to/skia-corp-config",
	}
	g.skiaPublicConfigCheckout = fakeSkiaPublicConfigCheckout
	g.skiaCorpConfigCheckout = fakeSkiaCorpConfigCheckout
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
	commits, err := clone.RevList(ctx, "master")
	require.NoError(t, err)
	lastCommit := commits[0]
	actualContents, err := clone.GetFile(ctx, filename, lastCommit)
	require.NoError(t, err)
	require.Equal(t, expectedContents, actualContents)
}

// Generated by running:
// $ kubectl get pods -o custom-columns=NAME:.metadata.labels.app,RUNNING_SINCE:.status.containerStatuses[0].state.running.startedAt
const kubectlGetPodsOutput = `NAME                                                   RUNNING_SINCE
fiddler                                                2019-09-26T22:59:31Z
fiddler                                                2019-09-26T22:59:31Z
fiddler                                                2019-09-26T22:59:54Z
<none>                                                 <none>
<none>                                                 <none>
<none>                                                 <none>
gitsync2                                               2019-09-25T18:34:24Z
gitsync2-staging                                       2019-09-25T18:29:42Z
gold-chrome-baselineserver                             2019-09-24T17:57:25Z
gold-chrome-baselineserver                             2019-09-24T17:57:19Z
gold-chrome-baselineserver                             2019-09-24T17:57:33Z
gold-chrome-diffserver                                 2019-09-05T20:53:42Z
gold-chrome-gpu-baselineserver                         2019-09-24T17:55:23Z
gold-chrome-gpu-baselineserver                         2019-09-24T17:55:06Z
gold-chrome-gpu-baselineserver                         2019-09-24T17:55:14Z
gold-chrome-gpu-diffserver                             2019-09-14T05:56:23Z
gold-chrome-gpu-ingestion-bt                           2019-09-24T17:53:24Z
gold-chrome-gpu-skiacorrectness                        2019-09-23T16:42:39Z
gold-chrome-ingestion-bt                               2019-09-24T17:56:10Z
gold-chrome-skiacorrectness                            2019-09-23T16:42:23Z
gold-flutter-baselineserver                            2019-09-24T17:57:32Z
gold-flutter-baselineserver                            <none>
gold-flutter-baselineserver                            2019-09-24T17:57:21Z
gold-flutter-diffserver                                <none>
gold-flutter-engine-baselineserver                     2019-09-24T12:11:35Z
gold-flutter-engine-baselineserver                     2019-09-24T12:11:34Z
gold-flutter-engine-baselineserver                     2019-09-24T12:11:34Z
gold-flutter-engine-diffserver                         2019-09-24T12:10:28Z
gold-flutter-engine-ingestion-bt                       2019-09-24T17:57:45Z
gold-flutter-engine-skiacorrectness                    2019-09-24T12:21:58Z
gold-flutter-ingestion-bt                              2019-09-24T17:59:26Z
gold-flutter-skiacorrectness                           2019-09-23T16:47:49Z
gold-goldpushk-test1-crashing-server                   <none>
gold-goldpushk-test1-healthy-server                    2019-09-26T20:31:44Z
gold-goldpushk-test2-crashing-server                   <none>
gold-goldpushk-test2-healthy-server                    2019-09-26T20:31:45Z
gold-lottie-diffserver                                 2019-09-25T07:36:38Z
gold-lottie-ingestion-bt                               2019-09-24T18:01:03Z
gold-lottie-skiacorrectness                            2019-09-23T16:49:10Z
gold-pdfium-diffserver                                 2019-08-16T15:16:37Z
gold-pdfium-ingestion-bt                               2019-09-25T07:36:14Z
gold-pdfium-skiacorrectness                            2019-09-23T16:49:22Z
gold-skia-diffserver                                   2019-09-05T15:17:16Z
gold-skia-ingestion-bt                                 2019-09-24T18:02:47Z
gold-skia-public-skiacorrectness                       2019-09-24T16:52:42Z
gold-skia-skiacorrectness                              2019-09-24T16:51:49Z
grafana                                                2019-08-28T14:09:11Z
jsdoc                                                  2019-09-20T13:04:44Z
jsdoc                                                  2019-09-20T13:04:38Z
jsfiddle                                               2019-09-26T22:55:01Z
jsfiddle                                               2019-09-26T22:55:10Z
k8s-checker                                            2019-09-22T14:50:26Z
leasing                                                2019-09-12T02:14:12Z
`

// Generated by printing out the stdout captured in TestMonitor above.
const expectedMonitorStdout = `
Monitoring the following services until they all reach 30 seconds of uptime (polling every 5 seconds):
  gold-chrome-baselineserver
  gold-chrome-diffserver
  gold-chrome-ingestion-bt

Waiting 10 seconds before starting the monitoring step.

UPTIME    READY     NAME
<None>    No        gold-chrome-baselineserver
<None>    No        gold-chrome-diffserver
<None>    No        gold-chrome-ingestion-bt
----------------------------------------------
0s        No        gold-chrome-baselineserver
<None>    No        gold-chrome-diffserver
<None>    No        gold-chrome-ingestion-bt
----------------------------------------------
5s        No        gold-chrome-baselineserver
2s        No        gold-chrome-diffserver
<None>    No        gold-chrome-ingestion-bt
----------------------------------------------
10s       No        gold-chrome-baselineserver
7s        No        gold-chrome-diffserver
1s        No        gold-chrome-ingestion-bt
----------------------------------------------
15s       No        gold-chrome-baselineserver
12s       No        gold-chrome-diffserver
6s        No        gold-chrome-ingestion-bt
----------------------------------------------
20s       No        gold-chrome-baselineserver
17s       No        gold-chrome-diffserver
11s       No        gold-chrome-ingestion-bt
----------------------------------------------
25s       No        gold-chrome-baselineserver
22s       No        gold-chrome-diffserver
16s       No        gold-chrome-ingestion-bt
----------------------------------------------
30s       Yes       gold-chrome-baselineserver
27s       No        gold-chrome-diffserver
21s       No        gold-chrome-ingestion-bt
----------------------------------------------
35s       Yes       gold-chrome-baselineserver
32s       Yes       gold-chrome-diffserver
26s       No        gold-chrome-ingestion-bt
----------------------------------------------
40s       Yes       gold-chrome-baselineserver
37s       Yes       gold-chrome-diffserver
31s       Yes       gold-chrome-ingestion-bt
`

// Generated by printing out the stdout captured in TestMonitorDryRun above.
const expectedMonitorDryRunStdout = `
Monitoring the following services until they all reach 30 seconds of uptime (polling every 5 seconds):
  gold-chrome-baselineserver
  gold-chrome-diffserver
  gold-chrome-ingestion-bt

Skipping monitoring step (dry run).
`
