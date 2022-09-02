package linux

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVendorAndBrand_StopsAfterExhaustingAllLines(t *testing.T) {
	_, _, err := VendorAndBrand(strings.NewReader(`processor       : 0
vendor_id       : GenuineIntel
`))
	require.NoError(t, err)
}

func TestVendorAndBrand_StopsAfterBlankLine(t *testing.T) {
	vendor, _, err := VendorAndBrand(strings.NewReader(`processor       : 0
vendor_id       : GenuineIntel

processor   : 1
vendor_id: SomeOtherCompany
`))
	require.NoError(t, err)
	assert.Equal(t, "GenuineIntel", vendor)
}

func TestVendorAndBrand_SkipsLinesWithoutColonsAndToleratesColonsInValues(t *testing.T) {
	vendor, _, err := VendorAndBrand(strings.NewReader(`vendor_id: Vendor: The Next Generation
vendor_id
`))
	require.NoError(t, err)
	// If it didn't skip the colonless line, vendor would have been replaced with "".
	assert.Equal(t, "Vendor: The Next Generation", vendor)
}

func TestVendorAndBrand_HardwareTrimmingAndBrandStringFallbacks(t *testing.T) {
	// Also test well-formed keys with empty values while we're at it.
	vendor, brandString, err := VendorAndBrand(strings.NewReader(`wellFormedKeyWithEmptyValue:
Hardware       : Toaster (Flattened Device Tree)
`))
	require.NoError(t, err)
	assert.Equal(t, "N/A", vendor)
	assert.Equal(t, "Toaster", brandString)
}

func TestVendorAndBrand_ReturnsBrandStringWhenVendorIDIsFound(t *testing.T) {
	vendor, brandString, err := VendorAndBrand(strings.NewReader(`vendor_id       : GenuineIntel
model name      : Intel(R) Pentium(R) CPU  N3700  @ 1.60GHz
`))
	require.NoError(t, err)
	assert.Equal(t, "GenuineIntel", vendor)
	assert.Equal(t, "Intel(R) Pentium(R) CPU  N3700  @ 1.60GHz", brandString)
}

func TestOSVersions_CapitalizesPlatform(t *testing.T) {
	assert.Equal(t, []string{"Linux", "Greeb", "Greeb-4", "Greeb-4.3"}, OSVersions("greeb", "4.3"))
}

func version123() string {
	return "1.2.3"
}

func version456(context.Context) string {
	return "4.5.6"
}

func TestGPUs_MultipleGPUsDetectedAndNonGPUDevicesSkipped(t *testing.T) {
	// Also tests full-length realistic lspci output.
	gpus, err := GPUs(
		context.Background(),
		`00:00.0 "Host bridge [0600]" "Intel Corporation [8086]" "Atom/Celeron/Pentium Processor x5-E8000/J3xxx/N3xxx Series SoC Transaction Register [2280]" -r21 "Intel Corporation [8086]" "Atom/Celeron/Pentium Processor x5-E8000/J3xxx/N3xxx Series SoC Transaction Register [2060]"
00:02.0 "VGA compatible controller [0300]" "Intel Corporation [8086]" "Atom/Celeron/Pentium Processor x5-E8000/J3xxx/N3xxx Integrated Graphics Controller [22b1]" -r21 "Intel Corporation [8086]" "Atom/Celeron/Pentium Processor x5-E8000/J3xxx/N3xxx Integrated Graphics Controller [2060]"
00:04.0 "Host bridge [0600]" "Intel Corporation [8086]" "Atom/Celeron/Pentium Processor x5-E8000/J3xxx/N3xxx Series SoC Transaction Register [2280]" -r21 "Intel Corporation [8086]" "Atom/Celeron/Pentium Processor x5-E8000/J3xxx/N3xxx Series SoC Transaction Register [2060]"
01:00.0 "VGA compatible controller [0300]" "NVIDIA Corporation [10de]" "Device [2489]" -ra1 "ASUSTeK Computer Inc. [1043]" "Device [884f]"
`,
		version123,
		version456,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"8086", "8086:22b1", "8086:22b1-4.5.6", "10de", "10de:2489", "10de:2489-1.2.3"}, gpus)
}

func TestGPUs_GPUHasBadVendorFormat_GetsSkipped(t *testing.T) {
	// This also tests the case in which no GPUs are returned.
	gpus, err := GPUs(
		context.Background(),
		`00:02.0 "VGA compatible controller [0300]" "Karnov [No ID Here]" "Atom/Celeron/Pentium Processor x5-E8000/J3xxx/N3xxx Integrated Graphics Controller [22b1]"
`,
		version123,
		version456,
	)
	require.NoError(t, err)
	assert.Equal(t, []string(nil), gpus)
}

func TestGPUs_NonIntelOrNvidiaVendor_OmitsVersion(t *testing.T) {
	gpus, err := GPUs(
		context.Background(),
		`00:02.0 "VGA compatible controller [0300]" "Schlocko Corporation [1111]" "Atom/Celeron/Pentium Processor x5-E8000/J3xxx/N3xxx Integrated Graphics Controller [3333]"
`,
		version123,
		version456,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"1111", "1111:3333"}, gpus)
}
