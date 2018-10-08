package imports

import (
	"context"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestNoPackageImportsTesting(t *testing.T) {
	testutils.LargeTest(t)

	// Assert that no package imports "testing".
	importers, err := FindImporters(context.Background(), "testing")
	assert.NoError(t, err)
	assert.Emptyf(t, importers, "No non-test packages should import \"testing\" but the following do:\n%s", strings.Join(importers, "\n"))
}
