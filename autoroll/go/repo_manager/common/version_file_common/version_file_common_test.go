package version_file_common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/sklog"
)

func TestGetPinnedRev(t *testing.T) {
	test := func(name string, dep *config.VersionFileConfig, versionFileContents, expectRev string) {
		t.Run(name, func(t *testing.T) {
			actual, err := getPinnedRevInFile(dep.Id, dep.File[0], versionFileContents)
			require.NoError(t, err)
			require.Equal(t, expectRev, actual)
		})
	}

	test("DEPSFile",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path: deps_parser.DepsFileName,
				},
			},
		},
		`deps = {
				"my-dep-path": "my-dep@my-rev",
			}`,
		"my-rev",
	)
	test("README.chromium",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path: "path/to/README.chromium",
				},
			},
		},
		`Name: Abseil
Short Name: absl
URL: https://github.com/abseil/abseil-cpp
License: Apache-2.0
License File: LICENSE
Version: N/A
Revision: 0437a6d16a02455a07bb59da6f08ef01c6a20682
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`,
		"0437a6d16a02455a07bb59da6f08ef01c6a20682",
	)
	test("PlainVersionFile",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path: "any/other/file",
				},
			},
		},
		`  my-rev

			`,
		"my-rev",
	)
	test("Regex",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path:  "regex-file",
					Regex: `"my-dep@([a-zA-Z_-]+)"`,
				},
			},
		},
		`deps = {
				"my-dep-path": "my-dep@my-rev",
			}`,
		"my-rev",
	)
	// Verify that the regex takes precedence over DEPS parsing.
	test(
		"Regex in DEPS",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path:  deps_parser.DepsFileName,
					Regex: `'some-var': '([a-zA-Z_-]+)',`,
				},
			},
		},
		`
			vars = {
				'some-var': 'my-other-rev',
			}
			deps = {
				"my-dep-path": "my-dep@my-rev",
			}`,
		"my-other-rev",
	)
	// Verify that we use the first match when multiple regex matches exist.
	test(
		"Multiple Regex Matches Take First",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path:  "some-file",
					Regex: `{"my-dep":"([a-z0-9]+)"},`,
				},
			},
		},
		`{
  "deps": [
    {"my-dep":"abc123"},
    {"my-dep":"def456"}, // Another copy, for reasons!
  ]
}`,
		"abc123",
	)
}

func TestSetPinnedRev(t *testing.T) {
	test := func(name string, dep *config.VersionFileConfig, versionFileContents string, newRev *revision.Revision, expectNewContents string) {
		t.Run(name, func(t *testing.T) {
			actual, err := setPinnedRevInFile(dep.Id, dep.File[0], newRev, versionFileContents)
			require.NoError(t, err)
			require.Equal(t, expectNewContents, actual)
		})
	}

	test("DEPSFile",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path: deps_parser.DepsFileName,
				},
			},
		},
		`deps = {
				"my-dep-path": "my-dep@old-rev",
			}`,
		&revision.Revision{
			Id: "new-rev",
		},
		`deps = {
				"my-dep-path": "my-dep@new-rev",
			}`,
	)
	test("README.chromium",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path: "path/to/README.chromium",
				},
			},
		},
		`Name: Abseil
Short Name: absl
URL: https://github.com/abseil/abseil-cpp
License: Apache-2.0
License File: LICENSE
Version: N/A
Revision: 0437a6d16a02455a07bb59da6f08ef01c6a20682
Date: 2025-09-23
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`,
		&revision.Revision{
			Id:        "new-rev",
			Release:   "v1.2.3",
			Timestamp: time.Date(2026, time.January, 25, 0, 0, 0, 0, time.UTC),
		},
		`Name: Abseil
Short Name: absl
URL: https://github.com/abseil/abseil-cpp
License: Apache-2.0
License File: LICENSE
Version: v1.2.3
Revision: new-rev
Date: 2026-01-25
Update Mechanism: Autoroll
Security Critical: yes
Shipped: yes
`,
	)

	test("README.chromium_NoRelease",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path: "path/to/README.chromium",
				},
			},
		},
		`Name: Abseil
Short Name: absl
URL: https://github.com/abseil/abseil-cpp
License: Apache-2.0
License File: LICENSE
Version: N/A
Revision: 0437a6d16a02455a07bb59da6f08ef01c6a20682
Date: 2025-09-23
Update Mechanism: Manual
Security Critical: yes
Shipped: yes
`,
		&revision.Revision{
			Id:        "new-rev",
			Release:   "",
			Timestamp: time.Date(2026, time.January, 25, 0, 0, 0, 0, time.UTC),
		},
		`Name: Abseil
Short Name: absl
URL: https://github.com/abseil/abseil-cpp
License: Apache-2.0
License File: LICENSE
Version: N/A
Revision: new-rev
Date: 2026-01-25
Update Mechanism: Autoroll
Security Critical: yes
Shipped: yes
`,
	)

	test("PlainVersionFile",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path: "any/other/file",
				},
			},
		},
		`  old-rev

			`,
		&revision.Revision{
			Id: "new-rev",
		},
		`new-rev
`,
	)
	test("BazelFile",
		&config.VersionFileConfig{
			Id: "infra/3pp/tools/git/linux-amd64",
			File: []*config.VersionFileConfig_File{
				{
					Path: "WORKSPACE",
				},
			},
		},
		`
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    tag = "version:2.29.2.chromium.6",
)
`,
		&revision.Revision{
			Id:       "version:2.30.1.chromium.7",
			Checksum: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		`
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    tag = "version:2.30.1.chromium.7",
)
`,
	)
	// Verify that we replace the first match when multiple regex matches exist.
	test(
		"Multiple Regex Matches Replace First",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path:  "some-file",
					Regex: `{"my-dep":"([a-z0-9]+)"},`,
				},
			},
		},
		`{
  "deps": [
    {"my-dep":"abc123"},
    {"my-dep":"def456"}, // Another copy, for reasons!
  ]
}`,
		&revision.Revision{
			Id: "999999",
		},
		`{
  "deps": [
    {"my-dep":"999999"},
    {"my-dep":"def456"}, // Another copy, for reasons!
  ]
}`,
	)
	// Verify that we replace all matches when RegexReplaceAll is true.
	test(
		"Multiple Regex Matches Replace All",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path:            "some-file",
					Regex:           `{"my-dep":"([a-z0-9]+)"},`,
					RegexReplaceAll: true,
				},
			},
		},
		`{
  "deps": [
    {"my-dep":"abc123"},
    {"my-dep":"def456"}, // Another copy, for reasons!
  ]
}`,
		&revision.Revision{
			Id: "999999",
		},
		`{
  "deps": [
    {"my-dep":"999999"},
    {"my-dep":"999999"}, // Another copy, for reasons!
  ]
}`,
	)
}

func TestUpdateSingleDep(t *testing.T) {
	test := func(name string, dep *config.VersionFileConfig, versionFileContents string, newRev *revision.Revision, expectOldRev string, expectNewContents string) {
		t.Run(name, func(t *testing.T) {
			changes := map[string]string{}
			getFile := func(ctx context.Context, path string) (string, error) {
				if path == dep.File[0].Path {
					return versionFileContents, nil
				}
				return "", fmt.Errorf("Unknown file path %s", path)
			}
			oldRev, err := updateSingleDep(context.Background(), dep, newRev, changes, getFile)
			require.NoError(t, err)
			require.Equal(t, expectOldRev, oldRev)
			actualNewContents := changes[dep.File[0].Path]
			require.Equal(t, expectNewContents, actualNewContents)
		})
	}

	test("DEPSFile",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path: deps_parser.DepsFileName,
				},
			},
		},
		`deps = {
				"my-dep-path": "my-dep@old-rev",
			}`,
		&revision.Revision{
			Id: "new-rev",
		},
		"old-rev",
		`deps = {
				"my-dep-path": "my-dep@new-rev",
			}`,
	)
	test("PlainVersionFile",
		&config.VersionFileConfig{
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path: "some/other/file",
				},
			},
		},
		`old-rev
`,
		&revision.Revision{
			Id: "new-rev",
		},
		"old-rev",
		`new-rev
`,
	)
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
			Id: "with-submodule",
			File: []*config.VersionFileConfig_File{
				{Path: deps_parser.DepsFileName},
			},
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
			Id: "without-submodule",
			File: []*config.VersionFileConfig_File{
				{Path: deps_parser.DepsFileName},
			},
		}, &revision.Revision{Id: "new-rev2"}, changes, getFile)
		require.NoError(t, err)
		require.Equal(t, "old-rev2", oldRev)
		actualNewContents := changes[deps_parser.DepsFileName]
		require.Equal(t, newDepsContent, actualNewContents)

		require.Equal(t, "", changes["bar"])
	})
}

func TestUpdateSingleDep_MultipleFiles(t *testing.T) {
	depsContents := `
deps = {
  "my-dep-path": "my-dep@old-rev",
}`

	entries, err := deps_parser.ParseDeps(depsContents)
	require.NoError(t, err)
	sklog.Infof("%+v", entries)
	entry, err := deps_parser.GetDep(depsContents, "my-dep")
	require.NoError(t, err)
	sklog.Infof("%+v", entry)

	secondaryPath := "other-dep-path"
	secondaryPathContents := "old-rev"
	changes := map[string]string{}
	getFile := func(ctx context.Context, path string) (string, error) {
		if path == deps_parser.DepsFileName {
			return depsContents, nil
		} else if path == secondaryPath {
			return secondaryPathContents, nil
		}
		return "", fmt.Errorf("Unknown file path %s", path)
	}
	oldRev, err := updateSingleDep(context.Background(), &config.VersionFileConfig{
		Id: "my-dep",
		File: []*config.VersionFileConfig_File{
			{Path: deps_parser.DepsFileName},
			{Path: secondaryPath},
		},
	}, &revision.Revision{Id: "new-rev"}, changes, getFile)
	require.NoError(t, err)
	require.Equal(t, "old-rev", oldRev)

	require.Equal(t, `
deps = {
  "my-dep-path": "my-dep@new-rev",
}`, changes[deps_parser.DepsFileName])
	require.Equal(t, "new-rev\n", changes[secondaryPath])
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
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{Path: deps_parser.DepsFileName},
			},
		},
		Transitive: []*config.TransitiveDepConfig{
			{
				Child: &config.VersionFileConfig{
					Id: "transitive-dep",
					File: []*config.VersionFileConfig_File{
						{Path: deps_parser.DepsFileName},
					},
				},
				Parent: &config.VersionFileConfig{
					Id: "transitive-dep",
					File: []*config.VersionFileConfig_File{
						{Path: deps_parser.DepsFileName},
					},
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
			Id: "my-dep",
			File: []*config.VersionFileConfig_File{
				{
					Path: deps_parser.DepsFileName,
				},
			},
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
