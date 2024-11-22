package upload

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_DefaultProject_BigQueryUploadClient(t *testing.T) {
	ctx := context.Background()

	cfg := &UploadClientConfig{
		Project: "chromeperf",
	}
	client, err := NewUploadClient(ctx, cfg)
	require.NoError(t, err)
	assert.IsType(t, &uploadChromeDataClient{}, client)
}
