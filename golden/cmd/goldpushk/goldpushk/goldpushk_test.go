package goldpushk

import (
	"context"
	"io/ioutil"
	"os"
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
	unittest.SmallTest(t)

	ctx := context.Background()

	// Create fake repositories to clone.
	skiaPublicConfig := testutils.GitInit(t, ctx)
	skiaCorpConfig := testutils.GitInit(t, ctx)
	defer skiaPublicConfig.Cleanup()
	defer skiaCorpConfig.Cleanup()

	// Populate fake repositories.
	skiaPublicConfig.Add(ctx, "which-repo.txt", "skia-public-config")
	skiaPublicConfig.Commit(ctx)
	skiaCorpConfig.Add(ctx, "which-repo.txt", "skia-corp-config")
	skiaCorpConfig.Commit(ctx)

	// Create goldpushk instance under test.
	g := New([]DeployableUnit{}, []DeployableUnit{}, "", false, skiaPublicConfig.RepoUrl(), skiaCorpConfig.RepoUrl())

	// Check out repositories.
	err := g.checkOutGitRepositories(ctx)
	assert.NoError(t, err)
	defer g.skiaPublicConfigCheckout.Delete()
	defer g.skiaCorpConfigCheckout.Delete()

	// Assert that the checked out repositories are not the same as the fake repositories.
	assert.NotEqual(t, g.skiaPublicConfigCheckout.GitDir, skiaPublicConfig.Dir())
	assert.NotEqual(t, g.skiaCorpConfigCheckout.GitDir, skiaCorpConfig.Dir())

	// Read files in the checked out repositories.
	publicWhichRepoTxt, err := readFile(filepath.Join(string(g.skiaPublicConfigCheckout.GitDir), "which-repo.txt"))
	assert.NoError(t, err)
	corpWhichRepoTxt, err := readFile(filepath.Join(string(g.skiaCorpConfigCheckout.GitDir), "which-repo.txt"))
	assert.NoError(t, err)

	// Assert that the files in the checked out repositories have the expected content.
	assert.Equal(t, "skia-public-config", publicWhichRepoTxt)
	assert.Equal(t, "skia-corp-config", corpWhichRepoTxt)
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

func readFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(contents), nil
}
