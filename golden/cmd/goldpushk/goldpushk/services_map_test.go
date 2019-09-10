package goldpushk

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

////////////////////////////////////////////////////////////////////////////////
// Test invariants of the GoldServicesMap returned by BuildServicesMap().     //
////////////////////////////////////////////////////////////////////////////////

// Utility function to generate an assertion message for a given instance/service pair.
func msg(i GoldInstance, s GoldService) string {
	return fmt.Sprintf("Instance: %s, service: %s", string(i), string(s))
}

func TestBuildServicesMapOnlyContainsKnownInstancesAndServices(t *testing.T) {
	unittest.SmallTest(t)
	m := BuildServicesMap()
	m.ForAll(func(i GoldInstance, s GoldService, _ GoldServiceDeployment) {
		assert.True(t, IsKnownGoldInstance(i), msg(i, s))
		assert.True(t, IsKnownGoldService(s), msg(i, s))
	})
}

func TestBuildServicesMapContainsAllKnownGoldInstances(t *testing.T) {
	unittest.SmallTest(t)
	m := BuildServicesMap()
	for _, i := range KnownGoldInstances {
		assert.Contains(t, m, i)
	}
}

func TestBuildServicesMapAllDeploymentsContainTheCorrectInstanceAndService(t *testing.T) {
	unittest.SmallTest(t)
	m := BuildServicesMap()
	m.ForAll(func(i GoldInstance, s GoldService, d GoldServiceDeployment) {
		assert.True(t, d.Instance == i, msg(i, s))
		assert.True(t, d.Service == s, msg(i, s))
	})
}

func TestBuildServicesMapAllFuchsiaServicesAndOnlyFuchsiaServicesAreInternal(t *testing.T) {
	unittest.SmallTest(t)
	m := BuildServicesMap()
	m.ForAll(func(i GoldInstance, s GoldService, d GoldServiceDeployment) {
		assert.True(t, d.Internal == (d.Instance == Fuchsia), msg(i, s))
	})
}

func TestBuildServicesMapAllIngestionServiceDeploymentsRequireAConfigMap(t *testing.T) {
	unittest.SmallTest(t)
	m := BuildServicesMap()
	m.ForAll(func(i GoldInstance, s GoldService, d GoldServiceDeployment) {
		if s == Ingestion || s == IngestionBT {
			assert.NotNil(t, d.ConfigMapName, msg(i, s))
		}
	})
}

////////////////////////////////////////////////////////////////////////////////
// Test utility functions.                                                    //
////////////////////////////////////////////////////////////////////////////////

func TestIsKnownGoldInstance(t *testing.T) {
	unittest.SmallTest(t)

	assert.True(t, IsKnownGoldInstance(Chrome))
	assert.False(t, IsKnownGoldInstance(GoldInstance("foo")))
}

func TestIsKnownGoldService(t *testing.T) {
	unittest.SmallTest(t)

	assert.True(t, IsKnownGoldService(DiffServer))
	assert.False(t, IsKnownGoldService("bar"))
}
