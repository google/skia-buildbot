package cq

import (
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

func TestGetCQTryBots(t *testing.T) {
	testutils.SkipIfShort(t)

	client := NewClient()
	tryBots, err := client.GetCQTryBots()
	assert.NoError(t, err)
	assert.True(t, len(tryBots) > 1)
}
