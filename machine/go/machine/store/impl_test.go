package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/machine/go/machine"
)

func TestMachineToStoreDescription_NoDimensions(t *testing.T) {
	d := machine.NewDescription()
	m := machineToStoreDescription(d)
	assert.Equal(t, storeDescription{
		Mode:               d.Mode,
		LastUpdated:        d.LastUpdated,
		MachineDescription: d,
	}, m)
}

func TestMachineToStoreDescription_WithDimensions(t *testing.T) {
	d := machine.NewDescription()
	d.Dimensions["os"] = []string{"Android"}
	d.Dimensions["device_type"] = []string{"sailfish"}
	d.Dimensions["quarantined"] = []string{"Device sailfish too hot."}

	m := machineToStoreDescription(d)
	assert.Equal(t, storeDescription{
		OS:                 []string{"Android"},
		DeviceType:         []string{"sailfish"},
		Quarantined:        []string{"Device sailfish too hot."},
		Mode:               d.Mode,
		LastUpdated:        d.LastUpdated,
		MachineDescription: d,
	}, m)
}
