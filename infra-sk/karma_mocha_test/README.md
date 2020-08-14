# karma_mocha_test

This directory contains the `karma_mocha_test` Bazel rule, which is based on the `karma_web_test`
rule defined
[here](https://github.com/bazelbuild/rules_nodejs/blob/374f56ff442e9d2a82fb0c0773db5e734afd4bee/packages/karma/karma_web_test.bzl#L330).

The reason the `karma_mocha_test` rule is necessary is because most JavaScript / TypeScript tests
in this reposiory are written for the [Mocha](https://mochajs.org/) test framework, whereas the
`karma_web_test` rule makes the hard assumption that tests are written for the
[Jasmine](https://jasmine.github.io/) framework, and does not provide a mechanism to override this.

The contents of this directory are copied from the `//packages/karma` directory in the
[rules_nodejs]((https://github.com/bazelbuild/rules_nodejs) repository with minimal changes to
replace Jasmine with Mocha and to remove some features we do not need.

In the future we might consider migrating our tests to Jasmine so we can use the standard
`karma_web_test` rule and delete this directory.
