package checks

import (
	"fmt"
	"regexp"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

var (
	// This block list was populated by using go/sk-npm-audit-mirror-license-list
	// and looking up those license's identifiers on https://spdx.org/licenses/
	licenseBlockListPrefixes = []string{"AGPL", "OSL", "SSPL", "BUSL-1.1", "CAL-", "CPAL-", "CPOL-", "EUPL-", "SISL", "Watcom-1.0", "CC-BY-NC-"}
)

func NewLicenceCheck() types.Check {
	return &LicenseCheck{}
}

// LicenseCheck implements the types.Checks interface.
type LicenseCheck struct {
}

// Name implements the types.Checks interface.
func (lc *LicenseCheck) Name() string {
	return "LicenseCheck"
}

type LegacyNpmLicense struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// PerformCheck implements the types.Checks interface.
// This check looks for 'restricted' licenses. In general we attempt to not be
// overly defensive here. If there is something we do not recognize then we
// error log it and move on.
// NPM license specs are here: https://docs.npmjs.com/cli/v8/configuring-npm/package-json#license
func (lc *LicenseCheck) PerformCheck(packageName, packageVersion string, npmPackage *types.NpmPackage) (bool, string, error) {
	// Can have multiple license types.
	var licenseType string
	switch l := npmPackage.Versions[packageVersion].License.(type) {
	default:
		if l == nil {
			sklog.Warningf("The license for package %s@%s was nil.", packageName, packageVersion)
			return true, "", nil
		}
		sklog.Errorf("Unexpected license type %+v for package %s@%s", npmPackage.Versions[packageVersion].License, packageName, packageVersion)
		return true, "", nil
	case string:
		licenseType = l
	case map[string]interface{}:
		licenseType = l["type"].(string)
	}

	for _, b := range licenseBlockListPrefixes {
		// The License type can have many possible forms-
		// * Single license strings like "BSD-3-Clause".
		// * Can be in a long multi-nested string that looks like-
		//     "(MIT AND (LGPL-2.1+ AND BSD-3-Clause))"
		//     See https://github.com/kemitchell/spdx.js
		// * "SEE LICENSE IN <filename>".
		// * Old packages with a licenses property-
		//     map[type:MIT url:https://github.com/thlorenz/deep-is/blob/master/LICENSE]
		// * nil. Then it's probably in the package files.
		//
		// To find the licenses we are looking for we look for them as prefixes
		// which can have preceeding whitespace, or open/close parenthesis.
		r := regexp.MustCompile(fmt.Sprintf("(\\s+|\\(|\\))?%s", b))
		m := r.FindStringSubmatch(licenseType)
		if len(m) > 0 {
			return false, fmt.Sprintf("Package %s@%s contains a banned license prefix \"%s\"", packageName, packageVersion, b), nil
		}
	}

	sklog.Infof("Package %s@%s contains an allowed license preifx \"%s\"", packageName, packageVersion, licenseType)
	return true, "", nil
}
