package version_file_common

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
)

func TestGetPinnedRev(t *testing.T) {

	type tc struct {
		name                string
		depId               string
		versionFilePath     string
		regex               string
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
		{
			name:            "Regex",
			depId:           "my-dep",
			versionFilePath: "regex-file",
			regex:           `"my-dep@([a-zA-Z_-]+)"`,
			versionFileContents: `deps = {
				"my-dep-path": "my-dep@my-rev",
			}`,
			expectRev: "my-rev",
		},
		{
			// Verify that the regex takes precedence over DEPS parsing.
			name:            "Regex in DEPS",
			depId:           "my-dep",
			versionFilePath: deps_parser.DepsFileName,
			regex:           `'some-var': '([a-zA-Z_-]+)',`,
			versionFileContents: `
			vars = {
				'some-var': 'my-other-rev',
			}
			deps = {
				"my-dep-path": "my-dep@my-rev",
			}`,
			expectRev: "my-other-rev",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			actual, err := GetPinnedRev(&config.VersionFileConfig{
				Id:    c.depId,
				Path:  c.versionFilePath,
				Regex: c.regex,
			}, c.versionFileContents)
			require.NoError(t, err)
			require.Equal(t, c.expectRev, actual)
		})
	}
}

func TestSetPinnedRev(t *testing.T) {

	type tc struct {
		name                string
		depId               string
		versionFilePath     string
		versionFileContents string
		newRev              *revision.Revision
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
			newRev: &revision.Revision{
				Id: "new-rev",
			},
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
			newRev: &revision.Revision{
				Id: "new-rev",
			},
			expectNewContents: `new-rev
`,
		},
		{
			name:            "BazelFile",
			depId:           "infra/3pp/tools/git/linux-amd64",
			versionFilePath: "WORKSPACE",
			versionFileContents: `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    tag = "version:2.29.2.chromium.6",
)
`,
			newRev: &revision.Revision{
				Id:       "version:2.30.1.chromium.7",
				Checksum: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
			expectNewContents: `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    tag = "version:2.30.1.chromium.7",
)
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

	type tc struct {
		name                string
		depId               string
		versionFilePath     string
		versionFileContents string
		newRev              *revision.Revision
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
			newRev: &revision.Revision{
				Id: "new-rev",
			},
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
			newRev: &revision.Revision{
				Id: "new-rev",
			},
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

func TestUpdateSingleDepSubmodule(t *testing.T) {
	oldDepsContent := `
git_dependencies = "SYNC"
deps = {
	"foo": "with-submodule@old-rev1",
	"bar": "without-submodule@old-rev2",
}`

	getFile := func(ctx context.Context, path string) (string, error) {
		if path == deps_parser.DepsFileName {
			return oldDepsContent, nil
		}
		if path == "foo" {
			return "old-rev1", nil
		}
		return "", fmt.Errorf("Unknown file path %s", path)
	}
	t.Run("change with submodule", func(t *testing.T) {
		changes := map[string]string{}
		newDepsContent := `
git_dependencies = "SYNC"
deps = {
	"foo": "with-submodule@new-rev1",
	"bar": "without-submodule@old-rev2",
}`
		oldRev, err := updateSingleDep(context.Background(), &config.VersionFileConfig{
			Id:   "with-submodule",
			Path: deps_parser.DepsFileName,
		}, &revision.Revision{Id: "new-rev1"}, changes, getFile)
		require.NoError(t, err)
		require.Equal(t, "old-rev1", oldRev)
		actualNewContents := changes[deps_parser.DepsFileName]
		require.Equal(t, newDepsContent, actualNewContents)

		require.Equal(t, "new-rev1", changes["foo"])
	})
	t.Run("change without submodule", func(t *testing.T) {
		changes := map[string]string{}
		newDepsContent := `
git_dependencies = "SYNC"
deps = {
	"foo": "with-submodule@old-rev1",
	"bar": "without-submodule@new-rev2",
}`
		oldRev, err := updateSingleDep(context.Background(), &config.VersionFileConfig{
			Id:   "without-submodule",
			Path: deps_parser.DepsFileName,
		}, &revision.Revision{Id: "new-rev2"}, changes, getFile)
		require.NoError(t, err)
		require.Equal(t, "old-rev2", oldRev)
		actualNewContents := changes[deps_parser.DepsFileName]
		require.Equal(t, newDepsContent, actualNewContents)

		require.Equal(t, "", changes["bar"])
	})
}

func TestUpdateDep(t *testing.T) {

	oldContents := map[string]string{
		deps_parser.DepsFileName: `deps = {
			"my-dep-path": "my-dep@old-rev",
			"transitive-dep-path": "transitive-dep@transitive-dep-old-rev",
		}`,
		"find/and/replace/file": `Unrelated stuff
Version: old-rev;
Transitive-dep-version: transitive-dep-old-rev;
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
Transitive-dep-version: transitive-dep-new-rev;
`,
	}, changes)
}

func TestUpdateDep_UsesChangeCache(t *testing.T) {
	// This configuration updates DEPS twice: once for the primary dependency
	// and again using find-and-replace to update a comment. Verify that we only
	// read the file from the repo once (since reading it a second time would
	// undo the first update).
	oldContents := map[string]string{
		deps_parser.DepsFileName: `deps = {
			# Use my-dep at commit old-rev.
			"my-dep-path": "my-dep@old-rev",
		}`,
	}
	alreadyRead := false
	getFile := func(ctx context.Context, path string) (string, error) {
		require.False(t, alreadyRead, "read %s multiple times instead of using version cached in changes map", path)
		contents, ok := oldContents[path]
		if !ok {
			return "", fmt.Errorf("Unknown path %s", path)
		}
		alreadyRead = true
		return contents, nil
	}

	changes, err := UpdateDep(context.Background(), &config.DependencyConfig{
		Primary: &config.VersionFileConfig{
			Id:   "my-dep",
			Path: deps_parser.DepsFileName,
		},
		FindAndReplace: []string{
			deps_parser.DepsFileName,
		},
	}, &revision.Revision{
		Id: "new-rev",
	}, getFile)
	require.NoError(t, err)

	require.Equal(t, map[string]string{
		deps_parser.DepsFileName: `deps = {
			# Use my-dep at commit new-rev.
			"my-dep-path": "my-dep@new-rev",
		}`,
	}, changes)
}
