package imports

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

// Any package named testutil(s) and the packages in the following list are
// allowed to import other test packages, but non-test packages are not allowed
// to import them.
var testPackages = []string{
	"github.com/stretchr/testify/assert",
	"github.com/stretchr/testify/require",
	"go.skia.org/infra/go/deepequal/assertdeep",
	"go.skia.org/infra/go/git/testutils/mem_git",
	"go.skia.org/infra/go/mockhttpclient",
	"go.skia.org/infra/golden/go/mocks",
	"go.skia.org/infra/task_scheduler/go/scheduling/perftest",
	"testing",
	"go.skia.org/infra/perf/go/shortcut/shortcuttest",
	"go.skia.org/infra/perf/go/sql/sqltest",
	"go.skia.org/infra/perf/go/alerts/alertstest",
}

// TODO(borenet): this list should be empty.
// items should not be added to this list, only removed.
var legacyTestImportExceptions = map[string][]string{
	"go.skia.org/infra/task_driver/go/td": {
		"github.com/stretchr/testify/require",
	},
	"go.skia.org/infra/task_scheduler/go/db": {
		"github.com/stretchr/testify/require",
		"go.skia.org/infra/go/deepequal/assertdeep",
		"go.skia.org/infra/go/git/testutils",
		"go.skia.org/infra/go/testutils",
	},
	"go.skia.org/infra/task_scheduler/go/isolate_cache": {
		"github.com/stretchr/testify/require",
		"go.skia.org/infra/go/bt/testutil",
	},
	"go.skia.org/infra/task_scheduler/go/task_scheduler": {
		"go.skia.org/infra/task_scheduler/go/testutils",
	},
	"go.skia.org/infra/task_scheduler/go/tryjobs": {
		"github.com/stretchr/testify/require",
		"go.skia.org/infra/go/depot_tools/testutils",
		"go.skia.org/infra/go/git/testutils",
		"go.skia.org/infra/go/mockhttpclient",
		"go.skia.org/infra/go/testutils",
		"go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils",
	},
}

// isTestPackage returns true if the given package should be considered a test
// package, ie. it is allowed to import other test packages and non-test
// packages are not allowed to import it.
func isTestPackage(pkg string) bool {
	return (strings.HasSuffix(pkg, "shared_tests") ||
		strings.HasSuffix(pkg, "testutil") ||
		strings.HasSuffix(pkg, "testutils") ||
		util.In(pkg, testPackages))
}

// testImportAllowed returns true if the given non-test package is whitelisted
// to import the given test package.
func testImportAllowed(importer, importee string) bool {
	return util.In(importee, legacyTestImportExceptions[importer])
}

// TestNoPackageImportsTesting asserts that no non-test package imports a test
// package, with the exception of those listed in legacyTestImportExceptions.
func TestNoPackageImportsTesting(t *testing.T) {
	unittest.LargeTest(t)

	// Find all packages, categorize as test and non-test.
	pkgs, err := LoadAllPackageData(context.Background())
	assert.NoError(t, err)
	testPkgs := make(map[string]*Package, len(pkgs))
	nonTestPkgs := make(map[string]*Package, len(pkgs))
	for name, pkg := range pkgs {
		if isTestPackage(name) {
			testPkgs[name] = pkg
		} else {
			nonTestPkgs[name] = pkg
		}
	}
	for _, name := range testPackages {
		testPkgs[name] = &Package{} // Not actually used.
	}

	// Ensure that no non-test package imports a test package.
	for name, pkg := range nonTestPkgs {
		for _, imported := range pkg.Imports {
			_, importsTestPkg := testPkgs[imported]
			// TODO: Remove this once legacyTestImportExceptions is empty.
			if importsTestPkg && testImportAllowed(name, imported) {
				importsTestPkg = false
			}
			assert.Falsef(t, importsTestPkg, "Non-test package %s imports test package %s", name, imported)
		}
		// Verify that legacyTestImportExceptions doesn't contain more
		// entries than it should.
		// TODO: Remove this once legacyTestImportExceptions is empty.
		for _, whitelistedTestImport := range legacyTestImportExceptions[name] {
			assert.Truef(t, util.In(whitelistedTestImport, pkg.Imports), "Non-test package %s is whitelisted to import %s but does not.", name, whitelistedTestImport)
		}
	}
}
