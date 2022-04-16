// allowlist package contains utilities for dealing with allowlists in the config file.
package allowlists

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

// GetAllowlistWithDeps returns a slice of packages that contains all packages
// explicitly specified in the config file and all of their dependencies.
// Note: This can only find dependencies of packages that use exact versions,
// else we will not know which version to use when querying the global NPM
// registry for dependencies. In other words, packages that use "~,^,-,*" in
// their versions will not have their dependencies added.
func GetAllowlistWithDeps(allowlist []config.PackagesAllowList, httpClient *http.Client) ([]*config.PackagesAllowList, error) {
	// New packages found will be added to this slice.
	allowListWithDeps := []*config.PackagesAllowList{}
	for _, allowedPackage := range allowlist {
		// Add the package to the allowlist that will be returned.
		allowListWithDeps = append(allowListWithDeps, &config.PackagesAllowList{Name: allowedPackage.Name, Version: allowedPackage.Version})

		// Now add all dependencies with exact versioning.
		deps, err := getDependencies(allowedPackage.Name, allowedPackage.Version, httpClient)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		allowListWithDeps = append(allowListWithDeps, deps...)
	}
	return allowListWithDeps, nil
}

// getDependencies recursively gets all dependencies of the specified start packages.
// Note: This can only find dependencies of packages that use exact versions,
// else we will not know which version to use when querying the global NPM
// registry for dependencies. In other words, packages that use "~,^,-,*" in
// their versions will not have their dependencies added.
func getDependencies(startPackageName, startPackageVersion string, httpClient *http.Client) ([]*config.PackagesAllowList, error) {
	allowListWithDeps := []*config.PackagesAllowList{}

	// Call registry.npmjs.org to get the package's dependencies.
	viewNpmURL := fmt.Sprintf("https://registry.npmjs.org/%s", startPackageName)
	r, err := httpClient.Get(viewNpmURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "Error getting response from %s", viewNpmURL)
	}
	defer r.Body.Close()

	var npmPackage types.NpmPackage
	if err := json.NewDecoder(r.Body).Decode(&npmPackage); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode response from %s", viewNpmURL)
	}

	if versionDetails, ok := npmPackage.Versions[startPackageVersion]; ok {
		for depName, depVersion := range versionDetails.Dependencies {
			allowListWithDeps = append(allowListWithDeps, &config.PackagesAllowList{Name: depName, Version: depVersion})

			// Recursive call to get all transitive deps.
			deps, err := getDependencies(depName, depVersion, httpClient)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			allowListWithDeps = append(allowListWithDeps, deps...)
		}
	} else {
		// The specified package was not found. Nothing to do.
		// Note: The specified package might not be found if the package
		// version used wildcards like "~,^,-,*"..
	}
	return allowListWithDeps, nil
}
