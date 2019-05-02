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

var unwantedTestifyPackages = []string{
	"github.com/stretchr/testify/require",
	"github.com/stretchr/testify/assert",
}

func TestNoPackageImportsTestify(t *testing.T) {
	// This is a Manualtest because there's a list of several packages (mostly task_scheduler
	// and task_driver) that still include Require.
	testutils.ManualTest(t)

	for _, p := range unwantedTestifyPackages {

		// Assert that no package has this import
		importers, err := FindImporters(context.Background(), p)
		assert.NoError(t, err)
		badImports := []string{}
		for _, i := range importers {
			if !strings.HasSuffix(i, "testutil") && !strings.HasSuffix(i, "testutils") && !util.In(i, testifyExceptions) {
				badImports = append(badImports, i)
			}
		}
		assert.Emptyf(t, badImports, "No non-test or testutil packages should import %q but the following do:\n%s", p, strings.Join(badImports, "\n"))
	}
}
