package frontend

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
	}, makeBasicWorkspace()...)

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
	}, makeBasicWorkspace()...)

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
	}, makeBasicWorkspace()...)

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
	gazelleAbsPath := filepath.Join(bazel.RunfilesDir(), "bazel/gazelle/frontend/gazelle_frontend_test_binary_/gazelle_frontend_test_binary")

	// Write the input files to a temporary directory.
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
