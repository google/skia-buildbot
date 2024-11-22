package testing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
)

func NewSecretsContextForTesting(t *testing.T, ctx context.Context) context.Context {
	secretsYml := `secrets:
  skolo_password: 'FakePassword'
`
	return exec.NewContext(ctx, func(ctx context.Context, c *exec.Command) error {
		assert.Equal(t, "gcloud", c.Name)
		assert.Equal(t, []string{"--project=skia-infra-public", "secrets", "versions", "access", "latest", "--secret=ansible-secret-vars"}, c.Args)
		_, err := c.CombinedOutput.Write([]byte(secretsYml))
		require.NoError(t, err)
		return nil
	})
}
