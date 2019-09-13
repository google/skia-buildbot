package goldpushk

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

////////////////////////////////////////////////////////////////////////////////
// Test invariants of the DeployableUnitSet returned by                       //
// BuildDeployableUnitSet().                                                  //
////////////////////////////////////////////////////////////////////////////////

// Utility function to generate an assertion message for a given instance/service pair.
func msg(id DeployableUnitID) string {
	return fmt.Sprintf("Instance: %s, service: %s", string(id.Instance), string(id.Service))
}

func TestBuildDeployableUnitSetOnlyContainsKnownInstancesAndServices(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := BuildDeployableUnitSet()
	for _, unit := range deployableUnitSet.deployableUnits {
		assert.True(t, IsKnownInstance(unit.Instance), msg(unit.DeployableUnitID))
		assert.True(t, IsKnownService(unit.Service), msg(unit.DeployableUnitID))
	}
}

func TestBuildDeployableUnitSetContainsAllKnownInstances(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := BuildDeployableUnitSet()

	seen := map[Instance]bool{}
	for _, unit := range deployableUnitSet.deployableUnits {
		seen[unit.Instance] = true
	}

	for _, i := range KnownInstances {
		assert.Contains(t, seen, i)
	}
}

func TestBuildDeployableUnitSetAllInstancesHaveCommonServices(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := BuildDeployableUnitSet()

	assertHasService := func(i Instance, s Service) {
		_, ok := deployableUnitSet.Get(DeployableUnitID{Instance: i, Service: s})
		assert.True(t, ok, fmt.Sprintf("%s is missing service %s", i, s))
	}

	for _, instance := range KnownInstances {
		assertHasService(instance, SkiaCorrectness)
		if !isPublicInstance(instance) {
			assertHasService(instance, DiffServer)
			assertHasService(instance, IngestionBT)
		}
	}
}

func TestBuildDeployableUnitSetAllExactlyFuchsiaServicesAreInternal(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := BuildDeployableUnitSet()
	for _, unit := range deployableUnitSet.deployableUnits {
		assert.True(t, unit.internal == (unit.Instance == Fuchsia), msg(unit.DeployableUnitID))
	}
}

func TestBuildDeployableUnitSetAllIngestionServiceDeploymentsRequireAConfigMap(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := BuildDeployableUnitSet()
	for _, unit := range deployableUnitSet.deployableUnits {
		if unit.Service == IngestionBT {
			assert.NotEmpty(t, unit.configMapName, msg(unit.DeployableUnitID))
		}
	}
}

func TestBuildDeployableUnitSetAllPublicSkiaCorrectnessDeploymentsRequireAConfigMap(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := BuildDeployableUnitSet()
	for _, unit := range deployableUnitSet.deployableUnits {
		if isPublicInstance(unit.Instance) && unit.Service == SkiaCorrectness {
			assert.NotEmpty(t, unit.configMapName, msg(unit.DeployableUnitID))
		}
	}
}

func TestBuildDeployableUnitSetConfigMapNamesAreCorrect(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := BuildDeployableUnitSet()

	// Public instance.
	skiaPublicSkiaCorrectness, ok := deployableUnitSet.Get(DeployableUnitID{Instance: SkiaPublic, Service: SkiaCorrectness})
	assert.True(t, ok)
	assert.Equal(t, skiaPublicSkiaCorrectness.configMapName, "skia-public-authorized-params")

	// Internal instance.
	skiaIngestionBT, ok := deployableUnitSet.Get(DeployableUnitID{Instance: Skia, Service: IngestionBT})
	assert.True(t, ok)
	assert.Equal(t, skiaIngestionBT.configMapName, "gold-skia-ingestion-config-bt")
}

////////////////////////////////////////////////////////////////////////////////
// Test utility functions.                                                    //
////////////////////////////////////////////////////////////////////////////////

func TestIsKnownInstance(t *testing.T) {
	unittest.SmallTest(t)

	assert.True(t, IsKnownInstance(Chrome))
	assert.False(t, IsKnownInstance(Instance("foo")))
}

func TestIsKnownService(t *testing.T) {
	unittest.SmallTest(t)

	assert.True(t, IsKnownService(DiffServer))
	assert.False(t, IsKnownService("bar"))
}

func TestIsPublicInstance(t *testing.T) {
	unittest.SmallTest(t)

	assert.True(t, isPublicInstance(SkiaPublic))
	assert.False(t, isPublicInstance(Skia))
}
