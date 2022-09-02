package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

type testCommonConfig struct {
	CommonString     string `json:"common_str"`
	CommonInt        int    `json:"common_int"`
	CommonBool       bool   `json:"common_bool"`
	WillBeOverridden string `json:"will_be_overridden"`
}

type testSpecificConfig struct {
	testCommonConfig
	Unique string `json:"unique"`

	OptionalDuration Duration `json:"optional_duration" optional:"true"`
}

func TestLoadFromJSON5_Success(t *testing.T) {

	td := testutils.TestDataDir(t)
	commonPath := filepath.Join(td, "common.json5")
	specificPath := filepath.Join(td, "specific.json5")

	var tsc testSpecificConfig
	err := LoadFromJSON5(&tsc, &commonPath, &specificPath)
	require.NoError(t, err)

	assert.Equal(t, testSpecificConfig{
		testCommonConfig: testCommonConfig{
			CommonString:     "somestring",
			CommonInt:        1234,
			CommonBool:       false,
			WillBeOverridden: "7890",
		},
		Unique: "1234",
	}, tsc)
}

func TestLoadFromJSON5_WithDuration_Success(t *testing.T) {

	td := testutils.TestDataDir(t)
	commonPath := filepath.Join(td, "common.json5")
	specificPath := filepath.Join(td, "specific_duration.json5")

	var tsc testSpecificConfig
	err := LoadFromJSON5(&tsc, &commonPath, &specificPath)
	require.NoError(t, err)

	assert.Equal(t, testSpecificConfig{
		testCommonConfig: testCommonConfig{
			CommonString:     "somestring",
			CommonInt:        1234,
			CommonBool:       false,
			WillBeOverridden: "7890",
		},
		Unique:           "1234",
		OptionalDuration: Duration{Duration: 3 * time.Hour},
	}, tsc)
}

func TestLoadFromJSON5_RequiredFieldMissing_Error(t *testing.T) {

	td := testutils.TestDataDir(t)
	commonPath := filepath.Join(td, "common_missing_field.json5")
	specificPath := filepath.Join(td, "specific.json5")

	var tsc testSpecificConfig
	err := LoadFromJSON5(&tsc, &commonPath, &specificPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CommonInt")
}
