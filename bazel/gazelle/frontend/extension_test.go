package frontend

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

var gazelleBin = flag.String("gazelle_bin", "", "Path to the test-only Gazelle binary.")

// basicWorkspace contains the minimum files necessary for the Gazelle extension to work.
var basicWorkspace = []testtools.FileSpec{
	{Path: "WORKSPACE"}, // Gazelle requires that a WORKSPACE file exists, even if it's empty.
	{
		Path: "infra-sk/package.json",
		Content: `
{
  "dependencies": {
    "common-sk": "^3.4.1",
    "elements-sk": "^4.0.0",
    "lit-html": "~1.1.2"
  },
  "devDependencies": {
    "@types/puppeteer": "^3.0.0",
    "puppeteer": "^5.0.0"
  }
}
`,
	},
}

func TestSassLibrary_Generate_Success(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := append([]testtools.FileSpec{
		{
			Path: "a/alpha.scss",
			Content: `
@import 'beta';
@import 'b/delta';
@import '../c/epsilon';
@import '../d/d';
@import '~elements-sk/colors';
`,
		},
		{Path: "a/beta.scss"},
		{Path: "a/b/delta.scss"},
		{Path: "c/epsilon.scss"},
		{Path: "d/d.scss", Content: `// Empty file with the same name as its Bazel package ("d").`},
	}, basicWorkspace...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library")

sass_library(
    name = "alpha",
    srcs = ["alpha.scss"],
    visibility = ["//visibility:public"],
    deps = [
        ":beta",
        "//a/b:delta",
        "//c:epsilon",
        "//d",
        "//infra-sk:elements-sk_scss",
    ],
)

sass_library(
    name = "beta",
    srcs = ["beta.scss"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "a/b/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library")

sass_library(
    name = "delta",
    srcs = ["delta.scss"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "c/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library")

sass_library(
    name = "epsilon",
    srcs = ["epsilon.scss"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "d/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library")

sass_library(
    name = "d",
    srcs = ["d.scss"],
    visibility = ["//visibility:public"],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

func TestSassLibrary_Update_Success(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := append([]testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library")

sass_library(
    name = "alpha",
    srcs = ["alpha.scss"],
    visibility = ["//visibility:public"],
    deps = [
        ":beta",  # Not imported from alpha.scss. Gazelle should remove this dep.
        ":delta",
    ],
)

sass_library(
    name = "beta",
    srcs = ["beta.scss"],
    visibility = ["//visibility:public"],
)

sass_library(
    name = "delta",
    srcs = ["delta.scss"],
    visibility = ["//visibility:public"],
)

sass_library(
    name = "epsilon",
    srcs = ["epsilon.scss"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "a/alpha.scss",
			Content: `
@import 'delta';                // Existing import.
@import 'epsilon';              // New import. Gazelle should add this dep.
@import '~elements-sk/colors';  // New import. Gazelle should add this dep.
`,
		},
		{Path: "a/beta.scss"},
		{Path: "a/delta.scss"},
		{Path: "a/epsilon.scss"},
	}, basicWorkspace...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library")

sass_library(
    name = "alpha",
    srcs = ["alpha.scss"],
    visibility = ["//visibility:public"],
    deps = [
        ":delta",
        ":epsilon",
        "//infra-sk:elements-sk_scss",
    ],
)

sass_library(
    name = "beta",
    srcs = ["beta.scss"],
    visibility = ["//visibility:public"],
)

sass_library(
    name = "delta",
    srcs = ["delta.scss"],
    visibility = ["//visibility:public"],
)

sass_library(
    name = "epsilon",
    srcs = ["epsilon.scss"],
    visibility = ["//visibility:public"],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

func TestSassLibrary_Remove_Success(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := append([]testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library")

sass_library(
    name = "alpha",
    srcs = [
        "alpha.scss",
        "beta.scss",  # This file was deleted. Gazelle should remove this dep.
    ],
    visibility = ["//visibility:public"],
)

sass_library(
    name = "beta",
    srcs = ["beta.scss"],  # This file was deleted. Gazelle should remove this target.
    visibility = ["//visibility:public"],
)
`,
		},
		{Path: "a/alpha.scss"},
	}, basicWorkspace...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library")

sass_library(
    name = "alpha",
    srcs = ["alpha.scss"],
    visibility = ["//visibility:public"],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

func TestTSLibrary_Generate_Success(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := append([]testtools.FileSpec{
		{
			Path: "a/alpha.ts",
			Content: `
// Empty file, no imports.
`,
		},
		{
			Path: "a/beta.ts",
			Content: `
import './alpha';
import '../b/b';
import '../b/delta';
import '../c';
import 'lit-html';   // NPM import with built-in TypeScript annotations.
import 'puppeteer';  // NPM import with a separate @types/puppeteer package.
import 'net'         // Built-in Node.js module.
`,
		},
		{Path: "b/delta.ts", Content: `import 'common-sk'; // NPM import.`},
		{Path: "b/b.ts", Content: `// Empty file with the same name as its Bazel package ("b").`},
		{Path: "c/index.ts", Content: `// Empty file which may be imported as its parent folder's "main" module.`},
	}, basicWorkspace...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "alpha",
    srcs = ["alpha.ts"],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "beta",
    srcs = ["beta.ts"],
    visibility = ["//visibility:public"],
    deps = [
        ":alpha",
        "//b",
        "//b:delta",
        "//c:index",
        "@infra-sk_npm//@types/puppeteer",
        "@infra-sk_npm//lit-html",
        "@infra-sk_npm//puppeteer",
    ],
)
`,
		},
		{
			Path: "b/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "b",
    srcs = ["b.ts"],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "delta",
    srcs = ["delta.ts"],
    visibility = ["//visibility:public"],
    deps = ["@infra-sk_npm//common-sk"],
)
`,
		},
		{
			Path: "c/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "index",
    srcs = ["index.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

func TestTSLibrary_Update_Success(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := append([]testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "alpha",
    srcs = ["alpha.ts"],
    visibility = ["//visibility:public"],
    deps = [
        "@infra-sk_npm//common-sk",    # Not imported from alpha.ts. Gazelle should remove this dep.
        "@infra-sk_npm//elements-sk",
    ],
)
`,
		},
		{
			Path: "a/alpha.ts",
			Content: `
import 'elements-sk';  // Existing import.
import 'lit-html';     // New import.
`,
		},
	}, basicWorkspace...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "alpha",
    srcs = ["alpha.ts"],
    visibility = ["//visibility:public"],
    deps = [
        "@infra-sk_npm//elements-sk",
        "@infra-sk_npm//lit-html",
    ],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

func TestTSLibrary_Remove_Success(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := append([]testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "alpha",
    srcs = [
        "alpha.ts",
        "beta.ts",  # This file was deleted. Gazelle should remove this src.
    ],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "beta",
    srcs = ["beta.ts"],  # This file was deleted. Gazelle should remove this target.
    visibility = ["//visibility:public"],
)
`,
		},
		{Path: "a/alpha.ts"},
	}, basicWorkspace...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "alpha",
    srcs = ["alpha.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

// test runs Gazelle on a temporary directory with the given input files, and asserts that Gazelle
// generated the expected output files.
func test(t *testing.T, inputFiles, expectedOutputFiles []testtools.FileSpec) {
	flag.Parse()
	gazelleAbsPath, err := filepath.Abs(*gazelleBin)
	require.NoError(t, err)

	// Create the input files.
	dir, cleanup := testtools.CreateFiles(t, inputFiles)
	defer cleanup()

	// Run Gazelle.
	cmd := exec.Command(gazelleAbsPath, "--frontend_unit_test")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	// Assert that Gazelle generated the expected files.
	testtools.CheckFiles(t, dir, expectedOutputFiles)
}
