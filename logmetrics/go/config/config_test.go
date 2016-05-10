package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigRead(t *testing.T) {
	m, err := ReadMetrics(filepath.Join("./testdata", "metrics.cfg"))
	assert.Nil(t, err)
	assert.Equal(t, 2, len(m))
	assert.Equal(t, "qps", m[0].Name)
	assert.Equal(t, "fiddle-sec-violations", m[1].Name)
}
