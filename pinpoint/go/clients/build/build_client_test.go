package build

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_ChromeProject_ChromeClient(t *testing.T) {
	ctx := context.Background()

	client, err := NewBuildClient(ctx, "chrome")
	require.NoError(t, err)
	assert.IsType(t, &buildChromeClient{}, client)
}
