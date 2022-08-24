package mac

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGPUsFromSystemProfilerXML_HappyPath(t *testing.T) {
	unittest.SmallTest(t)
	// This is pared down to the used fields plus one or two more at each level to make sure unused
	// ones get ignored. Additionally, the used one spdisplays_vendor-id is missing in one element
	// to make sure missing fields turned into empty strings.
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<array>
	<dict>
		<key>_SPCommandLineArguments</key>
		<array>
			<string>/usr/sbin/system_profiler</string>
			<string>-nospawn</string>
			<string>-xml</string>
			<string>SPDisplaysDataType</string>
			<string>-detailLevel</string>
			<string>full</string>
		</array>
		<key>_SPCompletionInterval</key>
		<real>0.38680398464202881</real>
		<key>_items</key>
		<array>
			<dict>
				<key>_name</key>
				<string>kHW_IntelUHDGraphics630Item</string>
				<key>_spdisplays_vram</key>
				<string>1536 MB</string>
				<key>spdisplays_device-id</key>
				<string>0x3e9b</string>
				<key>spdisplays_gmux-version</key>
				<string>5.0.0</string>
				<key>spdisplays_vendor</key>
				<string>Intel</string>
				<key>sppci_model</key>
				<string>Intel UHD Graphics 630</string>
				<key>spdisplays_vendor-id</key>
				<string>12345</string>
			</dict>
			<dict>
				<key>_name</key>
				<string>kHW_AMDRadeonPro5300MItem</string>
				<key>spdisplays_device-id</key>
				<string>0x7340</string>
				<key>spdisplays_gmux-version</key>
				<string>5.0.0</string>
				<key>spdisplays_vendor</key>
				<string>sppci_vendor_amd</string>
				<key>sppci_model</key>
				<string>AMD Radeon Pro 5300M</string>
			</dict>
		</array>
		<key>_parentDataType</key>
		<string>SPHardwareDataType</string>
	</dict>
</array>
</plist>`
	gpus, err := GPUsFromSystemProfilerXML(xml)
	assert.NoError(t, err)
	expected := []GPU{
		{
			ID:       "0x3e9b",
			VendorID: "12345",
			Vendor:   "Intel",
			Version:  "5.0.0",
			Model:    "Intel UHD Graphics 630",
		},
		{
			ID:       "0x7340",
			VendorID: "",
			Vendor:   "sppci_vendor_amd",
			Version:  "5.0.0",
			Model:    "AMD Radeon Pro 5300M",
		},
	}
	assert.Equal(t, expected, gpus)
}

func TestGPUsFromSystemProfilerXML_BadXML_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	xml := `invalid XML`
	_, err := GPUsFromSystemProfilerXML(xml)
	assert.Error(t, err)
}

func TestGPUsFromSystemProfilerXML_EmptyOuterArray_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<array>
</array>
</plist>`
	_, err := GPUsFromSystemProfilerXML(xml)
	assert.Error(t, err)
}

func TestDimensionsFromMacGPUs_EmptyIDYieldsNoDimensions(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(
		t,
		[]string{"none"},
		DimensionsFromGPUs([]GPU{{
			ID:       "",
			VendorID: "blah",
			Vendor:   "blah",
			Version:  "blah",
			Model:    "blah",
		}}),
	)
}

func TestDimensionsFromMacGPUs_VendorIDAndGPUIDGet0xPrefixRemovedAndGPUVersionGetsParsed(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(
		t,
		[]string{"1234", "1234:5678", "1234:5678-9.8.7-6.5.4"},
		DimensionsFromGPUs([]GPU{{
			ID:       "0x5678",
			VendorID: "0x1234",
			Vendor:   "blah",
			Version:  "9.8.7 [6.5.4]",
			Model:    "Schlocko Pencil 2000",
		}}),
	)
}

func TestDimensionsFromMacGPUs_ExtractVendorIDFromVendor(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(
		t,
		[]string{"eeee", "eeee:5678"},
		DimensionsFromGPUs([]GPU{{
			ID:       "0x5678",
			VendorID: "",
			Vendor:   "Something (0xeeee)",
			Version:  "",
			Model:    "",
		}}),
	)
}

func TestDimensionsFromMacGPUs_FindsVendorIDBasedOnVendorNameExtractedFromModel(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(
		t,
		[]string{"10de", "10de:5678"},
		DimensionsFromGPUs([]GPU{{
			ID:       "0x5678",
			VendorID: "", // must be so
			Vendor:   "",
			Version:  "",
			Model:    "Nvidia Hoodoo",
		}}),
	)
}

func TestDimensionsFromMacGPUs_FindsVendorIDBasedOnVendorNameExtractedFromVendorField(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(
		t,
		[]string{"10de", "10de:5678"},
		DimensionsFromGPUs([]GPU{{
			ID:       "0x5678",
			VendorID: "",
			Vendor:   "sppci_vendor_nvidia",
			Version:  "",
			Model:    "",
		}}),
	)
}

func TestDimensionsFromMacGPUs_SetsVendorIDToUnknownIfAllElseFails(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(
		t,
		[]string{"UNKNOWN", "UNKNOWN:5678"},
		DimensionsFromGPUs([]GPU{{
			ID:       "0x5678",
			VendorID: "",
			Vendor:   "sppci_vendor_noSuchCompany",
			Version:  "",
			Model:    "",
		}}),
	)
}

func TestDimensionsFromMacGPUs_YieldsNoDimensionsOnVMWare(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(
		t,
		[]string{"none"},
		DimensionsFromGPUs([]GPU{{
			ID:       "0xeeee",
			VendorID: "0x15ad", // VMWare's vendor ID
			Vendor:   "",
			Version:  "",
			Model:    "",
		}}),
	)
}
