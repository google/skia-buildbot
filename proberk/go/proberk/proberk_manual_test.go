package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/proberk/go/types"
)

func TestProbeSSL_UseDefaultValues_Success(t *testing.T) {
	probes := &types.Probe{
		URLs: []string{
			"https://skia.org",
			"https://skia.org:443",
			"https://35.201.76.220",
		},
		Method: "SSL",
	}

	// Verify the Certs are valid. This implies they are valid for 10 days.
	for _, url := range probes.URLs {
		assert.NoError(t, probeSSL(probes, url))
	}
}

func TestProbeSSL_UseVeryBigExpectedTime_ReturnsError(t *testing.T) {
	probes := &types.Probe{
		Expected: []int{7300}, // 20 years - no cert should be valid that long
		URLs: []string{
			"https://skia.org",
			"https://skia.org:443",
			"https://35.201.76.220",
		},
		Method: "SSL",
	}

	for _, url := range probes.URLs {
		assert.Error(t, probeSSL(probes, url))
	}
}
