package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/golden/go/config/validation"
)

func TestLoadExistingConfigs_Valid(t *testing.T) {
	var cfg baselineServerConfig
	err := validation.ValidateServiceConfigs("baselineserver", validation.PrimaryInstances, &cfg)
	require.NoError(t, err)
	assert.NotZero(t, cfg, "Config object should not be nil.")
}
