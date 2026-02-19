package version_file_common

import (
	"context"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/sklog"
)

func TestGetPinnedRev(t *testing.T) {
	test := func(name string, dep *config.VersionFileConfig, versionFileContents, expectRev string, expectMeta map[string]string) {
		t.Run(name, func(t *testing.T) {
			actual, actualMeta, err := getPinnedRevInFile(dep.Id, dep.File[0], versionFileContents)
			require.NoError(t, err)
			require.Equal(t, expectRev, actual)
			require.EqualExportedValues(t, expectMeta, actualMeta)
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
		nil,
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
		nil,
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
		nil,
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
		nil,
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
		nil,
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
		nil,
	)
	test(
		"MODULE.bazel single",
		&config.VersionFileConfig{
			Id: "skia/tools/goldctl/linux-amd64",
			File: []*config.VersionFileConfig_File{
				{
					Path: "MODULE.bazel",
				},
			},
		},
		`
cipd.download_http(
    name = "goldctl",
    cipd_package = "skia/tools/goldctl/linux-amd64",
    sha256 = "Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
    tag = "git_revision:808a00437f24bb404c09608ad8bf3847a78de369",
)
use_repo(cipd, "goldctl_linux-amd64")
`,
		"git_revision:808a00437f24bb404c09608ad8bf3847a78de369",
		nil,
	)

	test(
		"MODULE.bazel meta",
		&config.VersionFileConfig{
			Id: "skia/tools/goldctl/${platform}",
			File: []*config.VersionFileConfig_File{
				{
					Path: "MODULE.bazel",
				},
			},
		},
		`
cipd.download_http(
    name = "goldctl",
    cipd_package = "skia/tools/goldctl/${platform}",
    platform_to_sha256 = {
        "linux-amd64":   "Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
        "linux-arm64":   "XhM6rCCaRV3DJ4DkjsSTEahG-w3Er_ulss6w9-GwgkwC",
        "mac-amd64":     "1az5xiBG-ds55R4yd7fCkODv0xrApC5gC6iLb2SCig8C",
        "mac-arm64":     "bfOh3y10stM2Fj7HG-dDsJpJfm-J8yELSuoY94ec9UQC",
        "windows-amd64": "MeJ2G6pEJ4Vz3CvzoEf1QhrbEZqSzSh2uujaq7KwJtYC",
    },
    tag = "git_revision:808a00437f24bb404c09608ad8bf3847a78de369",
)
use_repo(cipd, "goldctl_linux-amd64")
`,
		"git_revision:808a00437f24bb404c09608ad8bf3847a78de369",
		map[string]string{
			"linux-amd64":   "Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
			"linux-arm64":   "XhM6rCCaRV3DJ4DkjsSTEahG-w3Er_ulss6w9-GwgkwC",
			"mac-amd64":     "1az5xiBG-ds55R4yd7fCkODv0xrApC5gC6iLb2SCig8C",
			"mac-arm64":     "bfOh3y10stM2Fj7HG-dDsJpJfm-J8yELSuoY94ec9UQC",
			"windows-amd64": "MeJ2G6pEJ4Vz3CvzoEf1QhrbEZqSzSh2uujaq7KwJtYC",
		},
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
	test("MODULE.bazel single",
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

	test(
		"MODULE.bazel meta",
		&config.VersionFileConfig{
			Id: "skia/tools/goldctl/${platform}",
			File: []*config.VersionFileConfig_File{
				{
					Path: "MODULE.bazel",
				},
			},
		},
		`
cipd.download_http(
    name = "goldctl",
    cipd_package = "skia/tools/goldctl/${platform}",
    platform_to_sha256 = {
        "linux-amd64":   "Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
        "linux-arm64":   "XhM6rCCaRV3DJ4DkjsSTEahG-w3Er_ulss6w9-GwgkwC",
        "mac-amd64":     "1az5xiBG-ds55R4yd7fCkODv0xrApC5gC6iLb2SCig8C",
        "mac-arm64":     "bfOh3y10stM2Fj7HG-dDsJpJfm-J8yELSuoY94ec9UQC",
        "windows-amd64": "MeJ2G6pEJ4Vz3CvzoEf1QhrbEZqSzSh2uujaq7KwJtYC",
    },
    tag = "git_revision:808a00437f24bb404c09608ad8bf3847a78de369",
)
use_repo(cipd, "goldctl_linux-amd64")
`,
		&revision.Revision{
			Id:       "git_revision:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Checksum: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Meta: map[string]string{
				"linux-amd64":   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"linux-arm64":   "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"mac-amd64":     "cccccccccccccccccccccccccccccccccccccccccccc",
				"mac-arm64":     "dddddddddddddddddddddddddddddddddddddddddddd",
				"windows-amd64": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
			},
		},
		`
cipd.download_http(
    name = "goldctl",
    cipd_package = "skia/tools/goldctl/${platform}",
    platform_to_sha256 = {
        "linux-amd64":   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "linux-arm64":   "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "mac-amd64":     "cccccccccccccccccccccccccccccccccccccccccccc",
        "mac-arm64":     "dddddddddddddddddddddddddddddddddddddddddddd",
        "windows-amd64": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
    },
    tag = "git_revision:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
)
use_repo(cipd, "goldctl_linux-amd64")
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

const exampleMulti = `Name: OpenXR SDK
Short Name: OpenXR
URL: https://github.com/KhronosGroup/OpenXR-SDK
Version: 1.1.53
Revision: 75c53b6e853dc12c7b3c771edc9c9c841b15faaa
Update Mechanism: Manual
License: Apache-2.0
License File: src/LICENSE
Security Critical: yes
Shipped: yes

Description:
OpenXR is a royalty-free, open standard that provides high-performance access to
Augmented Reality (AR) and Virtual Reality (VR)—collectively known as
XR—platforms and devices.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the OpenXR loader in Chromium, along with its dependencies. The
readme was expanded with information about transitive dependencies that are
copied directly into the OpenXR SDK repository. An openxr.def file works around
the fact that attributes aren't exposed by default for the compiler we use on
windows in component builds.

Added dev/xr_android.h for prototyping xr_android extensions that are currently
under active development and not in any openxr release at present. This file is
expected to be superceded by any official definitions and may require additional
work before a roll containing those definitions can be conducted.

Copied src/.clang-format into src_overrides/.clang-format and disabled
clang-format in src_overrides/src/external to mimic how khronos gitlab seems to
behave. This allows forked files to more closely match the base-files and allow
for easier "Compare with clipboard" comparisons.

The following changes should be reflected in 'src_overrides/patches':
* Forked android_utilites.cpp and manifest_file.cpp to allow for customizing to
ignore loading in Android ContentProvider supplied paths while investigating and
waiting for upstreaming.
* Forked AndroidManifest.xml.in to remove unnecessary fields that prevent
merging with Chrome's AndroidManifest.xml

-------------------- DEPENDENCY DIVIDER --------------------

Name: JNIPP
Short Name: JNIPP
URL: https://github.com/mitchdowd/jnipp
Version: v1.0.0-13-gcdd6293
Revision: cdd6293fca985993129f5ef5441709fc49ee507f
Update Mechanism: Manual
License: MIT
License File: src/src/external/jnipp/LICENSE
Security Critical: yes
Shipped: yes

Description:
JNIPP is just a C++ wrapper for the standard Java Native Interface (JNI).It
tries to take some of the long-winded annoyance out of integrating your Java
and C++ code.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the library in Chromium.

-------------------- DEPENDENCY DIVIDER --------------------

Name: android-jni-wrappers
Short Name: android-jni-wrappers
URL: https://gitlab.freedesktop.org/monado/utilities/android-jni-wrappers
Version: N/A
Date: 2023-12-13
Update Mechanism: Manual
License: Apache-2.0
License File: src/LICENSES/Apache-2.0.txt
Security Critical: yes
Shipped: yes

Description:
Python tool to generate C++ wrappers for (mostly Android-related) JNI/Java
objects. Generated files are typically slightly hand-modified.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the library in Chromium, along with its dependencies. Since it
is a transitive dependency that was directly included in OpenXR SDK repository,
the exact revision is unknown. The library also does not have any versioned
releases. The library contains auto-generated files with unknown hand-made
modifications. The library is triple-licensed, and the copy from OpenXR SDK
repository does not include a LICENSE file.
`

func TestGetPinnedRev_ReadmeChromium_Multi(t *testing.T) {
	file := &config.VersionFileConfig_File{
		Path: "path/to/README.chromium",
	}
	test := func(id, expectRev string) {
		name := path.Base(id)
		t.Run(name, func(t *testing.T) {
			actual, _, err := getPinnedRevInFile(id, file, exampleMulti)
			require.NoError(t, err)
			require.Equal(t, expectRev, actual)
		})
	}

	// This one has the GitHub URL in README.chromium but googlesource in DEPS.
	test("https://chromium.googlesource.com/external/github.com/KhronosGroup/OpenXR-SDK", "75c53b6e853dc12c7b3c771edc9c9c841b15faaa")

	// This isn't in DEPS but we'll query it by URL anyway.
	test("https://github.com/mitchdowd/jnipp", "cdd6293fca985993129f5ef5441709fc49ee507f")

	// Empty Revision field
	test("https://gitlab.freedesktop.org/monado/utilities/android-jni-wrappers", "")
}

func TestSetPinnedRev_ReadmeChromium_Multi(t *testing.T) {
	file := &config.VersionFileConfig_File{
		Path: "path/to/README.chromium",
	}
	depId := "https://chromium.googlesource.com/external/github.com/KhronosGroup/OpenXR-SDK"
	rev := &revision.Revision{
		Id:      "newRev",
		Release: "2.1.86",
	}
	actual, err := setPinnedRevInFile(depId, file, rev, exampleMulti)
	require.NoError(t, err)
	require.Equal(t, `Name: OpenXR SDK
Short Name: OpenXR
URL: https://github.com/KhronosGroup/OpenXR-SDK
Version: 2.1.86
Revision: newRev
Update Mechanism: Autoroll
License: Apache-2.0
License File: src/LICENSE
Security Critical: yes
Shipped: yes

Description:
OpenXR is a royalty-free, open standard that provides high-performance access to
Augmented Reality (AR) and Virtual Reality (VR)—collectively known as
XR—platforms and devices.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the OpenXR loader in Chromium, along with its dependencies. The
readme was expanded with information about transitive dependencies that are
copied directly into the OpenXR SDK repository. An openxr.def file works around
the fact that attributes aren't exposed by default for the compiler we use on
windows in component builds.

Added dev/xr_android.h for prototyping xr_android extensions that are currently
under active development and not in any openxr release at present. This file is
expected to be superceded by any official definitions and may require additional
work before a roll containing those definitions can be conducted.

Copied src/.clang-format into src_overrides/.clang-format and disabled
clang-format in src_overrides/src/external to mimic how khronos gitlab seems to
behave. This allows forked files to more closely match the base-files and allow
for easier "Compare with clipboard" comparisons.

The following changes should be reflected in 'src_overrides/patches':
* Forked android_utilites.cpp and manifest_file.cpp to allow for customizing to
ignore loading in Android ContentProvider supplied paths while investigating and
waiting for upstreaming.
* Forked AndroidManifest.xml.in to remove unnecessary fields that prevent
merging with Chrome's AndroidManifest.xml

-------------------- DEPENDENCY DIVIDER --------------------

Name: JNIPP
Short Name: JNIPP
URL: https://github.com/mitchdowd/jnipp
Version: v1.0.0-13-gcdd6293
Revision: cdd6293fca985993129f5ef5441709fc49ee507f
Update Mechanism: Manual
License: MIT
License File: src/src/external/jnipp/LICENSE
Security Critical: yes
Shipped: yes

Description:
JNIPP is just a C++ wrapper for the standard Java Native Interface (JNI).It
tries to take some of the long-winded annoyance out of integrating your Java
and C++ code.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the library in Chromium.

-------------------- DEPENDENCY DIVIDER --------------------

Name: android-jni-wrappers
Short Name: android-jni-wrappers
URL: https://gitlab.freedesktop.org/monado/utilities/android-jni-wrappers
Version: N/A
Date: 2023-12-13
Update Mechanism: Manual
License: Apache-2.0
License File: src/LICENSES/Apache-2.0.txt
Security Critical: yes
Shipped: yes

Description:
Python tool to generate C++ wrappers for (mostly Android-related) JNI/Java
objects. Generated files are typically slightly hand-modified.

Local Modifications:
No modifications to upstream files. BUILD.gn contains all of the configurations
needed to build the library in Chromium, along with its dependencies. Since it
is a transitive dependency that was directly included in OpenXR SDK repository,
the exact revision is unknown. The library also does not have any versioned
releases. The library contains auto-generated files with unknown hand-made
modifications. The library is triple-licensed, and the copy from OpenXR SDK
repository does not include a LICENSE file.
`, actual)
}
