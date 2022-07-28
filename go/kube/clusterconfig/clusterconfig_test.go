package clusterconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNewFromEmbeddedConfig_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	cfg, err := NewFromEmbeddedConfig()
	require.NoError(t, err)
	require.Equal(t, "https://skia.googlesource.com/k8s-config", cfg.Repo)
}
