package linux

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestVendorAndBrand_StopsAfterExhaustingAllLines(t *testing.T) {
	unittest.SmallTest(t)
	_, _, err := VendorAndBrand(strings.NewReader(`processor       : 0
vendor_id       : GenuineIntel
`))
	require.NoError(t, err)
}

func TestVendorAndBrand_StopsAfterBlankLine(t *testing.T) {
	unittest.SmallTest(t)
	vendor, _, err := VendorAndBrand(strings.NewReader(`processor       : 0
vendor_id       : GenuineIntel

processor   : 1
vendor_id: SomeOtherCompany
`))
	require.NoError(t, err)
	assert.Equal(t, "GenuineIntel", vendor)
}

func TestVendorAndBrand_SkipsLinesWithoutColonsAndToleratesColonsInValues(t *testing.T) {
	unittest.SmallTest(t)
	vendor, _, err := VendorAndBrand(strings.NewReader(`vendor_id: Vendor: The Next Generation
vendor_id
`))
	require.NoError(t, err)
	// If it didn't skip the colonless line, vendor would have been replaced with "".
	assert.Equal(t, "Vendor: The Next Generation", vendor)
}

func TestVendorAndBrand_HardwareTrimmingAndBrandStringFallbacks(t *testing.T) {
	// Also test well-formed keys with empty values while we're at it.
	unittest.SmallTest(t)
	vendor, brandString, err := VendorAndBrand(strings.NewReader(`wellFormedKeyWithEmptyValue:
Hardware       : Toaster (Flattened Device Tree)
`))
	require.NoError(t, err)
	assert.Equal(t, "N/A", vendor)
	assert.Equal(t, "Toaster", brandString)
}

func TestOSVersions_CapitalizesPlatform(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, []string{"Linux", "Greeb", "Greeb-4", "Greeb-4.3"}, OSVersions("greeb", "4.3"))
}
