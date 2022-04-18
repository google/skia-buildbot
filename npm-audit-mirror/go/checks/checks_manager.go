package checks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

var (
	// Examples of what the package tarball requests can look like:
	// * /import-fresh/-/import-fresh-3.3.0.tgz
	// * /@types/web/-/web-0.0.58.tgz
	// * /@google-web-components/google-chart/-/google-chart-4.0.2.tgz
	// * /gensync/-/gensync-1.0.0-beta.2.tgz
	packageTarballRequestRegex = regexp.MustCompile("/((@.+/)?(.+))/-/((.+).tgz)")
)

// NpmChecksManager implements types.ChecksManager.
type NpmChecksManager struct {
	trustedScopes   []string
	allowedPackages []*config.PackagesAllowList
	httpClient      *http.Client // probably not needed.
	projectMirror   types.ProjectMirror

	checks []types.Check
}

// NewNpmChecksManager returns an instance of NpmChecksManager.
func NewNpmChecksManager(trustedScopes []string, allowedPackages []*config.PackagesAllowList, httpClient *http.Client, projectMirror types.ProjectMirror) types.ChecksManager {
	// The checks manager will perform the following checks for packages.
	checks := []types.Check{NewPublishAgeCheck(), NewLicenceCheck()}

	return &NpmChecksManager{
		trustedScopes:   trustedScopes,
		allowedPackages: allowedPackages,
		httpClient:      httpClient,
		projectMirror:   projectMirror,
		checks:          checks,
	}
}

// getPackageDetails parses the provided requestURL and returns a populated
// PackageDetails obj.
func getPackageDetails(packageRequestURL string) *types.PackageDetails {
	// Replace all "%2f" with "/" in the request URL.
	packageRequestURL = strings.ReplaceAll(packageRequestURL, "%2f", "/")
	match := packageTarballRequestRegex.FindStringSubmatch(packageRequestURL)
	return &types.PackageDetails{
		NameWithScope: match[1],
		ScopeName:     match[2],
		TarballName:   match[4],
		Version:       strings.TrimPrefix(match[5], match[3]+"-"),
	}
}

// PerformChecks implements the types.ChecksManager interface.
func (ncm *NpmChecksManager) PerformChecks(packageRequestURL string) (bool, string, error) {
	// We only perform checks on requests for package tarballs. Because this is when
	// the package is downloaded to the developer/CI machine.
	if !strings.HasSuffix(packageRequestURL, ".tgz") {
		return true, "", nil
	}

	// Get package details.
	packageDetails := getPackageDetails(packageRequestURL)

	// Check to see if the packageTarball is in this project's installed packages.
	if ncm.projectMirror.IsPackageTarballDownloaded(packageDetails.TarballName) {
		// No need to perform checks on already downloaded packages.
		sklog.Infof("Package %s is already downloaded to project %s mirror. Skipping security checks.", packageDetails.TarballName, ncm.projectMirror.GetProjectName())
		return true, "", nil
	}

	// Check for trusted scopes.
	for _, trustedScope := range ncm.trustedScopes {
		if packageDetails.ScopeName == trustedScope {
			sklog.Infof("The package %s has the trusted scope %s. Skipping security checks.", packageDetails.NameWithScope, trustedScope)
			return true, "", nil
		}
	}

	// Check for packages in the allowlist.
	packageSemVer, err := semver.NewVersion(packageDetails.Version)
	if err != nil {
		sklog.Errorf("Could not parse semver in %s: %s", packageDetails.Version, err)
		return false, "", err
	}
	for _, ap := range ncm.allowedPackages {
		if ap.Name == packageDetails.NameWithScope {
			allowedSemVerConstaint, err := semver.NewConstraint(ap.Version)
			if err != nil {
				sklog.Errorf("Could not parse version of package %s@%s in %s: %s", ap.Name, ap.Version, err)
				return false, "", err
			}
			if allowedSemVerConstaint.Check(packageSemVer) {
				sklog.Infof("The package %s with version %s matched the semantic versioning of allowed package %s@%s. Skipping security checks.", packageDetails.NameWithScope, packageDetails.Version, ap.Name, ap.Version)
				return true, "", nil
			}
		}
	}

	// Call registry.npmjs.org to run checks on the package before allowing the
	// mirror to download it.
	viewNpmURL := fmt.Sprintf("https://registry.npmjs.org/%s", packageDetails.NameWithScope)
	r, err := ncm.httpClient.Get(viewNpmURL)
	if err != nil {
		return false, "", skerr.Wrapf(err, "Error getting response from %s", viewNpmURL)
	}
	defer r.Body.Close()
	var npmPackage types.NpmPackage
	if err := json.NewDecoder(r.Body).Decode(&npmPackage); err != nil {
		return false, "", skerr.Wrapf(err, "Failed to decode response from %s", viewNpmURL)
	}

	// Start the security checks.
	for _, check := range ncm.checks {
		result, resultReason, err := check.PerformCheck(packageDetails.NameWithScope, packageDetails.Version, &npmPackage)
		if err != nil {
			sklog.Errorf("Error performing check %s on package %s with version %s", check.Name(), packageDetails.NameWithScope, packageDetails.Version)
			return false, "", err
		}
		if result {
			sklog.Infof("The check %s succeeded for package %s with version %s", check.Name(), packageDetails.NameWithScope, packageDetails.Version)
		} else {
			sklog.Warningf("The check %s failed for package %s with version %s due to: %s", check.Name(), packageDetails.NameWithScope, packageDetails.Version, resultReason)
			return false, resultReason, nil
		}
	}

	// If we reach here then all security passes succeeded. Add the package to the mirror.
	sklog.Infof("Add the package %s to the in-memory map of downloaded tarballs for project %s.", packageDetails.TarballName, ncm.projectMirror.GetProjectName())
	ncm.projectMirror.AddToDownloadedPackageTarballs(packageDetails.TarballName)

	return true, "", nil
}
