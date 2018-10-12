package baseline

import (
	"bytes"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/golden/go/types"
)

func TestMergeableBaseline(t *testing.T) {
	baseLine := types.TestExp{}

	var buf bytes.Buffer
	assert.NoError(t, WriteMergeableBaseline(&buf, baseLine))

	foundBaseLine, err := ReadMergeableBaseline(&buf)
	assert.NoError(t, err)
	assert.Equal(t, baseLine, foundBaseLine)
}
