package taskname

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskNameSchema(t *testing.T) {
	tc := map[string]map[string]string{
		"Build-Ubuntu-GCC-x86-Release": {
			"role":          "Build",
			"os":            "Ubuntu",
			"compiler":      "GCC",
			"target_arch":   "x86",
			"configuration": "Release",
		},
		"Build-Ubuntu-GCC-x86-Debug-Android": {
			"role":          "Build",
			"os":            "Ubuntu",
			"compiler":      "GCC",
			"target_arch":   "x86",
			"configuration": "Debug",
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
			"test_filter":      "CT_DM_1m_SKPs",
		},
		"Upload-Test-Android-Clang-Nexus6p-GPU-Adreno430-arm64-Release-All-Android_Vulkan": {
			"role":             "Upload",
			"sub-role-1":       "Test",
			"os":               "Android",
			"compiler":         "Clang",
			"model":            "Nexus6p",
			"cpu_or_gpu":       "GPU",
			"cpu_or_gpu_value": "Adreno430",
			"arch":             "arm64",
			"configuration":    "Release",
			"test_filter":      "All",
			"extra_config":     "Android_Vulkan",
		},
	}
	p := DefaultTaskNameParser()
	for builderName, params := range tc {
		res, err := p.ParseTaskName(builderName)
		require.NoError(t, err)
		require.Equal(t, params, res)
	}
}

func TestBadTaskNameSchema(t *testing.T) {
	tc := []string{
		"Alpha-Ubuntu-GCC-x86-Release",
		"Build",
		"Build-Ubuntu-GCC-x86-Debug-Android-Way-Too-Many-Extras",
		"",
	}
	p := DefaultTaskNameParser()
	for _, builderName := range tc {
		_, err := p.ParseTaskName(builderName)
		require.Error(t, err)
	}
}
