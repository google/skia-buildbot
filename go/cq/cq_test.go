package cq

import (
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

func TestCQTryBots(t *testing.T) {
	testutils.SkipIfShort(t)

	client := NewClient()
	_, err := client.getCQBuilders()
	assert.NoError(t, err)
	// assert.Equal(t, 1, len(tries))
}
