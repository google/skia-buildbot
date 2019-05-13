package config

import (
	"path/filepath"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestConfigRead(t *testing.T) {
	unittest.SmallTest(t)
	m, err := ReadMetrics(filepath.Join("./testdata", "metrics.json5"))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(m))
	assert.Equal(t, "qps", m[0].Name)
	assert.Equal(t, "fiddle-sec-violations", m[1].Name)
}
