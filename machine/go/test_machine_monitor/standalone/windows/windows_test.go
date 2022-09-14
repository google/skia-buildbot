package windows

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gpus"
)

func TestOSVersions_HappyPath(t *testing.T) {
	versions, err := OSVersions("Microsoft Windows Server 2019 Datacenter", "10.0.17763 Build 17763")
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"Windows", "Windows-Server", "Windows-Server-17763"},
		versions,
	)
}

func TestOSVersions_CantParsePlatform_ReturnsError(t *testing.T) {
	_, err := OSVersions("Schlockosoft Grindows", "10.0.17763 Build 17763")
	require.Error(t, err)
}

func TestGPUsFindsVendorButNotDeviceID_ReturnsUnknown(t *testing.T) {
	assert.Equal(
		t,
		GPUs(
			[]GPUQueryResult{
				{
					DriverVersion: "",
					PNPDeviceID:   `PCI\VEN_80A1&SUBSYS_20898086&REV_00\3&11583659&1&10`,
				},
			},
		),
		[]string{"80a1", "80a1:UNKNOWN"}, // Note lowercase "a" as well.
	)
}

func TestGPUsFindsDeviceIDAndVendorAndVersion(t *testing.T) {
	assert.Equal(
		t,
		GPUs(
			[]GPUQueryResult{
				{
					DriverVersion: "1.2.3",
					PNPDeviceID:   `PCI\VEN_8086&DEV_3E9B&SUBSYS_20898086&REV_00\3&11583659&1&10`,
				},
			},
		),
		[]string{"8086", "8086:3e9b", "8086:3e9b-1.2.3"},
	)
}

func TestGPUsFindsNoGPUs(t *testing.T) {
	assert.Equal(
		t,
		GPUs(
			[]GPUQueryResult{},
		),
		[]string{"none"},
	)
}

func TestGPUsSeesVMWare_ReturnsNone(t *testing.T) {
	assert.Equal(
		t,
		GPUs(
			[]GPUQueryResult{
				{
					DriverVersion: "1.2.3",
					PNPDeviceID:   `PCI\VEN_` + strings.ToUpper(gpus.VMWare) + `&DEV_3E9B&SUBSYS_20898086&REV_00\3&11583659&1&10`,
				},
			},
		),
		[]string{"none"},
	)
}
