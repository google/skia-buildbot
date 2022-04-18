package checks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/types"
	"go.skia.org/infra/npm-audit-mirror/go/types/mocks"
)

func TestGetPackageDetails(t *testing.T) {
	unittest.SmallTest(t)

	tests := []struct {
		packageRequestURL     string
		expectedNameWithScope string
		expectedScopeName     string
		expectedTarballName   string
		expectedVersion       string
		name                  string
	}{
		{
			packageRequestURL:     "/import-fresh/-/import-fresh-3.3.0.tgz",
			expectedNameWithScope: "import-fresh",
			expectedScopeName:     "",
			expectedTarballName:   "import-fresh-3.3.0.tgz",
			expectedVersion:       "3.3.0",
			name:                  "GetPackageDetails test with /import-fresh/-/import-fresh-3.3.0.tgz",
		},
		{
			packageRequestURL:     "/@types/web/-/web-0.0.58.tgz",
			expectedNameWithScope: "@types/web",
			expectedScopeName:     "@types/",
			expectedTarballName:   "web-0.0.58.tgz",
			expectedVersion:       "0.0.58",
			name:                  "GetPackageDetails test with /@types/web/-/web-0.0.58.tgz",
		},
		{
			packageRequestURL:     "/@google-web-components/google-chart/-/google-chart-4.0.2.tgz",
			expectedNameWithScope: "@google-web-components/google-chart",
			expectedScopeName:     "@google-web-components/",
			expectedTarballName:   "google-chart-4.0.2.tgz",
			expectedVersion:       "4.0.2",
			name:                  "GetPackageDetails test with /@google-web-components/google-chart/-/google-chart-4.0.2.tgz",
		},
		{
			packageRequestURL:     "/gensync/-/gensync-1.0.0-beta.2.tgz",
			expectedNameWithScope: "gensync",
			expectedScopeName:     "",
			expectedTarballName:   "gensync-1.0.0-beta.2.tgz",
			expectedVersion:       "1.0.0-beta.2",
			name:                  "GetPackageDetails test with /gensync/-/gensync-1.0.0-beta.2.tgz",
		},
		{
			packageRequestURL:     "/@some-scope/gensync/-/gensync-1.0.0-beta.2.tgz",
			expectedNameWithScope: "@some-scope/gensync",
			expectedScopeName:     "@some-scope/",
			expectedTarballName:   "gensync-1.0.0-beta.2.tgz",
			expectedVersion:       "1.0.0-beta.2",
			name:                  "GetPackageDetails test with /@some-scope/gensync/-/gensync-1.0.0-beta.2.tgz",
		},
	}

	for _, test := range tests {
		details := getPackageDetails(test.packageRequestURL)
		require.Equal(t, test.expectedNameWithScope, details.NameWithScope, test.name)
		require.Equal(t, test.expectedScopeName, details.ScopeName, test.name)
		require.Equal(t, test.expectedTarballName, details.TarballName, test.name)
		require.Equal(t, test.expectedVersion, details.Version, test.name)
	}
}

func TestPerformChecks_NonTarballRequest_ReturnImmediately(t *testing.T) {
	unittest.SmallTest(t)

	cm := &NpmChecksManager{}
	checkPassed, result, err := cm.PerformChecks("gensync/-/gensync-1.0.0-beta")
	require.NoError(t, err)
	require.True(t, checkPassed)
	require.Empty(t, result)
}

func TestPerformChecks_PackageAlreadyDownloaded_ReturnAfterDownloadCheck(t *testing.T) {
	unittest.SmallTest(t)

	// Mock ProjectMirror.
	mockProjectMirror := &mocks.ProjectMirror{}
	defer mockProjectMirror.AssertExpectations(t)
	mockProjectMirror.On("IsPackageTarballDownloaded", "gensync-1.0.0-beta.tgz").Return(true).Once()
	mockProjectMirror.On("GetProjectName").Return("test-project").Once()

	cm := &NpmChecksManager{projectMirror: mockProjectMirror}
	checkPassed, result, err := cm.PerformChecks("/gensync/-/gensync-1.0.0-beta.tgz")
	require.NoError(t, err)
	require.True(t, checkPassed)
	require.Empty(t, result)
}

func TestPerformChecks_PackageWithTrustedScope_ReturnAfterTrustedScopeCheck(t *testing.T) {
	unittest.SmallTest(t)

	// Mock ProjectMirror.
	mockProjectMirror := &mocks.ProjectMirror{}
	defer mockProjectMirror.AssertExpectations(t)
	mockProjectMirror.On("IsPackageTarballDownloaded", "gensync-1.0.0-beta.tgz").Return(false).Once()

	cm := &NpmChecksManager{
		projectMirror: mockProjectMirror,
		trustedScopes: []string{"@some-scope/"},
	}
	checkPassed, result, err := cm.PerformChecks("/@some-scope/gensync/-/gensync-1.0.0-beta.tgz")
	require.NoError(t, err)
	require.True(t, checkPassed)
	require.Empty(t, result)
}

func TestPerformChecks_PackageInAllowlist_ReturnAfterAllowlistCheck(t *testing.T) {
	unittest.SmallTest(t)

	// Mock ProjectMirror.
	mockProjectMirror := &mocks.ProjectMirror{}
	defer mockProjectMirror.AssertExpectations(t)
	mockProjectMirror.On("IsPackageTarballDownloaded", "gensync-1.0.0-beta.tgz").Return(false).Once()

	// Add allowed package with full package version specified.
	allowedPackages := []*config.PackagesAllowList{
		{Name: "gensync", Version: "1.0.0-beta"},
	}
	cm := &NpmChecksManager{
		projectMirror:   mockProjectMirror,
		trustedScopes:   []string{},
		allowedPackages: allowedPackages,
	}
	checkPassed, result, err := cm.PerformChecks("/gensync/-/gensync-1.0.0-beta.tgz")
	require.NoError(t, err)
	require.True(t, checkPassed)
	require.Empty(t, result)

	// Add allowedPackage with semver.
	allowedPackages = []*config.PackagesAllowList{
		{Name: "gensync", Version: "^1.0"},
	}
	mockProjectMirror.On("IsPackageTarballDownloaded", "gensync-1.1.tgz").Return(false).Once()
	cm.allowedPackages = allowedPackages
	checkPassed, result, err = cm.PerformChecks("/gensync/-/gensync-1.1.tgz")
	require.NoError(t, err)
	require.True(t, checkPassed)
	require.Empty(t, result)
}

func TestPerformChecks_RunSecurityChecks(t *testing.T) {
	unittest.SmallTest(t)

	// Mock ProjectMirror.
	mockProjectMirror := &mocks.ProjectMirror{}
	defer mockProjectMirror.AssertExpectations(t)
	mockProjectMirror.On("IsPackageTarballDownloaded", "gensync-1.0.0-beta.tgz").Return(false)
	mockProjectMirror.On("AddToDownloadedPackageTarballs", "gensync-1.0.0-beta.tgz").Return()
	mockProjectMirror.On("GetProjectName").Return("test-project")

	// Mock HTTP client.
	testPackageResp, err := json.Marshal(&types.NpmPackage{
		Versions: map[string]types.NpmVersion{
			"1.0.0": {},
		},
	})
	mockHttpClient := mockhttpclient.NewURLMock()
	mockHttpClient.Mock("https://registry.npmjs.org/gensync", mockhttpclient.MockGetDialogue(testPackageResp))

	// Run with no security checks specified should pass.
	cm := &NpmChecksManager{
		projectMirror:   mockProjectMirror,
		trustedScopes:   []string{},
		allowedPackages: []*config.PackagesAllowList{},
		httpClient:      mockHttpClient.Client(),
		checks:          []types.Check{},
	}
	checkPassed, result, err := cm.PerformChecks("/gensync/-/gensync-1.0.0-beta.tgz")
	require.NoError(t, err)
	require.True(t, checkPassed)
	require.Empty(t, result)

	// Add a mock security check that passes.
	mockCheck := &mocks.Check{}
	mockCheck.On("PerformCheck", "gensync", "1.0.0-beta", mock.Anything).Return(true, "", nil).Once()
	mockCheck.On("Name").Return("Mock-Check")
	defer mockCheck.AssertExpectations(t)

	cm.checks = []types.Check{mockCheck}
	checkPassed, result, err = cm.PerformChecks("/gensync/-/gensync-1.0.0-beta.tgz")
	require.NoError(t, err)
	require.True(t, checkPassed)
	require.Empty(t, result)

	// Add a security check that fails.
	mockCheck.On("PerformCheck", "gensync", "1.0.0-beta", mock.Anything).Return(false, "some reason", nil).Once()
	checkPassed, result, err = cm.PerformChecks("/gensync/-/gensync-1.0.0-beta.tgz")
	require.NoError(t, err)
	require.False(t, checkPassed)
	require.Equal(t, "some reason", result)
}
