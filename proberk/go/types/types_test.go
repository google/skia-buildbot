package types

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestLoadFromJSONFile_FileViolatesSchema_ReturnsError(t *testing.T) {
	unittest.MediumTest(t)
	_, err := LoadFromJSONFile(context.Background(), "./testdata/invalid.json")
	require.Error(t, err)
}

func TestLoadFromJSONFile_ValidFile_Success(t *testing.T) {
	unittest.MediumTest(t)
	probers, err := LoadFromJSONFile(context.Background(), "./testdata/probersk.json")
	require.NoError(t, err)
	require.Len(t, probers, 1)
}
