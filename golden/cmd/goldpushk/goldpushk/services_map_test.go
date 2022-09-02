package goldpushk

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

////////////////////////////////////////////////////////////////////////////////////////////////////
// Test invariants of the DeployableUnitSet returned by ProductionDeployableUnits().              //
////////////////////////////////////////////////////////////////////////////////////////////////////

// Utility function to generate an assertion message for a given instance/service pair.
func msg(id DeployableUnitID) string {
	return fmt.Sprintf("Instance: %s, service: %s", string(id.Instance), string(id.Service))
}

func TestProductionDeployableUnitsOnlyContainsKnownInstancesAndServices(t *testing.T) {
	deployableUnitSet := ProductionDeployableUnits()
	for _, unit := range deployableUnitSet.deployableUnits {
		require.True(t, deployableUnitSet.IsKnownInstance(unit.Instance), msg(unit.DeployableUnitID))
		require.True(t, deployableUnitSet.IsKnownService(unit.Service), msg(unit.DeployableUnitID))
	}
}

func TestProductionDeployableUnitsContainsAllKnownInstances(t *testing.T) {
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
	deployableUnitSet := ProductionDeployableUnits()

	assertHasService := func(i Instance, s Service) {
		_, ok := deployableUnitSet.Get(DeployableUnitID{Instance: i, Service: s})
		require.True(t, ok, fmt.Sprintf("%s is missing service %s", i, s))
	}

	for _, instance := range deployableUnitSet.knownInstances {
		assertHasService(instance, Frontend)
		if !isPublicInstance(instance) {
			assertHasService(instance, DiffCalculator)
			assertHasService(instance, Ingestion)
		}
	}
}

func TestIsPublicInstance(t *testing.T) {
	require.True(t, isPublicInstance(SkiaPublic))
	require.False(t, isPublicInstance(Skia))
}
