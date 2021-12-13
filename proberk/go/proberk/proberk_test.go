package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/proberk/go/types"
)

func TestProbeSSL(t *testing.T) {
	t.Skip()
	unittest.LargeTest(t)
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
		require.NoError(t, probeSSL(probes, url))
	}

	// Verify failure by expecting certs to be valid for 20 years.
	probes.Expected = []int{7300}
	for _, url := range probes.URLs {
		require.Error(t, probeSSL(probes, url))
	}
}
