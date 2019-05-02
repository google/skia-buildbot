package imports

import (
	"context"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func TestNoPackageImportsTesting(t *testing.T) {
	testutils.LargeTest(t)

	// Assert that no package imports "testing".
	importers, err := FindImporters(context.Background(), "testing")
	assert.NoError(t, err)
	assert.Emptyf(t, importers, "No non-test packages should import \"testing\" but the following do:\n%s", strings.Join(importers, "\n"))
}

// Any package named testutil(s) and the packages in the
// following list are allowed to import testify
var testifyExceptions = []string{
	"go.skia.org/infra/go/deepequal",
	"go.skia.org/infra/go/mockhttpclient",
	"go.skia.org/infra/golden/go/mocks",
}

func TestNoPackageImportsTestifyRequire(t *testing.T) {
	// This is a Manualtest because there's a list of several packages (mostly task_scheduler
	// and task_driver) that still include Require.
	testutils.ManualTest(t)

	// Assert that no package imports "testing".
	importers, err := FindImporters(context.Background(), "github.com/stretchr/testify/require")
	assert.NoError(t, err)
	badImports := []string{}
	for _, i := range importers {
		if !strings.HasSuffix(i, "testutil") && !strings.HasSuffix(i, "testutils") && !util.In(i, testifyExceptions) {
			badImports = append(badImports, i)
		}
	}
	assert.Emptyf(t, badImports, "No non-test or testutil packages should import \"github.com/stretchr/testify/require\" but the following do:\n%s", strings.Join(badImports, "\n"))
}
