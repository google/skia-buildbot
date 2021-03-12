# Gazelle extension for Skia Infrastructure front-end code

This Gazelle extension generates Bazel build targets for front-end code (TypeScript, Sass) using the
rules defined in `//infra-sk/index.bzl`. Specifically, it generates the following kinds of targets:

 - ts_library
 - karma_test
 - sass_library
 - sk_element
 - sk_page
 - sk_element_demo_page_server
 - sk_element_puppeteer_test

## How Gazelle extensions work

A Gazelle extension is essentially a `go_library` with a function named `NewLanguage` that provides
an implementation of the `language.Language` interface. This interface provides hooks for generating
rules, parsing configuration directives, and resolving imports to Bazel labels.

Documentation: https://github.com/bazelbuild/bazel-gazelle/blob/master/extend.rst.
