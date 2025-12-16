"""Helper function used to generate Go files containing runfile paths."""

load("@bazel_skylib//rules:write_file.bzl", "write_file")
load("//bazel/go:go_test.bzl", "go_test")

def generate_go_runfile_path(name, go_library_name, path, platform_mapping):
    """Creates a target which generates a Go file containing a runfile path.

    A different Go file in the same package must declare an empty `runfilePath`
    string and a Find() function which uses it.

    Also generates a Go test file which runs under "bazel test" and ensures that
    the generated runfile path is correct.

    Example usage:

    vpython3.go:
    ```
    package vpython3

    var runfilePath = ""

    func Find() (string, error) {
        return bazel.FindExecutable("vpython3", runfilePath)
    }
    ```

    BUILD.bazel:
    ```
    gen_filename, data = generate_go_runfile_path(
        go_library_name = "vpython3",
        path = "vpython3",
        platform_mapping = {
            "@platforms//os:linux": "@vpython_amd64_linux//:all_files",
        },
    )

    go_library(
        name = "vpython3",
        srcs = [
            "vpython3.go",
            gen_filename,
        ],
        data = data,
        importpath = "go.skia.org/infra/bazel/external/cipd/vpython3",
        visibility = ["//visibility:public"],
        deps = [
            "//bazel/go/bazel",
            "@rules_go//go/runfiles",
        ],
    )
    ```

    Args:
        name: Unused, but required by buildifier.
        go_library_name: Name of the go_library rule, expected to be the same as
            the go package name.
        path: Path of the runfile within the module subdirectory.
        platform_mapping: Dictionary mapping platforms to single targets,
            similar to what is commonly passed to select().

    Returns:
        A tuple containing:
          - The name of the generated Go file, which must be added to srcs of
            the go_library.
          - A select() instance derived from platform_mapping which should be
            passed directly to the data attribute of the go_library rule which
            consumes the generated Go file.
    """

    # First, generate the Go file which sets runfilePath as required.
    srcs_map = {k: [v] for k, v in platform_mapping.items()}
    if not srcs_map.get("//conditions:default"):
        srcs_map["//conditions:default"] = []

    cmd_tmpl = """
    locations=($(rlocationpaths {dependency}))
    runfilePath="$$(echo "$${{locations[0]}}" | cut -d "/" -f1)"

    # TODO(borenet): This is messy, but I couldn't get here-documents to work.
    echo "package {go_pkg_name}" > $@
    echo "func init() {{" >> $@
    echo "    runfilePath = \\"$$runfilePath/{path}\\"" >> $@
    echo "}}" >> $@
    """
    cmd_map = {k: cmd_tmpl.format(
        dependency = v,
        go_pkg_name = go_library_name,
        path = path,
    ) for k, v in platform_mapping.items()}
    if not cmd_map.get("//conditions:default"):
        cmd_map["//conditions:default"] = """echo "package %s" > $@""" % go_library_name

    go_file_name = go_library_name + "_gen.go"
    native.genrule(
        name = go_file_name + "_rule",
        srcs = select(srcs_map),
        outs = [go_file_name],
        cmd = select(cmd_map),
    )

    # Now, generate a Go test file and a go_test target for it.
    test_file_name = go_library_name + "_gen_test.go"
    write_file(
        name = test_file_name + "_rule",
        out = test_file_name,
        content = ("""package %s

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFind(t *testing.T) {
	_, err := Find()
	require.NoError(t, err)
}
""" % go_library_name).splitlines(),
    )
    go_test(
        name = go_library_name + "_test",
        srcs = [test_file_name],
        embed = [":" + go_library_name],
        deps = ["@com_github_stretchr_testify//require"],
    )

    return go_file_name, select(srcs_map)
