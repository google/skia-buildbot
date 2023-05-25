package instance_types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	instance_types_testing "go.skia.org/infra/go/gce/swarming/instance_types/testing"
)

func TestFilesCorrectlyEmbedded(t *testing.T) {
	assert.Contains(t, setupScriptLinuxSH, "# Linux machines are configured via Ansible")
	assert.Contains(t, setupScriptLinuxCTSH, "# Setup script for CT machines")
	assert.Contains(t, setupWinPS1, "# Setup script for Windows machines configured via Ansible")
	assert.Contains(t, nodeSetup6xScript, "# Script to install the NodeSource Node.js 6.x")
}

func TestChromeBotSkoloPassword(t *testing.T) {
	password, err := getChromeBotSkoloPassword(instance_types_testing.NewBerglasContextForTesting(t, context.Background()))
	require.NoError(t, err)
	assert.Equal(t, "FakePassword", password)
}
