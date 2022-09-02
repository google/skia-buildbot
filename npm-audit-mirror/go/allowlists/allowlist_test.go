package allowlists

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

var (
	startPackageName        = "start-pkg"
	startPackageExactVer    = "1.4.0"
	startPackageNonExactVer = "^1.4.0"

	dep1PackageName        = "dep1-pkg"
	dep1PackageExactVer    = "4.0"
	dep1PackageNonExactVer = "^4.0"

	dep1_1PackageName = "dep1_1-pkg"
	dep1_1PackageVer  = "^9.1"

	dep2PackageName        = "dep2-pkg"
	dep2PackageExactVer    = "5.0.1"
	dep2PackageNonExactVer = "<5.0.1"
)

func setupAllowlistHttpClient(t *testing.T, dep1PackageUseNonExactVer, dep2PackageUseNonExactVer bool) *http.Client {
	dep1PackageVer := dep1PackageExactVer
	if dep1PackageUseNonExactVer {
		dep1PackageVer = dep1PackageNonExactVer
	}
	dep2PackageVer := dep2PackageExactVer
	if dep2PackageUseNonExactVer {
		dep2PackageVer = dep2PackageNonExactVer
	}

	mockClient := mockhttpclient.NewURLMock()
	startPackageResp, err := json.Marshal(&types.NpmPackage{
		Versions: map[string]types.NpmVersion{
			startPackageExactVer: {
				Dependencies: map[string]string{
					dep1PackageName: dep1PackageVer,
					dep2PackageName: dep2PackageVer,
				},
			},
		},
	})
	dep1PackageResp, err := json.Marshal(&types.NpmPackage{
		Versions: map[string]types.NpmVersion{
			dep1PackageExactVer: {
				Dependencies: map[string]string{
					dep1_1PackageName: dep1_1PackageVer,
				},
			},
		},
	})
	dep1_1PackageResp, err := json.Marshal(&types.NpmPackage{
		Versions: map[string]types.NpmVersion{
			dep1_1PackageVer: {
				Dependencies: map[string]string{},
			},
		},
	})
	dep2PackageResp, err := json.Marshal(&types.NpmPackage{
		Versions: map[string]types.NpmVersion{
			dep2PackageExactVer: {
				Dependencies: map[string]string{},
			},
		},
	})
	require.Nil(t, err)

	mockClient.Mock("https://registry.npmjs.org/"+startPackageName, mockhttpclient.MockGetDialogue(startPackageResp))
	mockClient.Mock("https://registry.npmjs.org/"+dep1PackageName, mockhttpclient.MockGetDialogue(dep1PackageResp))
	mockClient.Mock("https://registry.npmjs.org/"+dep1_1PackageName, mockhttpclient.MockGetDialogue(dep1_1PackageResp))
	mockClient.Mock("https://registry.npmjs.org/"+dep2PackageName, mockhttpclient.MockGetDialogue(dep2PackageResp))

	return mockClient.Client()
}

func TestGetDependencies_StartPackageWithExactVerDeps_ReturnsAllDeps(t *testing.T) {
	mockHttpClient := setupAllowlistHttpClient(t, false, false)

	// All 3 dependencies should be returned.
	deps, err := getDependencies(startPackageName, startPackageExactVer, mockHttpClient)
	require.NoError(t, err)
	require.Len(t, deps, 3)
	require.Equal(t, dep1PackageName, deps[0].Name)
	require.Equal(t, dep1PackageExactVer, deps[0].Version)
	require.Equal(t, dep1_1PackageName, deps[1].Name)
	require.Equal(t, dep1_1PackageVer, deps[1].Version)
	require.Equal(t, dep2PackageName, deps[2].Name)
	require.Equal(t, dep2PackageExactVer, deps[2].Version)
}

func TestGetDependencies_StartPackageWithNonExactVerDeps_ReturnsNoDeps(t *testing.T) {
	mockHttpClient := setupAllowlistHttpClient(t, false, false)

	// 0 dependencies should be returned because we cannot find the dependencies
	// of a start package with a non-exact version.
	deps, err := getDependencies(startPackageName, startPackageNonExactVer, mockHttpClient)
	require.NoError(t, err)
	require.Len(t, deps, 0)
}

func TestGetDependencies_Dep2WithNonExactVer_ReturnsAllDeps(t *testing.T) {
	mockHttpClient := setupAllowlistHttpClient(t, false, true)

	// All 3 dependencies should be returned because dep2 has no dependencies.
	deps, err := getDependencies(startPackageName, startPackageExactVer, mockHttpClient)
	require.NoError(t, err)
	require.Len(t, deps, 3)
	require.Equal(t, dep1PackageName, deps[0].Name)
	require.Equal(t, dep1PackageExactVer, deps[0].Version)
	require.Equal(t, dep1_1PackageName, deps[1].Name)
	require.Equal(t, dep1_1PackageVer, deps[1].Version)
	require.Equal(t, dep2PackageName, deps[2].Name)
	require.Equal(t, dep2PackageNonExactVer, deps[2].Version)
}

func TestGetDependencies_Dep1WithNonExactVer_ReturnsTwoDeps(t *testing.T) {
	mockHttpClient := setupAllowlistHttpClient(t, true, false)

	// pkg1 uses non-exact versioning so we will not be able to find pkg1_1.
	deps, err := getDependencies(startPackageName, startPackageExactVer, mockHttpClient)
	require.NoError(t, err)
	require.Len(t, deps, 2)
	require.Equal(t, dep1PackageName, deps[0].Name)
	require.Equal(t, dep1PackageNonExactVer, deps[0].Version)
	require.Equal(t, dep2PackageName, deps[1].Name)
	require.Equal(t, dep2PackageExactVer, deps[1].Version)
}
