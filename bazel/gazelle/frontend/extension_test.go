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
			Path: "package.json",
			Content: `
{
  "dependencies": {
    "@google-web-components/google-chart": "^4.0.2",
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
		// Various files which are not part of an sk_element or an sk_page.
		{
			Path: "a/alfa.scss",
			Content: `
@import 'bravo';                                    // Resolves to a/bravo.scss.
@import 'b/charlie';                                // Resolves to a/b/charlie.scss.
@import '../c/delta';                               // Resolves to c/delta.scss.
@import '../d_sass_lib/d';                          // Resolves to d_sass_lib/d.scss.
@use 'node_modules/codemirror5/lib/codemirror.css'; // Resolves to @npm//:node_modules/codemirror5/lib/codemirror.css.
`,
		},
		{
			Path: "a/alfa.ts",
			Content: `
import './bravo';                             // Resolves to a/bravo.ts.s
import './b/charlie';                         // Resolves to a/b/charlie.ts.
import '../c';                                // Resolves to c/index.ts.
import '../c/delta';                          // Resolves to c/delta.ts.
import '../d_ts_lib/d';                       // Resolves to d_ts_lib/d.ts.
import 'lit-html';                            // NPM import with built-in TypeScript annotations.
import 'puppeteer';                           // NPM import with a separate @types/puppeteer package.
import '@google-web-components/google-chart'; // Scoped NPM import.
import 'net'                                  // Built-in Node.js module.
`,
		},
		{Path: "a/alfa.html"}, // Ignored because this is neither an app page nor a demo page.
		{
			Path: "a/alfa_test.ts",
			Content: `
import './alfa';

// The below imports are copied from alfa.ts.
import './bravo';                             // Resolves to a/bravo.ts.
import './b/charlie';                         // Resolves to a/b/charlie.ts.
import '../c';                                // Resolves to c/index.ts.
import '../c/delta';                          // Resolves to c/delta.ts.
import '../d_ts_lib/d';                       // Resolves to d_ts_lib/d.ts.
import 'lit-html';                            // NPM import with built-in TypeScript annotations.
import 'puppeteer';                           // NPM import with a separate @types/puppeteer package.
import '@google-web-components/google-chart'; // Scoped NPM import.
import 'net'                                  // Built-in Node.js module.
`,
		},
		{
			Path: "a/alfa_nodejs_test.ts",
			Content: `
import './alfa';

// The below imports are copied from alfa.ts.
import './bravo';                             // Resolves to a/bravo.ts.
import './b/charlie';                         // Resolves to a/b/charlie.ts.
import '../c';                                // Resolves to c/index.ts.
import '../c/delta';                          // Resolves to c/delta.ts.
import '../d_ts_lib/d';                       // Resolves to d_ts_lib/d.ts.
import 'lit-html';                            // NPM import with built-in TypeScript annotations.
import 'puppeteer';                           // NPM import with a separate @types/puppeteer package.
import '@google-web-components/google-chart'; // Scoped NPM import.
import 'net'                                  // Built-in Node.js module.
`,
		},
		{Path: "a/bravo.scss"},
		{Path: "a/bravo.ts"},
		{Path: "a/b/charlie.scss"},
		{Path: "a/b/charlie.ts"},
		{Path: "c/delta.scss"},
		{Path: "c/delta.ts"},
		// Empty file which may be imported as its parent folder's "main" module.
		{Path: "c/index.ts"},
		// Will produce a sass_library with the same as its parent folder ("d_sass_lib").
		{Path: "d_sass_lib/d.scss"},
		// Will produce a ts_library with the same name as its parent folder ("d_ts_lib").
		{Path: "d_ts_lib/d.ts"},

		// These files look like they might belong to an sk_element, but do not, because their parent
		// directory does not follow the "<app>/modules/<element-name-sk>" pattern.
		{Path: "echo-sk/echo-sk.scss"},
		{Path: "echo-sk/echo-sk.ts"},
		{Path: "echo-sk/index.ts"},

		// An sk_element with a demo page.
		{
			Path:    "myapp/modules/foxtrot-sk/index.ts",
			Content: `import './foxtrot-sk';  // Resolves to myapp/modules/foxtrot-sk/foxtrot-sk.ts`,
		},
		{
			Path: "myapp/modules/foxtrot-sk/foxtrot-sk.scss",
			Content: `
@import 'wibble';                                   // Resolves to myapp/modules/foxtrot-sk/wibble.scss.
@import 'wobble/wubble';                            // Resolves to myapp/modules/foxtrot-sk/wobble/wubble.scss.
@import '../golf-sk/golf-sk';                       // Resolves to myapp/modules/golf-sk/golf-sk.scss.
@import '../../../d_sass_lib/d';                    // Resolves to d_sass_lib/d.scss.
@use 'node_modules/codemirror5/lib/codemirror.css'; // Resolves to @npm//:node_modules/codemirror5/lib/codemirror.css.

`,
		},
		{
			Path: "myapp/modules/foxtrot-sk/foxtrot-sk.ts",
			Content: `
import './wibble';                            // Resolves to myapp/modules/foxtrot-sk/wibble.ts.
import './wobble/wubble';                     // Resolves to myapp/modules/foxtrot-sk/wobble/wubble.ts.
import '../hotel-sk/hotel-sk';                // Resolves to myapp/modules/hotel-sk/hotel-sk.ts.
import '../../../c';                          // Resolves to c/index.ts.
import '../../../d_ts_lib/d';                 // Resolves to d_ts_lib/d.ts.
import 'lit-html';                            // NPM import with built-in TypeScript annotations.
import 'puppeteer';                           // NPM import with a separate @types/puppeteer package.
import '@google-web-components/google-chart'; // Scoped NPM import.
import 'net'                                  // Built-in Node.js module.
`,
		},
		{Path: "myapp/modules/foxtrot-sk/foxtrot-sk-demo.html"},
		{
			Path: "myapp/modules/foxtrot-sk/foxtrot-sk-demo.scss",
			Content: `
@import 'foxtrot-sk';  // Resolves to myapp/modules/foxtrot-sk/foxtrot-sk.scss.

// The below imports are copied from foxtrot-sk.scss.
@import 'wibble';                                   // Resolves to myapp/modules/foxtrot-sk/wibble.scss.
@import 'wobble/wubble';                            // Resolves to myapp/modules/foxtrot-sk/wobble/wubble.scss.
@import '../golf-sk/golf-sk';                       // Resolves to myapp/modules/golf-sk/golf-sk.scss.
@import '../../../d_sass_lib/d';                    // Resolves to d_sass_lib/d.scss.
@use 'node_modules/codemirror5/lib/codemirror.css'; // Resolves to @npm//:node_modules/codemirror5/lib/codemirror.css.

`,
		},
		{
			Path: "myapp/modules/foxtrot-sk/foxtrot-sk-demo.ts",
			Content: `
import './foxtrot-sk';  // Resolves to myapp/modules/foxtrot-sk/foxtrot-sk.ts.

// The below imports are copied from foxtrot-sk.ts.
import './wibble';                            // Resolves to myapp/modules/foxtrot-sk/wibble.ts.
import './wobble/wubble';                     // Resolves to myapp/modules/foxtrot-sk/wobble/wubble.ts.
import '../hotel-sk/hotel-sk';                // Resolves to myapp/modules/hotel-sk/hotel-sk.ts.
import '../../../c';                          // Resolves to c/index.ts.
import '../../../d_ts_lib/d';                 // Resolves to d_ts_lib/d.ts.
import 'lit-html';                            // NPM import with built-in TypeScript annotations.
import 'puppeteer';                           // NPM import with a separate @types/puppeteer package.
import '@google-web-components/google-chart'; // Scoped NPM import.
import 'net'                                  // Built-in Node.js module.
`,
		},
		{
			Path: "myapp/modules/foxtrot-sk/foxtrot-sk_puppeteer_test.ts",
			Content: `
import './foxtrot-sk';  // Resolves to myapp/modules/foxtrot-sk/foxtrot-sk.ts.

// The below imports are copied from foxtrot-sk.ts.
import './wibble';                            // Resolves to myapp/modules/foxtrot-sk/wibble.ts.
import './wobble/wubble';                     // Resolves to myapp/modules/foxtrot-sk/wobble/wubble.ts.
import '../hotel-sk/hotel-sk';                // Resolves to myapp/modules/hotel-sk/hotel-sk.ts.
import '../../../c';                          // Resolves to c/index.ts.
import '../../../d_ts_lib/d';                 // Resolves to d_ts_lib/d.ts.
import 'lit-html';                            // NPM import with built-in TypeScript annotations.
import 'puppeteer';                           // NPM import with a separate @types/puppeteer package.
import '@google-web-components/google-chart'; // Scoped NPM import.
import 'net'                                  // Built-in Node.js module.
`,
		},
		{
			Path: "myapp/modules/foxtrot-sk/foxtrot-sk_test.ts",
			Content: `
import './foxtrot-sk';  // Resolves to myapp/modules/foxtrot-sk/foxtrot-sk.ts.

// The below imports are copied from foxtrot-sk.ts.
import './wibble';                            // Resolves to myapp/modules/foxtrot-sk/wibble.ts.
import './wobble/wubble';                     // Resolves to myapp/modules/foxtrot-sk/wobble/wubble.ts.
import '../hotel-sk/hotel-sk';                // Resolves to myapp/modules/hotel-sk/hotel-sk.ts.
import '../../../c';                          // Resolves to c/index.ts.
import '../../../d_ts_lib/d';                 // Resolves to d_ts_lib/d.ts.
import 'lit-html';                            // NPM import with built-in TypeScript annotations.
import 'puppeteer';                           // NPM import with a separate @types/puppeteer package.
import '@google-web-components/google-chart'; // Scoped NPM import.
import 'net'                                  // Built-in Node.js module.
`,
		},
		{Path: "myapp/modules/foxtrot-sk/wibble.scss"},
		{Path: "myapp/modules/foxtrot-sk/wibble.ts"},
		{Path: "myapp/modules/foxtrot-sk/wobble/wubble.scss"},
		{Path: "myapp/modules/foxtrot-sk/wobble/wubble.ts"},

		// This sk_element does not have an index.ts file.
		{Path: "myapp/modules/golf-sk/golf-sk.scss"},
		{
			Path: "myapp/modules/golf-sk/golf-sk.ts",
			Content: `
import '../hotel-sk'; // Resolves to myapp/modules/hotel-sk/index.ts.
`,
		},
		{Path: "myapp/modules/golf-sk/golf-sk-demo.html"},
		{Path: "myapp/modules/golf-sk/golf-sk-demo.scss"},
		{Path: "myapp/modules/golf-sk/golf-sk-demo.ts"},

		// This sk_element does not have a Sass stylesheet.
		{Path: "myapp/modules/hotel-sk/hotel-sk.ts"},
		{Path: "myapp/modules/hotel-sk/index.ts"},

		// These files look like they comprise a demo page for hotel-sk, but do not, because they do not
		// follow the "hotel-sk-demo.{html,scss,ts}" pattern.
		{Path: "myapp/modules/hotel-sk/hello-world-demo.html"},
		{Path: "myapp/modules/hotel-sk/hello-world-demo.scss"},
		{Path: "myapp/modules/hotel-sk/hello-world-demo.ts"},

		// This sk_element has neither an index.ts file nor a Sass stylesheet.
		{Path: "myapp/modules/india-sk/india-sk.ts"},

		// An app page.
		{Path: "myapp/pages/juliett.html"},
		{Path: "myapp/pages/juliett.scss"},
		{Path: "myapp/pages/juliett.ts"},

		// An app page.
		{Path: "myapp/pages/kilo.html"},
		{Path: "myapp/pages/kilo.scss"},
		{Path: "myapp/pages/kilo.ts"},

		// Extra files in the app page directory.
		{Path: "myapp/pages/wibble.scss"},
		{Path: "myapp/pages/wibble.ts"},
		{Path: "myapp/pages/wobble/wubble.html"}, // Ignored because this is not an app or demo page.
		{Path: "myapp/pages/wobble/wubble.ts"},
		{Path: "myapp/pages/wobble/wubble.scss"},
	}, makeBasicWorkspace()...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "karma_test", "nodejs_test", "sass_library", "ts_library")

nodejs_test(
    name = "alfa_nodejs_test",
    src = "alfa_nodejs_test.ts",
    deps = [
        ":alfa_ts_lib",
        ":bravo_ts_lib",
        "//a/b:charlie_ts_lib",
        "//c:delta_ts_lib",
        "//c:index_ts_lib",
        "//d_ts_lib",
        "@npm//@google-web-components/google-chart",
        "@npm//@types/puppeteer",
        "@npm//lit-html",
        "@npm//puppeteer",
    ],
)

sass_library(
    name = "alfa_sass_lib",
    srcs = ["alfa.scss"],
    visibility = ["//visibility:public"],
    deps = [
        ":bravo_sass_lib",
        "//a/b:charlie_sass_lib",
        "//c:delta_sass_lib",
        "//d_sass_lib",
        "@npm//:node_modules/codemirror5/lib/codemirror.css",
    ],
)

karma_test(
    name = "alfa_test",
    src = "alfa_test.ts",
    deps = [
        ":alfa_ts_lib",
        ":bravo_ts_lib",
        "//a/b:charlie_ts_lib",
        "//c:delta_ts_lib",
        "//c:index_ts_lib",
        "//d_ts_lib",
        "@npm//@google-web-components/google-chart",
        "@npm//@types/puppeteer",
        "@npm//lit-html",
        "@npm//puppeteer",
    ],
)

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
        "@npm//@google-web-components/google-chart",
        "@npm//@types/puppeteer",
        "@npm//lit-html",
        "@npm//puppeteer",
    ],
)

sass_library(
    name = "bravo_sass_lib",
    srcs = ["bravo.scss"],
    visibility = ["//visibility:public"],
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
load("//infra-sk:index.bzl", "sass_library", "ts_library")

sass_library(
    name = "charlie_sass_lib",
    srcs = ["charlie.scss"],
    visibility = ["//visibility:public"],
)

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
load("//infra-sk:index.bzl", "sass_library", "ts_library")

sass_library(
    name = "delta_sass_lib",
    srcs = ["delta.scss"],
    visibility = ["//visibility:public"],
)

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
			Path: "d_sass_lib/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library")

sass_library(
    name = "d_sass_lib",
    srcs = ["d.scss"],
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
		{
			Path: "echo-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library", "ts_library")

sass_library(
    name = "echo-sk_sass_lib",
    srcs = ["echo-sk.scss"],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "echo-sk_ts_lib",
    srcs = ["echo-sk.ts"],
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
			Path: "myapp/modules/foxtrot-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "karma_test", "sass_library", "sk_demo_page_server", "sk_element", "sk_element_puppeteer_test", "sk_page", "ts_library")

sk_demo_page_server(
    name = "demo_page_server",
    sk_page = ":foxtrot-sk-demo",
)

sk_element(
    name = "foxtrot-sk",
    sass_deps = [
        "//d_sass_lib",
        "//myapp/modules/foxtrot-sk/wobble:wubble_sass_lib",
        ":wibble_sass_lib",
        "@npm//:node_modules/codemirror5/lib/codemirror.css",
    ],
    sass_srcs = ["foxtrot-sk.scss"],
    sk_element_deps = [
        "//myapp/modules/golf-sk",
        "//myapp/modules/hotel-sk",
    ],
    ts_deps = [
        "//c:index_ts_lib",
        "//d_ts_lib",
        "//myapp/modules/foxtrot-sk/wobble:wubble_ts_lib",
        ":wibble_ts_lib",
        "@npm//@google-web-components/google-chart",
        "@npm//@types/puppeteer",
        "@npm//lit-html",
        "@npm//puppeteer",
    ],
    ts_srcs = [
        "foxtrot-sk.ts",
        "index.ts",
    ],
    visibility = ["//visibility:public"],
)

sk_page(
    name = "foxtrot-sk-demo",
    html_file = "foxtrot-sk-demo.html",
    sass_deps = [
        "//d_sass_lib",
        "//myapp/modules/foxtrot-sk/wobble:wubble_sass_lib",
        ":wibble_sass_lib",
        "@npm//:node_modules/codemirror5/lib/codemirror.css",
    ],
    scss_entry_point = "foxtrot-sk-demo.scss",
    sk_element_deps = [
        "//myapp/modules/golf-sk",
        "//myapp/modules/hotel-sk",
        ":foxtrot-sk",
    ],
    ts_deps = [
        "//c:index_ts_lib",
        "//d_ts_lib",
        "//myapp/modules/foxtrot-sk/wobble:wubble_ts_lib",
        ":wibble_ts_lib",
        "@npm//@google-web-components/google-chart",
        "@npm//@types/puppeteer",
        "@npm//lit-html",
        "@npm//puppeteer",
    ],
    ts_entry_point = "foxtrot-sk-demo.ts",
)

sk_element_puppeteer_test(
    name = "foxtrot-sk_puppeteer_test",
    src = "foxtrot-sk_puppeteer_test.ts",
    sk_demo_page_server = ":demo_page_server",
    deps = [
        ":foxtrot-sk",
        ":wibble_ts_lib",
        "//c:index_ts_lib",
        "//d_ts_lib",
        "//myapp/modules/foxtrot-sk/wobble:wubble_ts_lib",
        "//myapp/modules/hotel-sk",
        "@npm//@google-web-components/google-chart",
        "@npm//@types/puppeteer",
        "@npm//lit-html",
        "@npm//puppeteer",
    ],
)

karma_test(
    name = "foxtrot-sk_test",
    src = "foxtrot-sk_test.ts",
    deps = [
        ":foxtrot-sk",
        ":wibble_ts_lib",
        "//c:index_ts_lib",
        "//d_ts_lib",
        "//myapp/modules/foxtrot-sk/wobble:wubble_ts_lib",
        "//myapp/modules/hotel-sk",
        "@npm//@google-web-components/google-chart",
        "@npm//@types/puppeteer",
        "@npm//lit-html",
        "@npm//puppeteer",
    ],
)

sass_library(
    name = "wibble_sass_lib",
    srcs = ["wibble.scss"],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "wibble_ts_lib",
    srcs = ["wibble.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "myapp/modules/foxtrot-sk/wobble/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library", "ts_library")

sass_library(
    name = "wubble_sass_lib",
    srcs = ["wubble.scss"],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "wubble_ts_lib",
    srcs = ["wubble.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "myapp/modules/golf-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sk_demo_page_server", "sk_element", "sk_page")

sk_demo_page_server(
    name = "demo_page_server",
    sk_page = ":golf-sk-demo",
)

sk_element(
    name = "golf-sk",
    sass_srcs = ["golf-sk.scss"],
    sk_element_deps = ["//myapp/modules/hotel-sk"],
    ts_srcs = ["golf-sk.ts"],
    visibility = ["//visibility:public"],
)

sk_page(
    name = "golf-sk-demo",
    html_file = "golf-sk-demo.html",
    scss_entry_point = "golf-sk-demo.scss",
    ts_entry_point = "golf-sk-demo.ts",
)
`,
		},
		{
			Path: "myapp/modules/hotel-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library", "sk_element", "ts_library")

sass_library(
    name = "hello-world-demo_sass_lib",
    srcs = ["hello-world-demo.scss"],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "hello-world-demo_ts_lib",
    srcs = ["hello-world-demo.ts"],
    visibility = ["//visibility:public"],
)

sk_element(
    name = "hotel-sk",
    ts_srcs = [
        "hotel-sk.ts",
        "index.ts",
    ],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "myapp/modules/india-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sk_element")

sk_element(
    name = "india-sk",
    ts_srcs = ["india-sk.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "myapp/pages/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library", "sk_page", "ts_library")

sk_page(
    name = "juliett",
    html_file = "juliett.html",
    scss_entry_point = "juliett.scss",
    ts_entry_point = "juliett.ts",
)

sk_page(
    name = "kilo",
    html_file = "kilo.html",
    scss_entry_point = "kilo.scss",
    ts_entry_point = "kilo.ts",
)

sass_library(
    name = "wibble_sass_lib",
    srcs = ["wibble.scss"],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "wibble_ts_lib",
    srcs = ["wibble.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "myapp/pages/wobble/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library", "ts_library")

sass_library(
    name = "wubble_sass_lib",
    srcs = ["wubble.scss"],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "wubble_ts_lib",
    srcs = ["wubble.ts"],
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
		// Various files which are not part of an sk_element or an sk_page.
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "karma_test", "nodejs_test", "sass_library", "ts_library")

nodejs_test(
    name = "alfa_nodejs_test",
    src = "alfa_nodejs_test.ts",
    deps = [
        ":alfa_ts_lib",
        # Not imported from alfa_nodejs_test.ts. Gazelle should remove this dep.
        "@npm//common-sk",
        "@npm//elements-sk",
    ],
)

sass_library(
    name = "alfa_sass_lib",
    srcs = ["alfa.scss"],
    visibility = ["//visibility:public"],
    deps = [
        ":bravo_sass_lib",  # Not imported from alfa.scss. Gazelle should remove this dep.
        ":charlie_sass_lib",
        # Not imported from alfa.scss. Gazelle should remove this dep.
        "@npm//:node_modules/codemirror5/lib/codemirror.css",
    ],
)

karma_test(
    name = "alfa_test",
    src = "alfa_test.ts",
    deps = [
        ":alfa_ts_lib",
        "@npm//common-sk",  # Not imported from alfa.ts. Gazelle should remove this dep.
        "@npm//elements-sk",
    ],
)

ts_library(
    name = "alfa_ts_lib",
    srcs = ["alfa.ts"],
    visibility = ["//visibility:public"],
    deps = [
        "@npm//common-sk",  # Not imported from alfa.ts. Gazelle should remove this dep.
        "@npm//elements-sk",
    ],
)

sass_library(
    name = "bravo_sass_lib",
    srcs = ["bravo.scss"],
    visibility = ["//visibility:public"],
)

sass_library(
    name = "charlie_sass_lib",
    srcs = ["charlie.scss"],
    visibility = ["//visibility:public"],
)

sass_library(
    name = "delta_sass_lib",
    srcs = ["delta.scss"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "a/alfa.scss",
			Content: `
@import 'charlie';                                  // Existing import.
@import 'delta';                                    // New import. Gazelle should add this dep.
@use 'node_modules/codemirror5/theme/ambiance.css'; // New import. Gazelle should add this dep.
`,
		},
		{
			Path: "a/alfa.ts",
			Content: `
import 'elements-sk/checkbox-sk';  // Existing import.
import 'lit-html';                 // New import. Gazelle should add this dep.
`,
		},
		{
			Path: "a/alfa_nodejs_test.ts",
			Content: `
import './alfa';                   // Existing import.
import 'elements-sk/checkbox-sk';  // Existing import.
import 'lit-html';                 // New import. Gazelle should add this dep.
`,
		},
		{
			Path: "a/alfa_test.ts",
			Content: `
import './alfa';                   // Existing import.
import 'elements-sk/checkbox-sk';  // Existing import.
import 'lit-html';                 // New import. Gazelle should add this dep.
`,
		},
		{Path: "a/bravo.scss"},
		{Path: "a/charlie.scss"},
		{Path: "a/delta.scss"},

		// An sk_element.
		{
			Path: "myapp/modules/echo-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "karma_test", "sk_demo_page_server", "sk_element", "sk_element_puppeteer_test", "sk_page")

sk_element(
    name = "echo-sk",
    sass_deps = [
        "//a:alfa_sass_lib",  # Not imported from echo-sk.scss. Gazelle should remove this dep.
        "//a:bravo_sass_lib",
        # Not imported from echo-sk.scss. Gazelle should remove this dep.
        "@npm//:node_modules/codemirror5/lib/codemirror.css",

    ],
    sass_srcs = ["echo-sk.scss"],
    sk_element_deps = [
        # Not imported from echo-sk.{scss,ts}. Gazelle should remove this dep.
        "//myapp/modules/foxtrot-sk",
        "//myapp/modules/golf-sk",
    ],
    ts_deps = [
        # Not imported from echo-sk.ts. Gazelle should remove this dep.
        "@npm//common-sk",
        "@npm//lit-html",
    ],
    ts_srcs = ["echo-sk.ts"],
    visibility = ["//visibility:public"],
)

sk_page(
    name = "echo-sk-demo",
    html_file = "echo-sk-demo.html",
    sass_deps = [
        "//a:alfa_sass_lib",  # Not imported from echo-sk-demo.scss. Gazelle should remove this dep.
        "//a:bravo_sass_lib",
        # Not imported from echo-sk-demo.scss. Gazelle should remove this dep.
        "@npm//:node_modules/codemirror5/lib/codemirror.css",
    ],
    scss_entry_point = "echo-sk-demo.scss",
    sk_element_deps = [
        ":echo-sk",
        # Not imported from echo-sk-demo.{scss,ts}. Gazelle should remove this dep.
        "//myapp/modules/foxtrot-sk",
        "//myapp/modules/golf-sk",
    ],
    ts_deps = [
        "@npm//common-sk",
        # Not imported from echo-sk-demo.ts. Gazelle should remove this dep.
        "@npm//elements-sk",
    ],
    ts_entry_point = "echo-sk-demo.ts",
)

sk_element_puppeteer_test(
    name = "echo-sk_puppeteer_test",
    src = "echo-sk_puppeteer_test.ts",
    sk_demo_page_server = ":demo_page_server",
    deps = [
        ":echo-sk",
        # Not imported from echo-sk_puppeteer_test.ts. Gazelle should remove this dep.
        "//myapp/modules/foxtrot-sk",
        "//myapp/modules/golf-sk",
        # Not imported from echo-sk_puppeteer_test.ts. Gazelle should remove this dep.
        "@npm//common-sk",
        "@npm//elements-sk",
    ],
)

karma_test(
    name = "echo-sk_test",
    src = "echo-sk_test.ts",
    deps = [
        ":echo-sk",
        # Not imported from echo-sk_test.ts. Gazelle should remove this dep.
        "//myapp/modules/foxtrot-sk",
        "//myapp/modules/golf-sk",
        # Not imported from echo-sk_test.ts. Gazelle should remove this dep.
        "@npm//common-sk",
        "@npm//elements-sk",
    ],
)

sk_demo_page_server(
    name = "demo_page_server",
    sk_page = ":echo-sk-demo",
)
`,
		},
		{
			Path: "myapp/modules/echo-sk/echo-sk.scss",
			Content: `
@import '../../../a/bravo.scss';                    // Existing import.
@import '../../../a/charlie.scss';                  // New import. Gazelle should add this dep.
@use 'node_modules/codemirror5/theme/ambiance.css'; // New import. Gazelle should add this dep.
`,
		},
		{
			Path: "myapp/modules/echo-sk/echo-sk.ts",
			Content: `
import '../golf-sk/golf-sk';       // Existing import.
import '../hotel-sk/hotel-sk';     // New import. Gazelle should add this dep.
import 'elements-sk/checkbox-sk';  // New import. Gazelle should add this dep.
import 'lit-html';                 // Existing import.
`,
		},
		{Path: "myapp/modules/echo-sk/index.ts"}, // This new file should be added to ts_srcs.
		{Path: "myapp/modules/echo-sk/echo-sk-demo.html"},
		{
			Path: "myapp/modules/echo-sk/echo-sk-demo.scss",
			Content: `
@import 'echo-sk.scss';                             // Existing import.
@import '../../../a/bravo.scss';                    // Existing import.
@import '../../../a/charlie.scss';                  // New import. Gazelle should add this dep.
@use 'node_modules/codemirror5/theme/ambiance.css'; // New import. Gazelle should add this dep.

`,
		},
		{
			Path: "myapp/modules/echo-sk/echo-sk-demo.ts",
			Content: `
import './echo-sk';                // Existing import.
import '../golf-sk/golf-sk';       // Existing import.
import '../hotel-sk/hotel-sk';     // New import. Gazelle should add this dep.
import 'common-sk';                // Existing import.
import 'lit-html';                 // New import. Gazelle should add this dep.
`,
		},
		{
			Path: "myapp/modules/echo-sk/echo-sk_puppeteer_test.ts",
			Content: `
import './echo-sk';                // Existing import.
import '../golf-sk/golf-sk';       // Existing import.
import '../hotel-sk/hotel-sk';     // New import. Gazelle should add this dep.
import 'elements-sk/checkbox-sk';  // Existing import.
import 'lit-html';                 // New import. Gazelle should add this dep.
`,
		},
		{
			Path: "myapp/modules/echo-sk/echo-sk_test.ts",
			Content: `
import './echo-sk';                // Existing import.
import '../golf-sk/golf-sk';       // Existing import.
import '../hotel-sk/hotel-sk';     // New import. Gazelle should add this dep.
import 'elements-sk/checkbox-sk';  // Existing import.
import 'lit-html';                 // New import. Gazelle should add this dep.
`,
		},

		// An sk_element.
		{
			Path: "myapp/modules/foxtrot-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sk_element")

sk_element(
    name = "foxtrot-sk",
    sass_srcs = ["foxtrot-sk.scss"],
    ts_srcs = ["foxtrot-sk.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{Path: "myapp/modules/foxtrot-sk/foxtrot-sk.scss"},
		{Path: "myapp/modules/foxtrot-sk/foxtrot-sk.ts"},

		// An sk_element.
		{
			Path: "myapp/modules/golf-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sk_element")

sk_element(
    name = "golf-sk",
    sass_srcs = ["golf-sk.scss"],
    ts_srcs = ["golf-sk.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{Path: "myapp/modules/golf-sk/golf-sk.scss"},
		{Path: "myapp/modules/golf-sk/golf-sk.ts"},

		// An sk_element.
		{
			Path: "myapp/modules/hotel-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sk_element")

sk_element(
    name = "hotel-sk",
    sass_srcs = ["hotel-sk.scss"],
    ts_srcs = ["hotel-sk.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{Path: "myapp/modules/hotel-sk/hotel-sk.scss"},
		{Path: "myapp/modules/hotel-sk/hotel-sk.ts"},
	}, makeBasicWorkspace()...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "karma_test", "nodejs_test", "sass_library", "ts_library")

nodejs_test(
    name = "alfa_nodejs_test",
    src = "alfa_nodejs_test.ts",
    deps = [
        ":alfa_ts_lib",
        "@npm//elements-sk",
        "@npm//lit-html",
    ],
)

sass_library(
    name = "alfa_sass_lib",
    srcs = ["alfa.scss"],
    visibility = ["//visibility:public"],
    deps = [
        ":charlie_sass_lib",
        ":delta_sass_lib",
        "@npm//:node_modules/codemirror5/theme/ambiance.css",
    ],
)

karma_test(
    name = "alfa_test",
    src = "alfa_test.ts",
    deps = [
        ":alfa_ts_lib",
        "@npm//elements-sk",
        "@npm//lit-html",
    ],
)

ts_library(
    name = "alfa_ts_lib",
    srcs = ["alfa.ts"],
    visibility = ["//visibility:public"],
    deps = [
        "@npm//elements-sk",
        "@npm//lit-html",
    ],
)

sass_library(
    name = "bravo_sass_lib",
    srcs = ["bravo.scss"],
    visibility = ["//visibility:public"],
)

sass_library(
    name = "charlie_sass_lib",
    srcs = ["charlie.scss"],
    visibility = ["//visibility:public"],
)

sass_library(
    name = "delta_sass_lib",
    srcs = ["delta.scss"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "myapp/modules/echo-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "karma_test", "sk_demo_page_server", "sk_element", "sk_element_puppeteer_test", "sk_page")

sk_element(
    name = "echo-sk",
    sass_deps = [
        "//a:bravo_sass_lib",
        "//a:charlie_sass_lib",
        "@npm//:node_modules/codemirror5/theme/ambiance.css",
    ],
    sass_srcs = ["echo-sk.scss"],
    sk_element_deps = [
        "//myapp/modules/golf-sk",
        "//myapp/modules/hotel-sk",
    ],
    ts_deps = [
        "@npm//lit-html",
        "@npm//elements-sk",
    ],
    ts_srcs = [
        "echo-sk.ts",
        "index.ts",
    ],
    visibility = ["//visibility:public"],
)

sk_page(
    name = "echo-sk-demo",
    html_file = "echo-sk-demo.html",
    sass_deps = [
        "//a:bravo_sass_lib",
        "//a:charlie_sass_lib",
        "@npm//:node_modules/codemirror5/theme/ambiance.css",
    ],
    scss_entry_point = "echo-sk-demo.scss",
    sk_element_deps = [
        ":echo-sk",
        "//myapp/modules/golf-sk",
        "//myapp/modules/hotel-sk",
    ],
    ts_deps = [
        "@npm//common-sk",
        "@npm//lit-html",
    ],
    ts_entry_point = "echo-sk-demo.ts",
)

sk_element_puppeteer_test(
    name = "echo-sk_puppeteer_test",
    src = "echo-sk_puppeteer_test.ts",
    sk_demo_page_server = ":demo_page_server",
    deps = [
        ":echo-sk",
        "//myapp/modules/golf-sk",
        "//myapp/modules/hotel-sk",
        "@npm//elements-sk",
        "@npm//lit-html",
    ],
)

karma_test(
    name = "echo-sk_test",
    src = "echo-sk_test.ts",
    deps = [
        ":echo-sk",
        "//myapp/modules/golf-sk",
        "//myapp/modules/hotel-sk",
        "@npm//elements-sk",
        "@npm//lit-html",
    ],
)

sk_demo_page_server(
    name = "demo_page_server",
    sk_page = ":echo-sk-demo",
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
load("//infra-sk:index.bzl", "karma_test", "nodejs_test", "sass_library", "ts_library")

# This target will be deleted because file alfa_nodejs_test.ts no longer exists.
nodejs_test(
    name = "alfa_nodejs_test",
    src = "alfa_nodejs_test.ts",
)

sass_library(
    name = "alfa_sass_lib",
    srcs = [
        "alfa.scss",
        "bravo.scss",  # This file was deleted. Gazelle should remove this dep.
    ],
    visibility = ["//visibility:public"],
)

# This target will be deleted because file alfa_test.ts no longer exists.
karma_test(
    name = "alfa_test",
    src = "alfa_test.ts",
)

ts_library(
    name = "alfa_ts_lib",
    srcs = [
        "alfa.ts",
        "bravo.ts",  # This file was deleted. Gazelle should remove this src.
    ],
    visibility = ["//visibility:public"],
)

# This target will be deleted because its source files no longer exist.
sass_library(
    name = "bravo_sass_lib",
    srcs = ["bravo.scss"],
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
		{Path: "a/alfa.scss"},
		{Path: "a/alfa.ts"},
		{
			Path: "myapp/modules/charlie-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "karma_test", "sk_demo_page_server", "sk_element", "sk_element_puppeteer_test", "sk_page")

sk_element(
    name = "charlie-sk",
    sass_srcs = [
        "charlie-sk.scss", # This file does not exist anymore. Gazelle should remove this entry.
    ],
    ts_srcs = [
        "charlie-sk.ts",
        "index.ts",  # This file does not exist anymore. Gazelle should remove this entry.
    ],
    visibility = ["//visibility:public"],
)

sk_page(
    name = "charlie-sk-demo",
    html_file = "charlie-sk-demo.html",
    # This file does not exist anymore. Gazelle should remove the scss_entry_point argument.
    scss_entry_point = "charlie-sk-demo.scss",
    ts_entry_point = "charlie-sk-demo.ts",
)

# This target will be deleted because file charlie-sk_puppeteer_test.ts no longer exists.
sk_element_puppeteer_test(
    name = "charlie-sk_puppeteer_test",
    src = "charlie-sk_puppeteer_test.ts",
    sk_demo_page_server = ":demo_page_server",
)

# This target will be deleted because file charlie-sk_test.ts no longer exists.
karma_test(
    name = "charlie-sk_test",
    src = "charlie-sk_test.ts",
)

sk_demo_page_server(
    name = "demo_page_server",
    sk_page = ":charlie-sk-demo",
)
`,
		},
		{Path: "myapp/modules/charlie-sk/charlie-sk.ts"},
		{Path: "myapp/modules/charlie-sk/charlie-sk-demo.html"},
		{Path: "myapp/modules/charlie-sk/charlie-sk-demo.ts"},
		{
			Path: "myapp/modules/delta-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sk_demo_page_server", "sk_element", "sk_element_puppeteer_test", "sk_page")

# This target will be deleted because its source files no longer exist.
sk_element(
    name = "delta-sk",
    sass_srcs = ["delta-sk.scss"],
    ts_srcs = ["delta-sk.ts"],
    visibility = ["//visibility:public"],
)

# This target will be deleted because its source files no longer exist.
sk_page(
    name = "delta-sk-demo",
    html_file = "delta-sk-demo.html",
    scss_entry_point = "delta-sk-demo.scss",
    ts_entry_point = "delta-sk-demo.ts",
)

# This target will be deleted because its sk_demo_page_server will be deleted as well.
sk_element_puppeteer_test(
    name = "delta-sk_puppeteer_test",
    src = "delta-sk_puppeteer_test.ts",
    sk_demo_page_server = ":demo_page_server",
)

# This target will be deleted because its sk_page will be deleted as well.
sk_demo_page_server(
    name = "demo_page_server",
    sk_page = ":delta-sk-demo",
)
`,
		},
		{Path: "myapp/modules/delta-sk/delta-sk_puppeteer_test.ts"},
	}, makeBasicWorkspace()...)

	expectedOutputFiles := []testtools.FileSpec{
		{
			Path: "a/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sass_library", "ts_library")

sass_library(
    name = "alfa_sass_lib",
    srcs = ["alfa.scss"],
    visibility = ["//visibility:public"],
)

ts_library(
    name = "alfa_ts_lib",
    srcs = ["alfa.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
		{
			Path: "myapp/modules/charlie-sk/BUILD.bazel",
			Content: `
load("//infra-sk:index.bzl", "sk_demo_page_server", "sk_element", "sk_page")

sk_element(
    name = "charlie-sk",
    ts_srcs = ["charlie-sk.ts"],
    visibility = ["//visibility:public"],
)

sk_page(
    name = "charlie-sk-demo",
    html_file = "charlie-sk-demo.html",
    ts_entry_point = "charlie-sk-demo.ts",
)

sk_demo_page_server(
    name = "demo_page_server",
    sk_page = ":charlie-sk-demo",
)
`,
		},
		{Path: "myapp/modules/delta-sk/BUILD.bazel"}, // Empty because its only rule was removed.
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
