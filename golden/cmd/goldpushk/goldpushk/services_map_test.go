package goldpushk

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

////////////////////////////////////////////////////////////////////////////////////////////////////
// Test invariants of the DeployableUnitSet returned by ProductionDeployableUnits().              //
////////////////////////////////////////////////////////////////////////////////////////////////////

// Utility function to generate an assertion message for a given instance/service pair.
func msg(id DeployableUnitID) string {
	return fmt.Sprintf("Instance: %s, service: %s", string(id.Instance), string(id.Service))
}

func TestProductionDeployableUnitsOnlyContainsKnownInstancesAndServices(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()
	for _, unit := range deployableUnitSet.deployableUnits {
		assert.True(t, deployableUnitSet.IsKnownInstance(unit.Instance), msg(unit.DeployableUnitID))
		assert.True(t, deployableUnitSet.IsKnownService(unit.Service), msg(unit.DeployableUnitID))
	}
}

func TestProductionDeployableUnitsContainsAllKnownInstances(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()

	seen := map[Instance]bool{}
	for _, unit := range deployableUnitSet.deployableUnits {
		seen[unit.Instance] = true
	}

	for _, i := range deployableUnitSet.knownInstances {
		assert.Contains(t, seen, i)
	}
}

func TestProductionDeployableUnitsAllInstancesHaveCommonServices(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()

	assertHasService := func(i Instance, s Service) {
		_, ok := deployableUnitSet.Get(DeployableUnitID{Instance: i, Service: s})
		assert.True(t, ok, fmt.Sprintf("%s is missing service %s", i, s))
	}

	for _, instance := range deployableUnitSet.knownInstances {
		assertHasService(instance, SkiaCorrectness)
		if !isPublicInstance(instance) {
			assertHasService(instance, DiffServer)
			assertHasService(instance, IngestionBT)
		}
	}
}

func TestProductionDeployableUnitsAllExactlyFuchsiaServicesAreInternal(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()
	for _, unit := range deployableUnitSet.deployableUnits {
		assert.True(t, unit.internal == (unit.Instance == Fuchsia), msg(unit.DeployableUnitID))
	}
}

func TestProductionDeployableUnitsAllIngestionServiceDeploymentsRequireAConfigMap(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()
	for _, unit := range deployableUnitSet.deployableUnits {
		if unit.Service == IngestionBT {
			assert.NotEmpty(t, unit.configMapName, msg(unit.DeployableUnitID))
		}
	}
}

func TestProductionDeployableUnitsAllPublicSkiaCorrectnessDeploymentsRequireAConfigMap(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()
	for _, unit := range deployableUnitSet.deployableUnits {
		if isPublicInstance(unit.Instance) && unit.Service == SkiaCorrectness {
			assert.NotEmpty(t, unit.configMapName, msg(unit.DeployableUnitID))
		}
	}
}

func TestProductionDeployableUnitsConfigMapNamesAreCorrect(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()

	// Public instance.
	skiaPublicSkiaCorrectness, ok := deployableUnitSet.Get(DeployableUnitID{Instance: SkiaPublic, Service: SkiaCorrectness})
	assert.True(t, ok)
	assert.Equal(t, skiaPublicSkiaCorrectness.configMapName, "skia-public-authorized-params")

	// Internal instance.
	skiaIngestionBT, ok := deployableUnitSet.Get(DeployableUnitID{Instance: Skia, Service: IngestionBT})
	assert.True(t, ok)
	assert.Equal(t, skiaIngestionBT.configMapName, "gold-skia-ingestion-config-bt")
}

func TestProductionDeployableUnitsConfigMapInvariantsHold(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()

	for _, unit := range deployableUnitSet.deployableUnits {
		// All DeployableUnits with any ConfigMap settings must have a configMapName
		// and exactly one of fields configMapFile or configMapTemplate set.
		if unit.configMapName != "" || unit.configMapFile != "" || unit.configMapTemplate != "" {
			assert.NotEmpty(t, unit.configMapName, unit.CanonicalName())

			numFieldsSet := 0
			if unit.configMapFile != "" {
				numFieldsSet++
			}
			if unit.configMapTemplate != "" {
				numFieldsSet++
			}
			assert.Equal(t, 1, numFieldsSet, unit.CanonicalName())
		}

		// All IngestionBT instances have templated ConfigMaps.
		if unit.Service == IngestionBT {
			assert.NotEmpty(t, unit.configMapTemplate, unit.CanonicalName())
		}
	}
}

func TestIsPublicInstance(t *testing.T) {
	unittest.SmallTest(t)
	assert.True(t, isPublicInstance(SkiaPublic))
	assert.False(t, isPublicInstance(Skia))
}
