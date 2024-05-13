package swarmingv2

import (
	"testing"

	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

func TestBotDimensionsToStringMap(t *testing.T) {
	input := []*apipb.StringListPair{
		{
			Key:   "os",
			Value: []string{"Linux"},
		},
		// Ensure that we handle multiple entries with the same key.
		{
			Key:   "os",
			Value: []string{"Debian", "Debian-13"},
		},
		{
			Key:   "gpu",
			Value: []string{"none"},
		},
	}
	expect := map[string][]string{
		"gpu": {"none"},
		"os": {
			"Linux",
			"Debian",
			"Debian-13",
		},
	}
	require.Equal(t, expect, BotDimensionsToStringMap(input))
}
