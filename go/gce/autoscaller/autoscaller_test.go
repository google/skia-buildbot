package gce

import (
	"testing"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/ct/instance_types"
	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func TestGetInstances(t *testing.T) {
	testutils.SmallTest(t)
	a, err := NewAutoscaller(gce.ZONE_CT, "/tmp/something", 0, 200, instance_types.CTInstance)
	assert.Nil(t, err)

	a.GetInstances(10)
}
