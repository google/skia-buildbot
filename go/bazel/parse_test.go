package bazel

import (
	"testing"

	"github.com/go-python/gpython/ast"
	"github.com/stretchr/testify/require"
)

const fakeWorkspace = `
http_file(
    name = "buildifier_linux_amd64",
    downloaded_file_path = "buildifier",
    executable = True,
    sha256 = "52bf6b102cb4f88464e197caac06d69793fa2b05f5ad50a7e7bf6fbd656648a3",
    urls = gcs_mirror_url(
        ext = "",
        sha256 = "52bf6b102cb4f88464e197caac06d69793fa2b05f5ad50a7e7bf6fbd656648a3",
        url = "https://github.com/bazelbuild/buildtools/releases/download/5.1.0/buildifier-linux-amd64",
    ),
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
    strip_prefix = "rules_docker-0.24.0",
    urls = gcs_mirror_url(
        sha256 = "27d53c1d646fc9537a70427ad7b034734d08a9c38924cc6357cc973fed300820",
        url = "https://github.com/bazelbuild/rules_docker/releases/download/v0.24.0/rules_docker-v0.24.0.tar.gz",
    ),
)

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

# Provides the pkg_tar rule, needed by the skia_app_container macro.
#
# See https://github.com/bazelbuild/rules_pkg/tree/main/pkg.
http_archive(
    name = "rules_pkg",
    sha256 = "038f1caa773a7e35b3663865ffb003169c6a71dc995e39bf4815792f385d837d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz",
        "https://github.com/bazelbuild/rules_pkg/releases/download/0.4.0/rules_pkg-0.4.0.tar.gz",
    ],
)

# Pulls the gcr.io/skia-public/skia-wasm-release container with the Skia WASM build.
container_pull(
    name = "container_pull_skia_wasm",
    digest = "sha256:cdd850f28dcf58c93339a264ba63c87bb76694daac7d8bc5720e8f4ae71fb12d",
    registry = "gcr.io",
    repository = "skia-public/skia-wasm-release",
)

# This is an arbitrary version of the public Alpine image. Given our current rules, we must pull
# a docker container and extract some files, even if we are just building local versions (e.g.
# of debugger or skottie), so this is the image for that.
container_pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
    registry = "index.docker.io",
    repository = "alpine",
)

cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
    tag = "version:2.29.2.chromium.6",
)
`

const fakeModule = `
module(name = "test_module", version = "")

# Pulls the gcr.io/skia-public/skia-wasm-release container with the Skia WASM build.
oci.pull(
    name = "oci.pull_skia_wasm",
    digest = "sha256:cdd850f28dcf58c93339a264ba63c87bb76694daac7d8bc5720e8f4ae71fb12d",
    image = "gcr.io/skia-public/skia-wasm-release",
)

# This is an arbitrary version of the public Alpine image. Given our current rules, we must pull
# a docker container and extract some files, even if we are just building local versions (e.g.
# of debugger or skottie), so this is the image for that.
oci.pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
    image = "index.docker.io/alpine",
)

cipd.package(
    name = "git_amd64_linux",
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
    tag = "version:2.29.2.chromium.6",
)
`

const fakeModule2 = `
module(name = "test_module", version = "")

cipd = use_extension("@skia-infra//bazel/external:cipd_install.bzl", "cipd")

_single_cipd_file = """
exports_files(
    [
        "%s",
    ],
    visibility = ["//visibility:public"]
)
"""

cipd.download_http(
    name = "autoroll-config-generator",
    build_file_content = _single_cipd_file % "autoroll-config-generator",
    cipd_package = "skia/tools/autoroll-config-generator/linux-amd64",
    sha256 = "d634b39d83a6caab822f6b7f6511edf35559bc42db15e05daea564f086515ee0",
    tag = "git_revision:22e0531dac73e5a70136e50f84ce2e435a5060bf",
)
use_repo(cipd, "autoroll-config-generator")

cipd.download_cipd(
    name = "via-cipd",
    cipd_package = "skia/internal/third_party/via-cipd",
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
    tag = "version:2.2.1",
)
use_repo(cipd, "via-cipd")
`

func TestParseDeps_ReturnsMapOfAllDependencies_WORKSPACE(t *testing.T) {
	actual, err := ParseDeps(fakeWorkspace)
	require.NoError(t, err)
	require.Equal(t, map[DependencyID]Dependency{
		"gcr.io/skia-public/skia-wasm-release": {
			ID:      "gcr.io/skia-public/skia-wasm-release",
			Version: "sha256:cdd850f28dcf58c93339a264ba63c87bb76694daac7d8bc5720e8f4ae71fb12d",
			versionPos: &ast.Pos{
				Lineno:    44,
				ColOffset: 13,
			},
			SHA256: "sha256:cdd850f28dcf58c93339a264ba63c87bb76694daac7d8bc5720e8f4ae71fb12d",
			sha256Pos: &ast.Pos{
				Lineno:    44,
				ColOffset: 13,
			},
		},
		"index.docker.io/alpine": {
			ID:      "index.docker.io/alpine",
			Version: "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
			versionPos: &ast.Pos{
				Lineno:    54,
				ColOffset: 13,
			},
			SHA256: "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
			sha256Pos: &ast.Pos{
				Lineno:    54,
				ColOffset: 13,
			},
		},
		"infra/3pp/tools/git/linux-amd64": {
			ID:      "infra/3pp/tools/git/linux-amd64",
			Version: "version:2.29.2.chromium.6",
			versionPos: &ast.Pos{
				Lineno:    70,
				ColOffset: 10,
			},
			SHA256: "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
			sha256Pos: &ast.Pos{
				Lineno:    69,
				ColOffset: 13,
			},
		},
	}, actual)
}

func TestParseDeps_ReturnsMapOfAllDependencies_MODULE(t *testing.T) {
	actual, err := ParseDeps(fakeModule)
	require.NoError(t, err)
	require.Equal(t, map[DependencyID]Dependency{
		"gcr.io/skia-public/skia-wasm-release": {
			ID:      "gcr.io/skia-public/skia-wasm-release",
			Version: "sha256:cdd850f28dcf58c93339a264ba63c87bb76694daac7d8bc5720e8f4ae71fb12d",
			versionPos: &ast.Pos{
				Lineno:    7,
				ColOffset: 13,
			},
			SHA256: "sha256:cdd850f28dcf58c93339a264ba63c87bb76694daac7d8bc5720e8f4ae71fb12d",
			sha256Pos: &ast.Pos{
				Lineno:    7,
				ColOffset: 13,
			},
		},
		"index.docker.io/alpine": {
			ID:      "index.docker.io/alpine",
			Version: "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
			versionPos: &ast.Pos{
				Lineno:    16,
				ColOffset: 13,
			},
			SHA256: "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
			sha256Pos: &ast.Pos{
				Lineno:    16,
				ColOffset: 13,
			},
		},
		"infra/3pp/tools/git/linux-amd64": {
			ID:      "infra/3pp/tools/git/linux-amd64",
			Version: "version:2.29.2.chromium.6",
			versionPos: &ast.Pos{
				Lineno:    30,
				ColOffset: 10,
			},
			SHA256: "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
			sha256Pos: &ast.Pos{
				Lineno:    29,
				ColOffset: 13,
			},
		},
	}, actual)
}

func TestParseDeps_ReturnsMapOfAllDependencies_MODULE2(t *testing.T) {
	actual, err := ParseDeps(fakeModule2)
	require.NoError(t, err)
	require.Equal(t, map[DependencyID]Dependency{
		"skia/tools/autoroll-config-generator/linux-amd64": {
			ID:      "skia/tools/autoroll-config-generator/linux-amd64",
			Version: "git_revision:22e0531dac73e5a70136e50f84ce2e435a5060bf",
			versionPos: &ast.Pos{
				Lineno:    20,
				ColOffset: 10,
			},
			SHA256: "d634b39d83a6caab822f6b7f6511edf35559bc42db15e05daea564f086515ee0",
			sha256Pos: &ast.Pos{
				Lineno:    19,
				ColOffset: 13,
			},
		},
		"skia/internal/third_party/via-cipd": {
			ID:      "skia/internal/third_party/via-cipd",
			Version: "version:2.2.1",
			versionPos: &ast.Pos{
				Lineno:    28,
				ColOffset: 10,
			},
			SHA256: "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
			sha256Pos: &ast.Pos{
				Lineno:    27,
				ColOffset: 13,
			},
		},
	}, actual)
}

func TestParseDeps_InvalidFile_WORKSPACE(t *testing.T) {
	check := func(name, errContains, content string) {
		t.Run(name, func(t *testing.T) {
			actual, err := ParseDeps(content)
			require.Error(t, err)
			require.Contains(t, err.Error(), errContains)
			require.Nil(t, actual)
		})
	}
	check("Not Python", "invalid syntax", `rm -rf /`)
	check("Missing Commas", "invalid syntax", `
container_pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
    registry = "index.docker.io"
    repository = "alpine"
)
`)
	check("Missing Registry", `no keyword argument "registry" found for call`, `
container_pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
    repository = "alpine",
)
`)
	check("Empty Registry", `found empty string for argument "registry"`, `
container_pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
    registry = "",
    repository = "alpine",
)
`)
	check("Missing Repository", `no keyword argument "repository" found for call`, `
container_pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
    registry = "index.docker.io",
)
`)
	check("Empty Repository", `found empty string for argument "repository"`, `
container_pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
    registry = "index.docker.io",
	repository = "",
)
`)

	check("Missing Digest", `no keyword argument "digest" found for call`, `
container_pull(
    name = "empty_container",
    registry = "index.docker.io",
	repository = "alpine",
)
`)
	check("Empty Digest", `found empty string for argument "digest"`, `
container_pull(
    name = "empty_container",
    digest = "",
    registry = "index.docker.io",
	repository = "alpine",
)
`)

	check("Missing CIPD Package", `no keyword argument "cipd_package" found for call`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
)
`)

	check("Empty CIPD Package", `found empty string for argument "cipd_package"`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
)
`)

	check("Missing SHA256", `no keyword argument "tag" found for call`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
)
`)

	check("Empty SHA256", `found empty string for argument "tag"`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    tag = "",
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
)
`)

	check("Missing SHA256", `no keyword argument "sha256" found for call`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    tag = "latest",
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
)
`)

	check("Empty SHA256", `found empty string for argument "sha256"`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    tag = "latest",
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "",
)
`)
}

func TestParseDeps_InvalidFile_MODULE(t *testing.T) {
	check := func(name, errContains, content string) {
		t.Run(name, func(t *testing.T) {
			actual, err := ParseDeps(content)
			require.Error(t, err)
			require.Contains(t, err.Error(), errContains)
			require.Nil(t, actual)
		})
	}
	check("Not Python", "invalid syntax", `rm -rf /`)
	check("Missing Commas", "invalid syntax", `
oci.pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801"
    image = "index.docker.io/alpine"
)
`)
	check("Missing Image", `no keyword argument "image" found for call`, `
oci.pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
)
`)
	check("Empty Image", `found empty string for argument "image"`, `
oci.pull(
    name = "empty_container",
    digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
	image = "",
)
`)

	check("Missing Digest", `no keyword argument "digest" found for call`, `
oci.pull(
    name = "empty_container",
    image = "index.docker.io/alpine",
)
`)
	check("Empty Digest", `found empty string for argument "digest"`, `
oci.pull(
    name = "empty_container",
    digest = "",
    image = "index.docker.io/alpine",
)
`)

	check("Missing CIPD Package", `no keyword argument "cipd_package" found for call`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
)
`)

	check("Empty CIPD Package", `found empty string for argument "cipd_package"`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
)
`)

	check("Missing SHA256", `no keyword argument "tag" found for call`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
)
`)

	check("Empty SHA256", `found empty string for argument "tag"`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    tag = "",
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
)
`)

	check("Missing SHA256", `no keyword argument "sha256" found for call`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    tag = "latest",
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
)
`)

	check("Empty SHA256", `found empty string for argument "sha256"`, `
cipd_install(
    name = "git_amd64_linux",
    build_file_content = all_cipd_files(),
    cipd_package = "infra/3pp/tools/git/linux-amd64",
    postinstall_cmds_posix = [
        "mkdir etc",
        "bin/git config --system user.name \"Bazel Test User\"",
        "bin/git config --system user.email \"bazel-test-user@example.com\"",
    ],
    tag = "latest",
    # From https://chrome-infra-packages.appspot.com/p/infra/3pp/tools/git/linux-amd64/+/version:2.29.2.chromium.6
    sha256 = "",
)
`)
}

func TestGetDep_ReturnsRequestedDependency_WORKSPACE(t *testing.T) {
	check := func(id DependencyID, expect Dependency) {
		actual, err := GetDep(fakeWorkspace, id)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
	}
	check("gcr.io/skia-public/skia-wasm-release", Dependency{
		ID:      "gcr.io/skia-public/skia-wasm-release",
		Version: "sha256:cdd850f28dcf58c93339a264ba63c87bb76694daac7d8bc5720e8f4ae71fb12d",
		versionPos: &ast.Pos{
			Lineno:    44,
			ColOffset: 13,
		},
		SHA256: "sha256:cdd850f28dcf58c93339a264ba63c87bb76694daac7d8bc5720e8f4ae71fb12d",
		sha256Pos: &ast.Pos{
			Lineno:    44,
			ColOffset: 13,
		},
	})
	check("index.docker.io/alpine", Dependency{
		ID:      "index.docker.io/alpine",
		Version: "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
		versionPos: &ast.Pos{
			Lineno:    54,
			ColOffset: 13,
		},
		SHA256: "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
		sha256Pos: &ast.Pos{
			Lineno:    54,
			ColOffset: 13,
		},
	})
	check("infra/3pp/tools/git/linux-amd64", Dependency{
		ID:      "infra/3pp/tools/git/linux-amd64",
		Version: "version:2.29.2.chromium.6",
		versionPos: &ast.Pos{
			Lineno:    70,
			ColOffset: 10,
		},
		SHA256: "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
		sha256Pos: &ast.Pos{
			Lineno:    69,
			ColOffset: 13,
		},
	})
}

func TestGetDep_ReturnsRequestedDependency_MODULE(t *testing.T) {
	check := func(id DependencyID, expect Dependency) {
		actual, err := GetDep(fakeModule, id)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
	}
	check("gcr.io/skia-public/skia-wasm-release", Dependency{
		ID:      "gcr.io/skia-public/skia-wasm-release",
		Version: "sha256:cdd850f28dcf58c93339a264ba63c87bb76694daac7d8bc5720e8f4ae71fb12d",
		versionPos: &ast.Pos{
			Lineno:    7,
			ColOffset: 13,
		},
		SHA256: "sha256:cdd850f28dcf58c93339a264ba63c87bb76694daac7d8bc5720e8f4ae71fb12d",
		sha256Pos: &ast.Pos{
			Lineno:    7,
			ColOffset: 13,
		},
	})
	check("index.docker.io/alpine", Dependency{
		ID:      "index.docker.io/alpine",
		Version: "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
		versionPos: &ast.Pos{
			Lineno:    16,
			ColOffset: 13,
		},
		SHA256: "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
		sha256Pos: &ast.Pos{
			Lineno:    16,
			ColOffset: 13,
		},
	})
	check("infra/3pp/tools/git/linux-amd64", Dependency{
		ID:      "infra/3pp/tools/git/linux-amd64",
		Version: "version:2.29.2.chromium.6",
		versionPos: &ast.Pos{
			Lineno:    30,
			ColOffset: 10,
		},
		SHA256: "36cb96051827d6a3f6f59c5461996fe9490d997bcd2b351687d87dcd4a9b40fa",
		sha256Pos: &ast.Pos{
			Lineno:    29,
			ColOffset: 13,
		},
	})
}

func TestGetDep_ErrorForUnknownID_WORKSPACE(t *testing.T) {
	check := func(id DependencyID) {
		_, err := GetDep(fakeWorkspace, id)
		require.Error(t, err)
	}
	check("container_pull_skia_wasm")
	check("bogus-id")
	check("../../")
}

func TestGetDep_ErrorForUnknownID_MODULE(t *testing.T) {
	check := func(id DependencyID) {
		_, err := GetDep(fakeModule, id)
		require.Error(t, err)
	}
	check("container_pull_skia_wasm")
	check("bogus-id")
	check("../../")
}

func TestSetDep_RoundTripsContentWithGetDep_WORKSPACE(t *testing.T) {
	const fakeNewVersion = "fake-new-version"
	const fakeNewSHA256 = "fake-new-sha256"
	check := func(id DependencyID, versionIsSHA256 bool) {
		newContents, err := SetDep(fakeWorkspace, id, fakeNewVersion, fakeNewSHA256)
		require.NoError(t, err)
		actual, err := GetDep(newContents, id)
		require.NoError(t, err)
		require.Equal(t, fakeNewVersion, actual.Version)
		if !versionIsSHA256 {
			require.Equal(t, fakeNewSHA256, actual.SHA256)
		}
	}
	check("gcr.io/skia-public/skia-wasm-release", true)
	check("index.docker.io/alpine", true)
	check("infra/3pp/tools/git/linux-amd64", false)
}

func TestSetDep_RoundTripsContentWithGetDep_MODULE(t *testing.T) {
	const fakeNewVersion = "fake-new-version"
	const fakeNewSHA256 = "fake-new-sha256"
	check := func(id DependencyID, versionIsSHA256 bool) {
		newContents, err := SetDep(fakeWorkspace, id, fakeNewVersion, fakeNewSHA256)
		require.NoError(t, err)
		actual, err := GetDep(newContents, id)
		require.NoError(t, err)
		require.Equal(t, fakeNewVersion, actual.Version)
		if !versionIsSHA256 {
			require.Equal(t, fakeNewSHA256, actual.SHA256)
		}
	}
	check("gcr.io/skia-public/skia-wasm-release", true)
	check("index.docker.io/alpine", true)
	check("infra/3pp/tools/git/linux-amd64", false)
}

func TestSetDep_MatchesExpectedContent_WORKSPACE(t *testing.T) {
	input := `
container_pull(
  name = "empty_container",
  digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
  registry = "index.docker.io",
  repository = "alpine",
)
`
	expect := `
container_pull(
  name = "empty_container",
  digest = "fake-new-version",
  registry = "index.docker.io",
  repository = "alpine",
)
`
	actual, err := SetDep(input, "index.docker.io/alpine", "fake-new-version", "fake-new-sha256")
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestSetDep_MatchesExpectedContent_MODULE(t *testing.T) {
	input := `
oci.pull(
  name = "empty_container",
  digest = "sha256:1e014f84205d569a5cc3be4e108ca614055f7e21d11928946113ab3f36054801",
  image = "index.docker.io/alpine",
)
`
	expect := `
oci.pull(
  name = "empty_container",
  digest = "fake-new-version",
  image = "index.docker.io/alpine",
)
`
	actual, err := SetDep(input, "index.docker.io/alpine", "fake-new-version", "fake-new-sha256")
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestSetDep_ErrorForUnknownID_WORKSPACE(t *testing.T) {
	check := func(id DependencyID) {
		actual, err := SetDep(fakeWorkspace, id, "fake-new-version", "fake-new-sha256")
		require.Empty(t, actual)
		require.Error(t, err)
	}
	check("container_pull_skia_wasm")
	check("bogus-id")
	check("../../")
}

func TestSetDep_ErrorForUnknownID_MODULE(t *testing.T) {
	check := func(id DependencyID) {
		actual, err := SetDep(fakeModule, id, "fake-new-version", "fake-new-sha256")
		require.Empty(t, actual)
		require.Error(t, err)
	}
	check("container_pull_skia_wasm")
	check("bogus-id")
	check("../../")
}
