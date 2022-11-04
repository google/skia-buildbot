package sser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPeerFinderLocalhost_Start_AlwaysReturnsTheSameValues(t *testing.T) {
	ips, _, err := PeerFinderLocalhost{}.Start(context.Background())
	require.Equal(t, []string{"127.0.0.1"}, ips)
	require.NoError(t, err)
}
