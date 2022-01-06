package version_file_common

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetPinnedRev(t *testing.T) {
	unittest.SmallTest(t)

	type tc struct {
		name                string
		depId               string
		versionFilePath     string
		versionFileContents string
		expectRev           string
	}
	for _, c := range []tc{
		{
			name:            "DEPSFile",
			depId:           "my-dep",
			versionFilePath: deps_parser.DepsFileName,
			versionFileContents: `deps = {
				"my-dep-path": "my-dep@my-rev",
			}`,
			expectRev: "my-rev",
		},
		{
			name:            "PlainVersionFile",
			depId:           "my-dep",
			versionFilePath: "any/other/file",
			versionFileContents: `  my-rev

			`,
			expectRev: "my-rev",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			actual, err := GetPinnedRev(&config.VersionFileConfig{
				Id:   c.depId,
				Path: c.versionFilePath,
			}, c.versionFileContents)
			require.NoError(t, err)
			require.Equal(t, c.expectRev, actual)
		})
	}
}

func TestSetPinnedRev(t *testing.T) {
	unittest.SmallTest(t)

	type tc struct {
		name                string
		depId               string
		versionFilePath     string
		versionFileContents string
		newRev              string
		expectNewContents   string
	}
	for _, c := range []tc{
		{
			name:            "DEPSFile",
			depId:           "my-dep",
			versionFilePath: deps_parser.DepsFileName,
			versionFileContents: `deps = {
				"my-dep-path": "my-dep@old-rev",
			}`,
			newRev: "new-rev",
			expectNewContents: `deps = {
				"my-dep-path": "my-dep@new-rev",
			}`,
		},
		{
			name:            "PlainVersionFile",
			depId:           "my-dep",
			versionFilePath: "any/other/file",
			versionFileContents: `  old-rev

			`,
			newRev: "new-rev",
			expectNewContents: `new-rev
`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			actual, err := SetPinnedRev(&config.VersionFileConfig{
				Id:   c.depId,
				Path: c.versionFilePath,
			}, c.newRev, c.versionFileContents)
			require.NoError(t, err)
			require.Equal(t, c.expectNewContents, actual)
		})
	}
}

func TestUpdateSingleDep(t *testing.T) {
	unittest.SmallTest(t)

	type tc struct {
		name                string
		depId               string
		versionFilePath     string
		versionFileContents string
		newRev              string
		expectOldRev        string
		expectNewContents   string
	}
	for _, c := range []tc{
		{
			name:            "DEPSFile",
			depId:           "my-dep",
			versionFilePath: deps_parser.DepsFileName,
			versionFileContents: `deps = {
				"my-dep-path": "my-dep@old-rev",
			}`,
			newRev:       "new-rev",
			expectOldRev: "old-rev",
			expectNewContents: `deps = {
				"my-dep-path": "my-dep@new-rev",
			}`,
		},
		{
			name:            "PlainVersionFile",
			depId:           "my-dep",
			versionFilePath: "some/other/file",
			versionFileContents: `old-rev
`,
			newRev:       "new-rev",
			expectOldRev: "old-rev",
			expectNewContents: `new-rev
`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			changes := map[string]string{}
			getFile := func(ctx context.Context, path string) (string, error) {
				if path == c.versionFilePath {
					return c.versionFileContents, nil
				}
				return "", fmt.Errorf("Unknown file path %s", path)
			}
			oldRev, err := updateSingleDep(context.Background(), &config.VersionFileConfig{
				Id:   c.depId,
				Path: c.versionFilePath,
			}, c.newRev, changes, getFile)
			require.NoError(t, err)
			require.Equal(t, c.expectOldRev, oldRev)
			actualNewContents := changes[c.versionFilePath]
			require.Equal(t, c.expectNewContents, actualNewContents)
		})
	}
}

func TestUpdateDep(t *testing.T) {
	unittest.SmallTest(t)

	oldContents := map[string]string{
		deps_parser.DepsFileName: `deps = {
			"my-dep-path": "my-dep@old-rev",
			"transitive-dep-path": "transitive-dep@transitive-dep-old-rev",
		}`,
		"find/and/replace/file": `Unrelated stuff
Version: old-rev;
Unrelated stuff
`,
	}
	getFile := func(ctx context.Context, path string) (string, error) {
		contents, ok := oldContents[path]
		if !ok {
			return "", fmt.Errorf("Unknown path %s", path)
		}
		return contents, nil
	}

	changes, err := UpdateDep(context.Background(), &config.DependencyConfig{
		Primary: &config.VersionFileConfig{
			Id:   "my-dep",
			Path: deps_parser.DepsFileName,
		},
		Transitive: []*config.TransitiveDepConfig{
			{
				Child: &config.VersionFileConfig{
					Id:   "transitive-dep",
					Path: deps_parser.DepsFileName,
				},
				Parent: &config.VersionFileConfig{
					Id:   "transitive-dep",
					Path: deps_parser.DepsFileName,
				},
			},
		},
		FindAndReplace: []string{
			"find/and/replace/file",
		},
	}, &revision.Revision{
		Id: "new-rev",
		Dependencies: map[string]string{
			"transitive-dep": "transitive-dep-new-rev",
		},
	}, getFile)
	require.NoError(t, err)

	require.Equal(t, map[string]string{
		deps_parser.DepsFileName: `deps = {
			"my-dep-path": "my-dep@new-rev",
			"transitive-dep-path": "transitive-dep@transitive-dep-new-rev",
		}`,
		"find/and/replace/file": `Unrelated stuff
Version: new-rev;
Unrelated stuff
`,
	}, changes)
}
