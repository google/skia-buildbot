package types

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromJSONFile_FileViolatesSchema_ReturnsError(t *testing.T) {
	_, err := LoadFromJSONFile(context.Background(), "./testdata/invalid.json")
	require.Error(t, err)
}

func TestLoadFromJSONFile_ValidFile_Success(t *testing.T) {
	probers, err := LoadFromJSONFile(context.Background(), "./testdata/probersk.json")
	require.NoError(t, err)
	require.Len(t, probers, 1)
}
