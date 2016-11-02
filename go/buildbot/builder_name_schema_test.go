package buildbot

import (
	"testing"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func TestBuilderNameSchema(t *testing.T) {
	testutils.SmallTest(t)
	tc := map[string]map[string]string{
		"Build-Ubuntu-GCC-x86-Release": {
			"role":          "Build",
			"os":            "Ubuntu",
			"compiler":      "GCC",
			"target_arch":   "x86",
			"configuration": "Release",
			"is_trybot":     "false",
		},
		"Build-Ubuntu-GCC-x86-Release-Trybot": {
			"role":          "Build",
			"os":            "Ubuntu",
			"compiler":      "GCC",
			"target_arch":   "x86",
			"configuration": "Release",
			"is_trybot":     "true",
		},
		"Build-Ubuntu-GCC-x86-Debug-Android": {
			"role":          "Build",
			"os":            "Ubuntu",
			"compiler":      "GCC",
			"target_arch":   "x86",
			"configuration": "Debug",
			"is_trybot":     "false",
			"extra_config":  "Android",
		},
		"Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Debug-CT_DM_1m_SKPs": {
			"role":             "Test",
			"os":               "Ubuntu",
			"compiler":         "GCC",
			"model":            "GCE",
			"cpu_or_gpu":       "CPU",
			"cpu_or_gpu_value": "AVX2",
			"arch":             "x86_64",
			"configuration":    "Debug",
			"extra_config":     "CT_DM_1m_SKPs",
			"is_trybot":        "false",
		},
	}
	for builderName, params := range tc {
		res, err := ParseBuilderName(builderName)
		if params == nil {
			assert.NotNil(t, err)
		} else {
			assert.Equal(t, params, res)
		}
	}
}
