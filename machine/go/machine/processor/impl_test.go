package processor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestParseAndroidProperties_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	const adbResponseHappyPath = `
[ro.product.manufacturer]: [asus]
[ro.product.model]: [Nexus 7]
[ro.product.name]: [razor]
`
	want := map[string]string{
		"ro.product.manufacturer": "asus",
		"ro.product.model":        "Nexus 7",
		"ro.product.name":         "razor",
	}
	got := parseAndroidProperties(adbResponseHappyPath)
	assert.Equal(t, want, got)
}

func TestParseAndroidProperties_EmptyStringGivesEmptyMap(t *testing.T) {
	unittest.SmallTest(t)

	assert.Empty(t, parseAndroidProperties(""))
}

func TestDimensionsFromAndroidProperties_Success(t *testing.T) {
	unittest.SmallTest(t)

	adbResponse := strings.Join([]string{
		"[ro.product.manufacturer]: [Google]", // Ignored
		"[ro.product.model]: [Pixel 3a]",      // Ignored
		"[ro.build.id]: [QQ2A.200305.002]",    // device_os
		"[ro.product.brand]: [google]",        // device_os_flavor
		"[ro.build.type]: [user]",             // device_os_type
		"[ro.product.device]: [sargo]",        // device_type
		"[ro.product.system.brand]: [google]", // device_os_flavor (dup should be ignored)
		"[ro.product.system.brand]: [aosp]",   // device_os_flavor (should be converted to "android")
	}, "\n")

	dimensions := parseAndroidProperties(adbResponse)
	got := dimensionsFromAndroidProperties(dimensions)

	expected := map[string][]string{
		"android_devices":  {"1"},
		"device_os":        {"Q", "QQ2A.200305.002"},
		"device_os_flavor": {"google", "android"},
		"device_os_type":   {"user"},
		"device_type":      {"sargo"},
		"os":               {"Android"},
	}
	assert.Equal(t, expected, got)
}

func TestDimensionsFromAndroidProperties_EmptyFromEmpty(t *testing.T) {
	unittest.SmallTest(t)

	dimensions := parseAndroidProperties("")
	assert.Empty(t, dimensionsFromAndroidProperties(dimensions))
}
