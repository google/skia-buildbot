package testing

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
)

func NewBerglasContextForTesting(t *testing.T, ctx context.Context) context.Context {
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

	return exec.NewContext(ctx, func(ctx context.Context, c *exec.Command) error {
		assert.Equal(t, "berglas", c.Name)
		assert.Equal(t, []string{"access", "skia-secrets/etc/ansible-secret-vars"}, c.Args)
		_, err := c.CombinedOutput.Write([]byte(berglasOutput))
		require.NoError(t, err)
		return nil
	})
}
