package backends

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBigQueryClient_ValidProject_Client(t *testing.T) {
	ctx := context.Background()
	client, err := NewBigQueryClient(ctx, "chromeperf")
	require.NoError(t, err)
	assert.NotNil(t, client)
}
