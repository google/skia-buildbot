package goldpushk

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

// TODO(lovisolo): Implement and test.
func TestGoldpushkRun(t *testing.T) {
	unittest.SmallTest(t)

	t.Skip("Not implemented")
}

func TestGoldpushkCheckOutGitRepositories(t *testing.T) {
	unittest.MediumTest(t)

	ctx := context.Background()

	// Create two fake "skia-public-config" and "skia-corp-config" Git repos on
	// the local file system (i.e. "git init" two temporary directories).
	fakeSkiaPublicConfig := testutils.GitInit(t, ctx)
	fakeSkiaCorpConfig := testutils.GitInit(t, ctx)
	defer fakeSkiaPublicConfig.Cleanup()
	defer fakeSkiaCorpConfig.Cleanup()

	// Populate fake repositories with a file that will make it easier to tell
	// them apart later on.
	fakeSkiaPublicConfig.Add(ctx, "which-repo.txt", "This is repo skia-public-config!")
	fakeSkiaPublicConfig.Commit(ctx)
	fakeSkiaCorpConfig.Add(ctx, "which-repo.txt", "This is repo skia-corp-config!")
	fakeSkiaCorpConfig.Commit(ctx)

	// Create the goldpushk instance under test. We pass it the file://... URLs to
	// the two Git repositories created earlier.
	g := New([]DeployableUnit{}, []DeployableUnit{}, "", false, fakeSkiaPublicConfig.RepoUrl(), fakeSkiaCorpConfig.RepoUrl())

	// Check out the fake "skia-public-config" and "skia-corp-config"
	// repositories. This will clone the repositories created earlier by running
	// "git clone file://...".
	err := g.checkOutGitRepositories(ctx)

	// Assert that no errors occurred and that we have a git.TempCheckout instance
	// for each cloned repo.
	assert.NoError(t, err)
	assert.NotNil(t, g.skiaPublicConfigCheckout)
	assert.NotNil(t, g.skiaCorpConfigCheckout)

	// Clean up the checkouts after the test finishes.
	defer g.skiaPublicConfigCheckout.Delete()
	defer g.skiaCorpConfigCheckout.Delete()

	// Assert that the local path to the checkouts is not the same as the local
	// path to the fake "skia-public-config" and "skia-corp-config" repos created
	// earlier. This is just a basic sanity check that ensures that we're actually
	// dealing with clones of the original repos, as opposed to the original repos
	// themselves.
	assert.NotEqual(t, g.skiaPublicConfigCheckout.GitDir, fakeSkiaPublicConfig.Dir())
	assert.NotEqual(t, g.skiaCorpConfigCheckout.GitDir, fakeSkiaCorpConfig.Dir())

	// Read files from the checkouts.
	publicWhichRepoTxtBytes, err := ioutil.ReadFile(filepath.Join(string(g.skiaPublicConfigCheckout.GitDir), "which-repo.txt"))
	assert.NoError(t, err)
	corpWhichRepoTxtBytes, err := ioutil.ReadFile(filepath.Join(string(g.skiaCorpConfigCheckout.GitDir), "which-repo.txt"))
	assert.NoError(t, err)

	// Assert that the contents of file "which-repo.txt" on each checkout matches
	// the contents of the same file on the corresponding origin repository.
	assert.Equal(t, "This is repo skia-public-config!", string(publicWhichRepoTxtBytes))
	assert.Equal(t, "This is repo skia-corp-config!", string(corpWhichRepoTxtBytes))
}

func TestRegenerateConfigFiles(t *testing.T) {
	unittest.SmallTest(t)

	// Test on a good combination of different types of deployments.
	s := BuildDeployableUnitSet()
	deployableUnits := []DeployableUnit{}
	deployableUnits = appendUnit(t, deployableUnits, s, Skia, DiffServer)            // Regular deployment.
	deployableUnits = appendUnit(t, deployableUnits, s, SkiaPublic, SkiaCorrectness) // Public deployment with non-templated ConfigMap.
	canariedDeployableUnits := []DeployableUnit{}
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Skia, IngestionBT)    // Regular deployment with templated ConfigMap.
	canariedDeployableUnits = appendUnit(t, canariedDeployableUnits, s, Fuchsia, IngestionBT) // Internal deployment with templated ConfigMap.

	// Set up mocks.
	commandCollector := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), commandCollector.Run)

	// Call code under test.
	g := New(deployableUnits, canariedDeployableUnits, "/foo/bar/buildbot", false, "", "")
	err := g.regenerateConfigFiles(ctx)

	// Expected commands.
	expected := []string{
		// Skia DiffServer
		"kube-conf-gen " +
			"-c /foo/bar/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /foo/bar/buildbot/golden/k8s-instances/skia-instance.json5 " +
			"-extra INSTANCE_ID:skia " +
			"-t /foo/bar/buildbot/golden/k8s-config-templates/gold-diffserver-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o /foo/bar/buildbot/golden/build/gold-skia-diffserver.yaml",

		// SkiaPublic SkiaCorrectness
		"kube-conf-gen " +
			"-c /foo/bar/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /foo/bar/buildbot/golden/k8s-instances/skia-public-instance.json5 " +
			"-extra INSTANCE_ID:skia-public " +
			"-t /foo/bar/buildbot/golden/k8s-config-templates/gold-skiacorrectness-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o /foo/bar/buildbot/golden/build/gold-skia-public-skiacorrectness.yaml",

		// Skia IngestionBT
		"kube-conf-gen " +
			"-c /foo/bar/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /foo/bar/buildbot/golden/k8s-instances/skia-instance.json5 " +
			"-extra INSTANCE_ID:skia " +
			"-t /foo/bar/buildbot/golden/k8s-config-templates/gold-ingestion-bt-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o /foo/bar/buildbot/golden/build/gold-skia-ingestion-bt.yaml",

		// Skia IngestionBT ConfigMap
		"kube-conf-gen " +
			"-c /foo/bar/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /foo/bar/buildbot/golden/k8s-instances/skia-instance.json5 " +
			"-extra INSTANCE_ID:skia " +
			"-t /foo/bar/buildbot/golden/k8s-config-templates/ingest-config-template.json5 " +
			"-parse_conf=false " +
			"-strict " +
			"-o /foo/bar/buildbot/golden/build/gold-skia-ingestion-config-bt.json5",

		// Fuchsia IngestionBT
		"kube-conf-gen " +
			"-c /foo/bar/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /foo/bar/buildbot/golden/k8s-instances/fuchsia-instance.json5 " +
			"-extra INSTANCE_ID:fuchsia " +
			"-t /foo/bar/buildbot/golden/k8s-config-templates/gold-ingestion-bt-template.yaml " +
			"-parse_conf=false " +
			"-strict " +
			"-o /foo/bar/buildbot/golden/build/gold-fuchsia-ingestion-bt.yaml",

		// Fuchsia IngestionBT ConfigMap
		"kube-conf-gen " +
			"-c /foo/bar/buildbot/golden/k8s-config-templates/gold-common.json5 " +
			"-c /foo/bar/buildbot/golden/k8s-instances/fuchsia-instance.json5 " +
			"-extra INSTANCE_ID:fuchsia " +
			"-t /foo/bar/buildbot/golden/k8s-config-templates/ingest-config-template.json5 " +
			"-parse_conf=false " +
			"-strict " +
			"-o /foo/bar/buildbot/golden/build/gold-fuchsia-ingestion-config-bt.json5",
	}

	assert.NoError(t, err)
	for i, e := range expected {
		assert.Equal(t, e, exec.DebugString(commandCollector.Commands()[i]))
	}
}

func appendUnit(t *testing.T, units []DeployableUnit, s DeployableUnitSet, instance Instance, service Service) []DeployableUnit {
	unit, ok := s.Get(DeployableUnitID{Instance: instance, Service: service})
	assert.True(t, ok)
	return append(units, unit)
}
