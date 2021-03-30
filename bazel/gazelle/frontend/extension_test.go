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

func TestGazelle_NewSourceFilesAdded_GeneratesBuildRules(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := append([]testtools.FileSpec{
		{
			Path: "a/alfa.ts",
			Content: `
import './bravo';        // Resolves to a/bravo.ts.
import './b/charlie';    // Resolves to a/b/charlie.ts.
import '../c';           // Resolves to c/index.ts.
import '../c/delta';     // Resolves to c/delta.ts.
import '../d_ts_lib/d';  // Resolves to d_ts_lib/d.ts.
import 'lit-html';       // NPM import with built-in TypeScript annotations.
import 'puppeteer';      // NPM import with a separate @types/puppeteer package.
import 'net'             // Built-in Node.js module.
`,
		},
		{Path: "a/bravo.ts"},
		{Path: "a/b/charlie.ts"},
		{Path: "c/delta.ts"},
		// Empty file which may be imported as its parent folder's "main" module.
		{Path: "c/index.ts"},
		// Will produce a ts_library with the same name as its parent folder ("d_ts_lib").
		{Path: "d_ts_lib/d.ts"},
	}, makeBasicWorkspace()...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "alfa_ts_lib",
    srcs = ["alfa.ts"],
    visibility = ["//visibility:public"],
    deps = [
        ":bravo_ts_lib",
        "//a/b:charlie_ts_lib",
        "//c:delta_ts_lib",
        "//c:index_ts_lib",
        "//d_ts_lib",
        "@infra-sk_npm//@types/puppeteer",
        "@infra-sk_npm//lit-html",
        "@infra-sk_npm//puppeteer",
    ],
)

ts_library(
    name = "bravo_ts_lib",
    srcs = ["bravo.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "a/b/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "charlie_ts_lib",
    srcs = ["charlie.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "c/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "delta_ts_lib",
    srcs = ["delta.ts"],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "index_ts_lib",
    srcs = ["index.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "d_ts_lib/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "d_ts_lib",
    srcs = ["d.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

func TestGazelle_ImportsInSourceFilesChanged_UpdatesBuildRules(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := append([]testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "alfa_ts_lib",
    srcs = ["alfa.ts"],
    visibility = ["//visibility:public"],
    deps = [
        "@infra-sk_npm//common-sk",    # Not imported from alfa.ts. Gazelle should remove this dep.
        "@infra-sk_npm//elements-sk",
    ],
)
`,
		},
		{
			Path: "a/alfa.ts",
			Content: `
import 'elements-sk';  // Existing import.
import 'lit-html';     // New import. Gazelle should add this dep.
`,
		},
	}, makeBasicWorkspace()...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "alfa_ts_lib",
    srcs = ["alfa.ts"],
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

func TestGazelle_SomeSourceFilesRemoved_UpdatesOrDeletesBuildRules(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := append([]testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "alfa_ts_lib",
    srcs = [
        "alfa.ts",
        "bravo.ts",  # This file was deleted. Gazelle should remove this src.
    ],
    visibility = ["//visibility:public"],
)

# This target will be deleted because its source files no longer exist.
ts_library(
    name = "bravo_ts_lib",
    srcs = ["bravo.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{Path: "a/alfa.ts"},
	}, makeBasicWorkspace()...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "alfa_ts_lib",
    srcs = ["alfa.ts"],
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
