package cpp

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/testutils/unittest"
)

// makeBasicWorkspace returns the minimum files necessary for the Gazelle extension to work.
func makeBasicWorkspace() []testtools.FileSpec {
	return []testtools.FileSpec{
		{Path: "WORKSPACE"}, // Gazelle requires that a WORKSPACE file exists, even if it's empty.
	}
}

func TestGazelle_NoExistingBuildFile_GeneratesBuildRules(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := append([]testtools.FileSpec{
		{
			Path: "include/avocado.h",
			Content: `
#define AVOCADO
`,
		}, {
			Path: "include/avocado.cpp",
			Content: `
#include "include/avocado.h"
`,
		}, {
			Path: "include/avocado_util.cpp",
			Content: `
#include "include/avocado.h"
#include "include/common_foods.h"
#include "spirv-tools/libspirv.hpp"
#include "third_party/externals/spirv-cross/spirv_hlsl.hpp"
`,
		}, {
			Path: "include/common_foods.h",
			Content: `
#define COMMON
`,
		},
		{
			Path: "include/avocado_desert.cpp",
			Content: `
#include "include/avocado.h"
#include "experimental/desserts/pudding.h"
#include "png.h"
#include "jpeg.h"
#include <string>
`,
		},
	}, makeBasicWorkspace()...)

	// Note: we see "@spirv_tools" instead of "@spirv_tools//:spirv_tools" (the value specified
	// in test_filemap.json) because the former can be simplified to the latter.
	// "When it matches the last component of the package path, it, and the colon, may be omitted"
	// https://bazel.build/concepts/labels
	// Gazelle does this simplification after generating the file.
	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "include/BUILD.bazel",
			Content: `
load("//bazel:macros.bzl", "generated_cc_atom")

generated_cc_atom(
    name = "avocado_desert_src",
    srcs = ["avocado_desert.cpp"],
    visibility = ["//:__subpackages__"],
    deps = [
        ":avocado_hdr",
        "//experimental/desserts:pudding_hdr",
        "//third_party:libpng",
        "@jpeg//:with_special_features",
    ],
)

generated_cc_atom(
    name = "avocado_hdr",
    hdrs = ["avocado.h"],
    visibility = ["//:__subpackages__"],
)

generated_cc_atom(
    name = "avocado_src",
    srcs = ["avocado.cpp"],
    visibility = ["//:__subpackages__"],
    deps = [":avocado_hdr"],
)

generated_cc_atom(
    name = "avocado_util_src",
    srcs = ["avocado_util.cpp"],
    visibility = ["//:__subpackages__"],
    deps = [
        ":avocado_hdr",
        ":common_foods_hdr",
        "@spirv_cross",
        "@spirv_tools",
    ],
)

generated_cc_atom(
    name = "common_foods_hdr",
    hdrs = ["common_foods.h"],
    visibility = ["//:__subpackages__"],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

func TestGazelle_ExistingBuildFiles_NewIncludesAreAdded(t *testing.T) {
	unittest.BazelOnlyTest(t)

	// In this testcase, we pretend two new includes were added to util.cpp
	inputFiles := append([]testtools.FileSpec{
		{
			Path: "src/BUILD.bazel",
			Content: `load("@rules_cc//cc:defs.bzl", "cc_library")
load("//bazel:macros.bzl", "generated_cc_atom")

cc_library(
	name = "hand_written_rule",
	deps = [
		":that_rule",
		"//experimental:this_rule",
	],
)

generated_cc_atom(
    name = "util_hdr",
    hdrs = ["util.h"],
    visibility = ["//:__subpackages__"],
    deps = ["//include:avocado_hdr"],
)

generated_cc_atom(
    name = "util_src",
    srcs = ["util.cpp"],
    visibility = ["//:__subpackages__"],
    deps = [":util_hdr"],
)

`,
		},
		{
			Path: "src/util.h",
			Content: `
#include "include/avocado.h"
`,
		},
		{
			Path: "src/util.cpp",
			Content: `
#include "src/util.h"

#include "include/avocado.h"
#include "include/common_foods.h"
`,
		},
	}, makeBasicWorkspace()...)

	// We expect those to be added here.
	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "src/BUILD.bazel",
			Content: `load("@rules_cc//cc:defs.bzl", "cc_library")
load("//bazel:macros.bzl", "generated_cc_atom")

cc_library(
    name = "hand_written_rule",
    deps = [
        ":that_rule",
        "//experimental:this_rule",
    ],
)

generated_cc_atom(
    name = "util_hdr",
    hdrs = ["util.h"],
    visibility = ["//:__subpackages__"],
    deps = ["//include:avocado_hdr"],
)

generated_cc_atom(
    name = "util_src",
    srcs = ["util.cpp"],
    visibility = ["//:__subpackages__"],
    deps = [
        ":util_hdr",
        "//include:avocado_hdr",
        "//include:common_foods_hdr",
    ],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

func TestGazelle_ExistingBuildFiles_MissingIncludesAreRemoved(t *testing.T) {
	unittest.BazelOnlyTest(t)

	// In this testcase, we pretend one include was removed from util.cpp
	inputFiles := append([]testtools.FileSpec{
		{
			Path: "src/BUILD.bazel",
			Content: `load("@rules_cc//cc:defs.bzl", "cc_library")
load("//bazel:macros.bzl", "generated_cc_atom")

cc_library(
	name = "hand_written_rule",
	deps = [
		":that_rule",
		"//experimental:this_rule",
	],
)

generated_cc_atom(
    name = "util_hdr",
    hdrs = ["util.h"],
    visibility = ["//:__subpackages__"],
    deps = ["//include:avocado_hdr"],
)

generated_cc_atom(
    name = "util_src",
    srcs = ["util.cpp"],
    visibility = ["//:__subpackages__"],
    deps = [
        ":util_hdr",
        "//include:common_foods_hdr",
        "//tools/not/there:anymore_hdr",
    ],
)

`,
		},
		{
			Path: "src/util.h",
			Content: `
#include "include/avocado.h"
`,
		},
		{
			Path: "src/util.cpp",
			Content: `
#include "src/util.h"

#include "include/common_foods.h"
`,
		},
	}, makeBasicWorkspace()...)

	// We that include ("//tools/not/there:anymore_hdr") to not be there anymore
	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "src/BUILD.bazel",
			Content: `load("@rules_cc//cc:defs.bzl", "cc_library")
load("//bazel:macros.bzl", "generated_cc_atom")

cc_library(
    name = "hand_written_rule",
    deps = [
        ":that_rule",
        "//experimental:this_rule",
    ],
)

generated_cc_atom(
    name = "util_hdr",
    hdrs = ["util.h"],
    visibility = ["//:__subpackages__"],
    deps = ["//include:avocado_hdr"],
)

generated_cc_atom(
    name = "util_src",
    srcs = ["util.cpp"],
    visibility = ["//:__subpackages__"],
    deps = [
        ":util_hdr",
        "//include:common_foods_hdr",
    ],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

func TestGazelle_ExistingBuildFiles_MissingFilesHaveRulesRemoved(t *testing.T) {
	unittest.BazelOnlyTest(t)

	// In this testcase, we pretend util.cpp was deleted.
	inputFiles := append([]testtools.FileSpec{
		{
			Path: "src/BUILD.bazel",
			Content: `load("@rules_cc//cc:defs.bzl", "cc_library")
load("//bazel:macros.bzl", "generated_cc_atom")

cc_library(
	name = "hand_written_rule",
	deps = [
		":that_rule",
		"//experimental:this_rule",
	],
)

generated_cc_atom(
    name = "util_hdr",
    hdrs = ["util.h"],
    visibility = ["//:__subpackages__"],
    deps = ["//include:avocado_hdr"],
)

generated_cc_atom(
    name = "util_src",
    srcs = ["util.cpp"],
    visibility = ["//:__subpackages__"],
    deps = [
        ":util_hdr",
        "//include:common_foods_hdr",
    ],
)
`,
		},
		{
			Path: "src/util.h",
			Content: `
#include "include/avocado.h"
`,
		},
	}, makeBasicWorkspace()...)

	// We that rule to not be there anymore
	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "src/BUILD.bazel",
			Content: `load("@rules_cc//cc:defs.bzl", "cc_library")
load("//bazel:macros.bzl", "generated_cc_atom")

cc_library(
    name = "hand_written_rule",
    deps = [
        ":that_rule",
        "//experimental:this_rule",
    ],
)

generated_cc_atom(
    name = "util_hdr",
    hdrs = ["util.h"],
    visibility = ["//:__subpackages__"],
    deps = ["//include:avocado_hdr"],
)

`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

// test runs Gazelle on a temporary directory with the given input files, and asserts that Gazelle
// generated the expected output files.
func test(t *testing.T, inputFiles, expectedOutputFiles []testtools.FileSpec) {
	// These paths were determined with a bit of trial and error, using --sandbox_debug.
	gazelleAbsPath := filepath.Join(bazel.RunfilesDir(), "bazel/gazelle/cpp/gazelle_cpp_test_binary_/gazelle_cpp_test_binary")
	filemapAbsPath := filepath.Join(bazel.RunfilesDir(), "bazel/gazelle/cpp/test_filemap.json")
	// Write the input files to a temporary directory.
	dir, cleanup := testtools.CreateFiles(t, inputFiles)
	defer cleanup()

	// Run Gazelle.
	cmd := exec.Command(gazelleAbsPath, "--third_party_file_map", filemapAbsPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	// Assert that Gazelle generated the expected files.
	testtools.CheckFiles(t, dir, expectedOutputFiles)
}
