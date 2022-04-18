package checks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

func TestPerformLicenseCheck(t *testing.T) {
	unittest.SmallTest(t)

	tests := []struct {
		licenseType interface{}
		checkPassed bool
		name        string
	}{
		// Single license strings.
		{
			licenseType: "MIT",
			checkPassed: true,
			name:        "Single license string test with allowed license",
		},
		{
			licenseType: licenseBlockListPrefixes[0], // Banned
			checkPassed: false,
			name:        "Single license string test with banned license",
		},
		// Multi-nested license strings.
		{
			licenseType: "(MIT AND LGPL-2.1 OR (BSD AND MIT)",
			checkPassed: true,
			name:        "Multi-nested license strings test with allowed licenses",
		},
		{
			licenseType: fmt.Sprintf("(MIT AND LGPL-2.1 OR (BSD AND %s)", licenseBlockListPrefixes[0]),
			checkPassed: false,
			name:        "Multi-nested license strings test with one banned license at the end",
		},
		{
			licenseType: fmt.Sprintf("(%s AND LGPL-2.1 OR BSD)", licenseBlockListPrefixes[0]),
			checkPassed: false,
			name:        "Multi-nested license strings test with one banned license at the start",
		},
		{
			licenseType: fmt.Sprintf("(MIT AND %s OR LGPL-2.1 OR BSD)", licenseBlockListPrefixes[0]),
			checkPassed: false,
			name:        "Multi-nested license strings test with one banned license in the middle",
		},
		// Nil license.
		{
			licenseType: nil,
			checkPassed: true,
			name:        "Nil license string",
		},
		// Unknown type license.
		{
			licenseType: true,
			checkPassed: true,
			name:        "Unknown license type",
		},
		// "LICENSE in" text.
		{
			licenseType: "SEE LICENSE IN some file",
			checkPassed: true,
			name:        "\"SEE LICENSE IN ...\" license string",
		},
		// Deprecated license structure only found in old packages.
		{
			licenseType: map[string]interface{}{"type": "MIT", "url": "some-url"},
			checkPassed: true,
			name:        "Deprecated license type with allowed license",
		},
		{
			licenseType: map[string]interface{}{"type": licenseBlockListPrefixes[0], "url": "some-url"},
			checkPassed: false,
			name:        "Deprecated license type with banned license",
		},
	}

	lc := LicenseCheck{}
	testPackageVersion := "1.1.0"
	for _, test := range tests {
		npm := &types.NpmPackage{
			Versions: map[string]types.NpmVersion{
				testPackageVersion: {License: test.licenseType},
			},
		}
		checkPassed, result, err := lc.PerformCheck("test-package", testPackageVersion, npm)
		require.NoError(t, err, test.name)
		require.Equal(t, test.checkPassed, checkPassed, test.name)
		if checkPassed {
			require.Empty(t, result, test.name)
		} else {
			require.NotEmpty(t, result, test.name)
		}

		// Also test with a non-existant license version.
		checkPassed, result, err = lc.PerformCheck("test-package", "does-not-exist", npm)
		require.NoError(t, err)
		require.True(t, checkPassed)
		require.Empty(t, result)
	}
}
