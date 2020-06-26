package goldpushk

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
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
		require.True(t, deployableUnitSet.IsKnownInstance(unit.Instance), msg(unit.DeployableUnitID))
		require.True(t, deployableUnitSet.IsKnownService(unit.Service), msg(unit.DeployableUnitID))
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
		require.Contains(t, seen, i)
	}
}

func TestProductionDeployableUnitsAllInstancesHaveCommonServices(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()

	assertHasService := func(i Instance, s Service) {
		_, ok := deployableUnitSet.Get(DeployableUnitID{Instance: i, Service: s})
		require.True(t, ok, fmt.Sprintf("%s is missing service %s", i, s))
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
		require.True(t, unit.internal == (unit.Instance == Fuchsia), msg(unit.DeployableUnitID))
	}
}

func TestProductionDeployableUnitsAllPublicSkiaCorrectnessDeploymentsRequireAConfigMap(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()
	for _, unit := range deployableUnitSet.deployableUnits {
		if isPublicInstance(unit.Instance) && unit.Service == SkiaCorrectness {
			require.NotEmpty(t, unit.configMapName, msg(unit.DeployableUnitID))
		}
	}
}

func TestProductionDeployableUnitsConfigMapNamesAreCorrect(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()

	// Public instance.
	skiaPublicSkiaCorrectness, ok := deployableUnitSet.Get(DeployableUnitID{Instance: SkiaPublic, Service: SkiaCorrectness})
	require.True(t, ok)
	require.Equal(t, skiaPublicSkiaCorrectness.configMapName, "skia-public-authorized-params")
}

func TestProductionDeployableUnitsConfigMapInvariantsHold(t *testing.T) {
	unittest.SmallTest(t)
	deployableUnitSet := ProductionDeployableUnits()

	for _, unit := range deployableUnitSet.deployableUnits {
		// All DeployableUnits with any ConfigMap settings must have a configMapName
		// and exactly one of fields configMapFile or configMapTemplate set.
		if unit.configMapName != "" || unit.configMapFile != "" || unit.configMapTemplate != "" {
			require.NotEmpty(t, unit.configMapName, unit.CanonicalName())

			numFieldsSet := 0
			if unit.configMapFile != "" {
				numFieldsSet++
			}
			if unit.configMapTemplate != "" {
				numFieldsSet++
			}
			require.Equal(t, 1, numFieldsSet, unit.CanonicalName())
		}
	}
}

func TestIsPublicInstance(t *testing.T) {
	unittest.SmallTest(t)
	require.True(t, isPublicInstance(SkiaPublic))
	require.False(t, isPublicInstance(Skia))
}
