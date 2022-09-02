package clusterconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFromEmbeddedConfig_HappyPath(t *testing.T) {

	cfg, err := NewFromEmbeddedConfig()
	require.NoError(t, err)
	require.Equal(t, "https://skia.googlesource.com/k8s-config", cfg.Repo)
}
