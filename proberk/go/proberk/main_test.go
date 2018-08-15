package main

import (
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/proberk/go/types"
)

func TestProbeSSL(t *testing.T) {
	probes := &types.Probe{
		URLs: []string{
			"https://skia.org",
			"https://skia.org:443",
			"https://35.201.76.220",
		},
		Method: "SSL",
	}

	for _, url := range probes.URLs {
		assert.NoError(t, probeSSL(probes, url))
	}

	// Expect certs that are valid for 3000 days.
	probes.Expected = []int{3000}
	for _, url := range probes.URLs {
		assert.Error(t, probeSSL(probes, url))
	}
}
