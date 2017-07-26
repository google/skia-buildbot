package gce

import (
	"fmt"
	"testing"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/ct/instance_types"
	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

//func TestGetInstances(t *testing.T) {
//	testutils.SmallTest(t)
//	a, err := NewAutoscaller(gce.ZONE_CT, "/tmp/something", instance_types.CT_WORKER_PREFIX, 1, 2, instance_types.CTInstance)
//	assert.Nil(t, err)

//	a.GetRunningInstances()
//}

func TestStartAllInstances(t *testing.T) {
	testutils.SmallTest(t)
	a, err := NewAutoscaller(gce.ZONE_CT, "/tmp/something", instance_types.CT_WORKER_PREFIX, 1, 2, instance_types.CTInstance)
	assert.Nil(t, err)

	fmt.Println(a.GetRunningInstances())
	a.StartAllInstances()
	fmt.Println(a.GetRunningInstances())
}

//func TestStopAllInstances(t *testing.T) {
//	testutils.SmallTest(t)
//	a, err := NewAutoscaller(gce.ZONE_CT, "/tmp/something", instance_types.CT_WORKER_PREFIX, 1, 2, instance_types.CTInstance)
//	assert.Nil(t, err)

//	fmt.Println(a.GetRunningInstances())
//	a.StopAllInstances()
//	fmt.Println(a.GetRunningInstances())
//}
