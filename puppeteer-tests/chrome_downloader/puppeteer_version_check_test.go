// This package defines a test that ensures the "puppeteer" and "puppeteer-core" NPM packages have
// the same exact version. This ensures that chrome_downloader.ts faithfully reproduces Puppeteer's
// post-install hook. See //.puppeteerrc.js for additional context.

package chrome_downloader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/bazel/go/bazel"
)

type packageJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func TestPuppeteerAndPuppeteerCoreNPMPackagesVersionsMatch(t *testing.T) {
	deps := parseDepsFromPackageJSONFile(t)

	puppeteerVersion, ok := deps["puppeteer"]
	require.True(t, ok)
	puppeteerCoreVersion, ok := deps["puppeteer-core"]
	require.True(t, ok)

	// Package versions must match.
	assert.Equal(
		t,
		puppeteerVersion,
		puppeteerCoreVersion,
		`NPM packages "puppeteer" and "puppeteer-core" must be the same version.

    "puppeteer" NPM package version:      %s
    "puppeteer-core" NPM package version: %s

To fix this issue, update the above NPM packages via the "npm" command, e.g.:

    $ bazel run --config=remote //:npm -- install --save-exact puppeteer@<version>
    $ bazel run --config=remote //:npm -- install --save-exact puppeteer-core@<version>

See the following links for a list of valid versions:

    - https://www.npmjs.com/package/puppeteer?activeTab=versions"
    - https://www.npmjs.com/package/puppeteer-core?activeTab=versions
`,
		puppeteerVersion,
		puppeteerCoreVersion)

	// It is important that we specify an exact version, or (at least in theory) NPM might resolve
	// slightly different versions of the puppeteer and pupppeteer-core packages.
	assert.Regexp(t, `^[0-9].*$`, puppeteerVersion, "Puppeteer version specifier must be exact (no range prefixes such as ^ or ~).")

}

func parseDepsFromPackageJSONFile(t *testing.T) map[string]string {
	b, err := os.ReadFile(filepath.Join(bazel.TestWorkspaceDir(), "package.json"))
	require.NoError(t, err)
	var packageJSON struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	require.NoError(t, json.Unmarshal(b, &packageJSON))
	return packageJSON.Dependencies
}
