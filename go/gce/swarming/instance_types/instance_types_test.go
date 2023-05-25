package instance_types

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
)

func TestFilesCorrectlyEmbedded(t *testing.T) {
	assert.Contains(t, setupScriptLinuxSH, "# Linux machines are configured via Ansible")
	assert.Contains(t, setupScriptLinuxCTSH, "# Setup script for CT machines")
	assert.Contains(t, setupWinPS1, "# Setup script for Windows machines configured via Ansible")
	assert.Contains(t, nodeSetup6xScript, "# Script to install the NodeSource Node.js 6.x")
}

func TestChromeBotSkoloPassword(t *testing.T) {
	secretsYml := `secrets:
  skolo_password: 'FakePassword'
`

	ansibleSecretVarsYml := `apiVersion: v1
data:
  secrets.yml: ` + base64.StdEncoding.EncodeToString([]byte(secretsYml)) + `
kind: Secret
metadata:
  creationTimestamp: null
  name: ansible-secret-vars
`

	berglasOutput := base64.StdEncoding.EncodeToString([]byte(ansibleSecretVarsYml))

	ctx := exec.NewContext(context.Background(), func(ctx context.Context, c *exec.Command) error {
		assert.Equal(t, "berglas", c.Name)
		assert.Equal(t, []string{"access", "skia-secrets/etc/ansible-secret-vars"}, c.Args)
		_, err := c.CombinedOutput.Write([]byte(berglasOutput))
		require.NoError(t, err)
		return nil
	})

	password, err := getChromeBotSkoloPassword(ctx)
	require.NoError(t, err)
	assert.Equal(t, "FakePassword", password)
}
