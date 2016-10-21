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

	// TODO(rmistry): Move to a different test?
	err = client.ReportCQStats(tryBots, 3722)
	assert.NoError(t, err)
}
