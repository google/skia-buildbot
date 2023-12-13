# Bazel cheatsheet

This cheatsheet provides quick tips on how to build and test code in our repository using Bazel.

Start [here](https://docs.bazel.build/versions/4.1.0/bazel-overview.html) if you're completely new
to Bazel.

The original design documents for our Bazel build can be found at the following Golinks:

- [go/skia-infra-bazel](http://go/skia-infra-bazel)
- [go/skia-infra-bazel-frontend](http://go/skia-infra-bazel-frontend)
- [go/skia-infra-bazel-backend](http://go/skia-infra-bazel-backend)

## Initial setup

This section includes steps every engineer should follow to get a consistent development
experience.

### Install Bazelisk

[Bazelisk](https://github.com/bazelbuild/bazelisk) is a wrapper for Bazel that downloads and runs
the version of Bazel specified in `//.bazelversion`. It serves a similar purpose as
[nvm](https://github.com/nvm-sh/nvm) does for NodeJS.

Bazelisk is recommended over plain Bazel because the `bazel` command on our gLinux workstations is
automatically updated every time a new version of Bazel is released.

To install Bazelisk, grab the latest binary for your platform from
[GitHub](https://github.com/bazelbuild/bazelisk/releases), then add it to your `PATH`.

Tips:

- Make a Bash alias with `alias bazel="bazelisk"` and add it to your `~/.bash_aliases` file.
- Set the full path to `bazel` to be the full path to `bazelisk` in your IDE of choice. This is
  necessary for some extensions to work correctly, such as the
  [Bazel plugin for Visual Studio Code](https://marketplace.visualstudio.com/items?itemName=BazelBuild.vscode-bazel).

### Set up RBE access

[Remote Build Execution](https://bazel.build/remote/rbe) (RBE) allows faster build and test
execution times by distributing build and test actions across multiple machines.

Run the following command to get access our RBE instance:

```
$ gcloud auth application-default login
```

This should enable you to utilize RBE by including the flag `--config=remote` in your Bazel
invocations. For more details on this flag, see below.

Then, create a `//bazel/user/bazelrc` file in your repository checkout with the following contents:

```
build:mayberemote --config=remote
```

This will enable RBE for any Bazel invocations in scripts, Make targets, etc.

**Warning:** Do not use `--config=mayberemote` in your manual Bazel invocations. The "mayberemote"
configuration is only intended to be used from scripted actions. For manual Bazel invocations, use
`--config=remote` instead, which will explicitly let you know if there are any problems with your
RBE setup. For more details on the `--config=mayberemote` flag and why it is necessary, see below.

## Gazelle

We use [Gazelle](https://github.com/bazelbuild/bazel-gazelle) to automatically generate
`BUILD.bazel` files for most of our Go and TypeScript code.

Note that we occasionally edit Gazelle-generated `BUILD.bazel` files by hand, e.g. to mark tests as
[flaky](#flaky-tests).

### Usage

Run `make gazelle` from the repository's root directory.

### TypeScript support

TypeScript support is provided via a custom Gazelle extension which can be found in
`//bazel/gazelle/frontend`.

Tip: See [here](#testing-typescript-code) for details on how this extension decides which rule to
generate for a given TypeScript file.

## Buildifier

[Buildifier](https://github.com/bazelbuild/buildtools/tree/master/buildifier) is a linter and
formatter for `BUILD.bazel` files and other Bazel files (`WORKSPACE`, `*.bzl`, etc.).

### Usage

Run `bazel run //:buildifier`.

## Bazel CI tasks

Our Bazel build is tested on RBE via the following tasks:

- Infra-PerCommit-Build-Bazel-RBE (roughly equivalent to `bazel build //... --config=remote`)
- Infra-PerCommit-Test-Bazel-RBE (roughly equivalent to `bazel test //... --config=remote`)

We regard the above tasks as the source of truth for build and test correctness.

As an insurance policy against RBE outages, we also have the following tasks:

- Infra-PerCommit-Build-Bazel-Local (roughly equivalent to `bazel build //...`)
- Infra-PerCommit-Test-Bazel-Local (roughly equivalent to `bazel test //...`)

The non-RBE tasks tend to be a bit more brittle than the RBE ones, which is why they are excluded
from the CQ.

## Building and testing

Use commands `bazel build` and `bazel test` to build and test Bazel targets, respectively.
Examples:

```
# Single target.
$ bazel build //go/util:util
$ bazel test //go/util:util_test

# All targets under a directory and any subdirectoriews.
$ bazel build //go/...
$ bazel test //go/...

# All targets in the repository.
$ bazel build //...
$ bazel test //...
```

Any build artifacts produced by `bazel build` or `bazel test` will be found under `//_bazel_bin`.

Note that it's not necessary to `bazel build` a test target before `bazel test`-ing it.
`bazel test` will automatically build the test target if it wasn't built already (i.e. if it
wasn't found in the Bazel cache).

More on `bazel build`
[here](https://docs.bazel.build/versions/main/guide.html#building-programs-with-bazel).

More on `bazel test` [here](https://docs.bazel.build/versions/main/user-manual.html#test).

### Building and testing on RBE

By default, Bazel will build and test targets on the host system (aka a local build). To build on
RBE, invoke Bazel with flag `--config=remote`, e.g.:

```
$ bazel build //go/util:util --config=remote
$ bazel test //go/util:util_test --config=remote
```

This repository contains some scripted actions that shell out to Bazel, such as certain `make`
targets (e.g. `make gazelle`, `make buildifier`) and `go generate` actions. These actions use the
"mayberemote" configuration via the `--config=mayberemote` flag, e.g.:

```
# //Makefile

update-go-bazel-files:
	$(BAZEL) run --config=mayberemote //:gazelle -- update ./
```

By default, the "mayberemote" configuration does nothing. This is to support users that might not
have RBE access, or when working offline (e.g. on a plane with no WiFi). To get the benefits of RBE
when running scripted actions, please create a `//bazel/user/bazelrc` file with the following
contents:

```
build:mayberemote --config=remote
```

To learn more about the `mayberemote` configuration:

- See comments in `//.bazelrc`
  [here](https://skia.googlesource.com/buildbot/+/576558265598b54751233441814aa389f1e96d53/.bazelrc#142).
- See [`//bazel/user/README.md`](https://skia.googlesource.com/buildbot/+/576558265598b54751233441814aa389f1e96d53/bazel/user/README.md).

## Running Bazel-built binaries

Use command `bazel run` to run binary Bazel targets (such as `go_binary`, `sh_binary`, etc.), e.g.:

```
# Without command-line parameters.
$ bazel run //scripts/run_emulators:run_emulators

# With command-line parameters.
$ bazel run //scripts/run_emulators:run_emulators -- start
```

Alternatively, you can run the Bazel-built artifact directly, e.g.:

```
$ bazel build //scripts/run_emulators:run_emulators
$ _bazel_bin/scripts/run_emulators/run_emulators_/run_emulators start
```

The exact path of the binary under `//_bazel_bin` depends on the Bazel rule (`go_binary`,
`py_binary`, etc.). As you can see, said path can be non-obvious, so it's generally recommended to
use `bazel run`.

More on `bazel run` [here](https://docs.bazel.build/versions/main/user-manual.html#run).

## Back-end development in Go

Our Go codebase is built and tested using Bazel rules from the
[rules_go](https://github.com/bazelbuild/rules_go) repository. The `go_test` rule
[documentation](https://github.com/bazelbuild/rules_go/blob/master/go/core.rst#go_test) is a great
read to get started.

As mentioned in the [Gazelle](#gazelle) section, all Bazel targets for Go code are generated with
Gazelle.

Read [go/skia-infra-bazel-backend](http://go/skia-infra-bazel-backend) for the full details.

### Invoking "go", "gofmt" and other tools

On non-Bazel Go projects, developers typically use locally installed binaries such as `go` and
`gofmt` for code generation and code formatting tasks. However, our Bazel build aims to be as
[hermetic](https://bazel.build/basics/hermeticity) as possible. To this end, rather than requiring
the developer to install a Go SDK on their system, we provide convenience Bazel targets defined in
`//BUILD.bazel` to invoke binaries in the Bazel-downloaded Go SDK and other Bazel-downloaded tools.

Example invocations:

```
# Equivalent to "go generate ./..."
$ bazel run //:go -- generate ./...

# Equivalent to "gofmt -s -w ."
$ bazel run //:gofmt -- -s -w .

# Equivalent to "errcheck go.skia.org/infra/..."
$ bazel run //:errcheck -- go.skia.org/infra/...

# Equivalent to "protoc --go_out . myproto.proto"
$ bazel run //:protoc -- --go_out=. myproto.proto
```

Our CI tasks and Makefiles use these Bazel targets. This prevents diffs that might arise from
using locally installed binaries, which might differ from system to system. Developers should
always use Bazel-downloaded binaries for any tasks that produce changes in checked-in files.

Note that it might still be desirable to have a locally installed Go SDK. For example,
[Visual Studio Code](https://code.visualstudio.com/)'s
[Go extension](https://code.visualstudio.com/docs/languages/go) requires a locally installed Go SDK
to enable autocompletion and debugging. It is the developer's responsibility to ensure that their
locally installed Go SDK matches the version used by the Bazel build, which is defined in the
[`//WORKSPACE`](https://skia.googlesource.com/buildbot/+/d18bcaf0173de9c054dd5809fe5ccd459a1adec5/WORKSPACE#99)
file.

### Building Go code

Simply use `bazel build` (and optionally `bazel run`) as described
[earlier](#building-and-testing).

### Testing Go code

Tip: Start by reading the [General testing tips](#general-testing-tips) section.

Our setup differs slightly from typical Go + Bazel projects in that we use a wrapper macro around
`go_test` to handle manual tests. Gazelle is configured to use this macro via a `gazelle:map_kind`
directive in `//BUILD.bazel`. The macro is defined in `//bazel/go/go_test.bzl`. Read the macro's
docstring for the full details.

#### Manual Go tests

To mark specific Go test cases as manual, extract them out into a separate file ending with
`_manual_test.go` within the same directory.

The `go_test` macro in `//bazel/go/go_test.bzl` places files ending with `_manual_test.go` in a
separate `go_test` target, which is tagged as manual.

More on manual tests [here](#manual-tests).

#### Passing flags to Go tests

The `go test` command supports flags such as `-v` to print verbose outputs, `-run` to run a
specific test case, etc. Under Bazel, these flags can be passed to a `go_test` test target via
`--test_arg`, but they need to be prefixed with `-test.`, e.g.:

```
# Equivalent to "go test ./go/util -v".
$ bazel test //go/util:util_test --test_arg=-test.v

# Equivalent to "go test ./go/util -run=TestFoo"
$ bazel test //go/util:util_test --test_arg=-test.run=TestFoo
```

#### Example `bazel test` invocation for Go tests

The following example shows what a typical `bazel test` invocation might look like while debugging
a `go_test` target locally.

```
# Equivalent to "$ MY_ENV_VAR=foo go test ./go/my_pkg -v -logtostderr"
$ bazel test //go/my_pkg:my_pkg_test \
             --test_output=streamed \
             --nocache_test_results \
             --test_arg=-test.v \
             --test_arg=-logtostderr \
             --test_env=MY_ENV_VAR=foo
```

### Go modules

Unlike normal Go projects, Bazel Go projects based on rules_go specify Go module dependencies via
Gazelle's
[`go_repository`](https://github.com/bazelbuild/bazel-gazelle/blob/master/repository.md#go_repository)
rule. In our repository, those rules are located in `//go_repositories.bzl`.

While a Bazel Go project does not need a `go.mod` file, absence of a `go.mod` file breaks common
tooling such as VSCode's code completion. For this reason, our repository contains a `//go.mod`
file which we treat as the source of truth for Go dependencies, and we automatically generate file
`//go_repositories.bzl` from `//go.mod` via Gazelle.

#### Adding or updating Go modules

The process is similar to non-Bazel Go projects, but instead of running e.g.
`go get example.com/foo@v0.1.2`, we use the `//:go` Bazel wrapper mentioned earlier. This avoids
potential differences in `//go.mod` and `//go.sum` that might be introduced by a local Go SDK whose
version differs from the hermetic Go SDK downloaded by Bazel.

First, update the `//go.mod` file:

```
$ bazel run //:go -- get example.com/foo@v0.1.2
```

Then run Gazelle, which will update `//go_repositories.bzl`:

```
$ make gazelle
```

#### Troubleshooting Go dependency changes

While the above steps work for most Go dependencies, we occasionally run into issues that require
manual work to resolve.

For example:

- Most Go modules do not include `BUILD` files, in which case the `go_repository` rule will
  generate `BUILD` files automatically using Gazelle. However, some Go modules are distributed
  with `BUILD` files, in which case the `go_repository` rule will not generate any `BUILD` files.
  This behavior can be customized to some extent via the `go_repository` rule's
  `build_file_generation` attribute
  ([example](https://skia-review.googlesource.com/c/buildbot/+/772355/6/go_repositories.bzl#205)).

- For those Go modules that include `BUILD` files, sometimes the target names in those files will
  use the
  [`go_default_library`](https://github.com/bazelbuild/rules_go#what-s-up-with-the-go-default-library-name)
  naming convention. In such cases, we must explicitly tell the `go_repository` rule via the
  `build_naming_convention` attribute
  ([example](https://skia.googlesource.com/buildbot/+/232683f4840a825514e931d8201940388b3997dc/go_repositories.bzl#204)).

- For Go modules that include `.proto` files, sometimes they also include pre-generated `.pb.go`
  files, in which case we need to tell the `go_repository` rule not to generate
  [`go_proto_library`](https://github.com/bazelbuild/rules_go/blob/master/proto/core.rst#go_proto_library)
  targets
  ([example](https://skia.googlesource.com/buildbot/+/232683f4840a825514e931d8201940388b3997dc/go_repositories.bzl#3174)).

Another important aspect to keep in mind while debugging problematic Go modules is that the order
in which we import external Bazel repositories in the `//WORKSPACE` file matters greatly. This is
relevant to Go modules because the `//go_repositories.bzl` file is imported from `//WORKSPACE`.

As an example, on Q4 2023 we updated the `google.golang.org/grpc` Go module. This update was
particularly challenging because it required:

- Investigating and simplifying non-`go_repository` external Bazel repositories in `//WORKSPACE`.
- Updating both rules_go and Gazelle.
- Updating numerous transitive dependencies which required customizing the attributes of some
  `go_repository` rules in `//go_repositories.bzl`
- Compatibility hacks for certain Go modules that assumed an older version of `rules_go`.

This [bug](https://g-issues.skia.org/issues/308044304) tracks all the work that went into this
update. We recommend reading through the CL descriptions; it might give you some ideas as to what
to try next if you get stuck during a tricky Go module update.

## Front-end development in TypeScript

Our front-end code is built and tested using a set of custom Bazel macros built on top of rules
provided by the [rules_js](https://github.com/aspect-build/rules_js) and
[rules_ts](https://github.com/aspect-build/rules_ts) rulesets. All such macros are either defined
in or re-exported from `//infra-sk/index.bzl`. This section uses the terms macro and rule
interchangeably when referring to the macros exported from said file.

As mentioned in the [Gazelle](#gazelle) section, most Bazel targets for front-end code are
generated with Gazelle.

Read [go/skia-infra-bazel-frontend](http://go/skia-infra-bazel-frontend) for the full details.

### Building TypeScript code

Simply use `bazel build` (and optionally `bazel run`) as described
[earlier](#building-and-testing).

### Working with demo pages

Demo pages are served via a Gazelle-generated `sk_demo_page_server` rule.

Use `bazel run` to serve a demo page via its `sk_demo_page_server` rule, e.g.:

```
$ bazel run //golden/modules/dots-sk:demo_page_server
```

#### Watching for changes

To rebuild the demo page automatically upon changes in the custom element's directory, use the
`demopage.sh` script found in the repository's root directory, e.g.:

```
$ ./demopage.sh golden/modules/dots-sk
```

This script uses [entr](https://eradman.com/entrproject/) to watch for file changes and re-execute
the `bazel run` command as needed. The above `demopage.sh` invocation is equivalent to:

```
$ ls golden/modules/dots-sk/* | entr -r bazel run //golden/modules/dots-sk:demo_page_server
```

Install `entr` on a gLinux workstation with `sudo apt-get install entr`.

In the future, we might replace this script with
[ibazel](https://github.com/bazelbuild/bazel-watcher), which requires changes to the
`sk_demo_page_server` rule.

### Testing TypeScript code

Tip: Start by reading the [General testing tips](#general-testing-tips) section.

Front-end code testing is done via three different Bazel rules:

- `karma_test` for in-browser tests based on the Karma test runner.
- `sk_element_puppeteer_test` for Puppeteer tests that require a running `sk_demo_page_server`.
- `nodejs_test` for any other server-side TypeScript tests (i.e. NodeJS tests).

Gazelle decides which rule to generate for a given `*_test.ts` file based the following patterns:

- `karma_test` is used for files matching `//<app>/modules/<element>/<element>_test.ts`.
- `sk_element_puppeteer_test` is used for files matching
  `//<app>/modules/<element>/<element>_puppeteer_test.ts`.
- `nodejs_test` is used for files matching `*_nodejs_test.ts`.

#### Karma tests (`karma_test` rule)

Use `bazel test` to run a Karma test in headless mode:

```
$ bazel test //golden/modules/dots-sk:dots-sk_test
```

To run a Karma test in the browser during development, use `bazel run` instead:

```
$ bazel run //golden/modules/dots-sk:dots-sk_test
...
Karma v4.4.1 server started at http://<hostname>:9876/
```

##### Watching for changes

As an alternative to `bazel run` when debugging tests in the browser, consider using the
`karmatest.sh` script found in the repository's root directory. Similarly to the `demopage.sh`
script mentioned earlier, it watches for changes in the custom element's directory, and relaunches
the test runner when a file changes. Example usage:

```
$ ./karmatest.sh golden/modules/digest-details-sk
```

As with `demopage.sh`, this script depends on the `entr` command, which can be installed on a
gLinux workstation with `sudo apt-get install entr`.

#### Puppeteer tests (`sk_element_puppeteer_test` rule)

Use `bazel test` to run a Puppeteer test, e.g.:

```
$ bazel test //golden/modules/dots-sk:dots-sk_puppeteer_test
```

To view the screenshots captured by a Puppeteer test, use the `//:puppeteer_screenshot_server`
target:

```
$ bazel run //:puppeteer_screenshot_server
...
Serving Puppeteer screenshots viewer at: http://<hostname>:8000
```

To extract the screenshots captured by a Puppeteer test into a directory, use the
`//:extract_puppeteer_screenshots` target:

```
$ mkdir /tmp/screenshots
$ bazel run //:extract_puppeteer_screenshots -- --output_dir /tmp/screenshots
```

To step through a Puppeteer test with a debugger, run your test with `bazel run`, and append
`_debug` at the end of the target name, e.g.:

```
# Normal test execution (for reference).
$ bazel test //golden/modules/dots-sk:dots-sk_puppeteer_test

# Test execution in debug mode.
$ bazel run //golden/modules/dots-sk:dots-sk_puppeteer_test_debug
```

This will print a URL to stdout that you can use to attach a Node.js debugger (such as the VS Code
Node.js debugger, or Chrome DevTools). Your test will wait until a debugger is attached before
continuing.

Example debug session with Chrome DevTools:

1. Add one or more `debugger` statements in your test code to set breakpoints, e.g.:

```
// //golden/modules/dots-sk/dots-sk_puppeteer_test.ts

describe('dots-sk', () => {
  it('should do something', () => {
    debugger;
    ...
  });
});
```

2. Run `bazel run //golden/modules/dots-sk:dots-sk_puppeteer_test_debugger`.
3. Launch Chrome **in the machine where the test is running**, otherwise Chrome won't see the
   Node.js process associated to your test.
4. Enter `chrome://inspect` in the URL bar, then press return.
5. You should see an "inspect" link under the "Remote Target" heading.
6. Click that link to launch a Chrome DevTools window attached to your Node.js process.
7. Click the "Resume script execution" button (looks like a play/pause icon).
8. Test execution should start, and eventually pause at your `debugger` statement.

By default, Puppeteer starts a Chromium instance in headless mode. If you would like to run your
test in headful mode, invoke your test with `bazel run`, and append `_debug_headful` at the end of
the target name, e.g.:

```
$ bazel run //golden/modules/dots-sk:dots-sk_puppeteer_test_debug_headful
```

Run your test in headful mode to visually inspect how your test interacts with the demo page under
test as you step through your test code with the attached debugger.

#### NodeJS tests (`nodejs_test` rule)

Use `bazel test` to run a NodeJS test, e.g.:

```
$ bazel test //puppeteer-tests:util_nodejs_test
```

## General testing tips

The below tips apply to all Bazel test targets (e.g. `go_test`, `karma_test`, etc.).

### Test output

By default, Bazel omits the standard output of tests (e.g. `fmt.Println("Hello")`).

Use flag `--test_output=all` to see the full output of your tests:

```
$ bazel test //perf/... --test_output=all
```

Note that Bazel runs tests in parallel, so it will only print out their output once all tests have
finished running.

Flag `--test_output=errors` can be used to only print out the output of failing tests.

To see the tests' output in real time, use flag `--test_output=streamed`. Note however that this
forces serial execution of tests, so this can be significantly slower.

### Caching

Bazel caches successful test runs, and reports `(cached) PASSED` on subsequent `bazel test`
invocations, e.g.:

```
$ bazel test //go/util:util_test
...
//go/util:util_test                                                      PASSED in 0.1s

$ bazel test //go/util:util_test
...
//go/util:util_test                                             (cached) PASSED in 0.1s
```

To disable caching, use flag `--nocache_test_results`, e.g.

```
$ bazel test //go/util:util_test
...
//go/util:util_test                                             (cached) PASSED in 0.1s

$ bazel test //go/util:util_test --nocache_test_results
...
//go/util:util_test                                                      PASSED in 0.1s
```

### Flaky tests

Flaky tests can cause the CI to fail (see [Bazel CI tasks](#bazel-ci-tasks)).

Tests can be marked as flaky via the `flaky` argument, e.g.:

```
go_test(
    name = "some_flaky_test",
    srcs = ["some_flaky_test.go"],
    flaky = True,
    ...
)
```

Bazel will execute tests marked as flaky up to three times, and report test failure only if the
three attempts fail.

Using `flaky` is generally discouraged, but can be useful until the root cause of the flake is
diagnosed (see [Debugging flaky tests](#debugging-flaky-tests)) and fixed.

As a last resort, consider marking your flaky test as manual (see [Manual tests](#manual-tests)).

More on the `flaky` attribute
[here](https://docs.bazel.build/versions/main/be/common-definitions.html#common-attributes-tests).

### Debugging flaky tests

While `--nocache_test_results` can be useful for debugging flaky tests, flag `--runs_per_test` was
specifically added for this purpose. Example:

```
$ bazel test //path/to:some_flaky_test --runs_per_test=10
...
//path/to:some_flaky_test                                             FAILED in 4 out of 10 in 0.1s
```

### Manual tests

Manual tests are excluded from Bazel wildcards such as `bazel test //...`.

To mark a test target as manual, use the `manual` tag, e.g.:

```
nodejs_test(
    name = "some_manual_nodejs_test",
    src = "some_manual_nodejs_test.ts",
    tags = ["manual"],
    ...
)
```

Note that the instructions to mark `go_test` targets as manual are different. See
[Manual Go tests](#manual-go-tests) for more.

Note that manual tests are excluded from the [Bazel CI tasks](#bazel-ci-tasks).

More on manual tests and Bazel tags
[here](https://docs.bazel.build/versions/main/be/common-definitions.html#common-attributes).

### Test timeouts

By default, Bazel will report `TIMEOUT` if the test does not finish within 5 minutes. This can be
overridden via the `--test_timeout` flag, e.g.

`$ bazel test //go/util:slow_test --test_timeout=20`

This can also be overridden via the `timeout` and `size` arguments of the test target, e.g.

```
go_test(
    name = "my_test",
    srcs = ["my_test.go"],
    timeout = "long",
    ....
)
```

More on how to handle timeouts and slow tests
[here](https://docs.bazel.build/versions/master/be/common-definitions.html#test.timeout).

### Passing command-line flags to test binaries

Use flag `--test_arg` to pass flags to the binary produced by a test target.

For example, our `go_test` targets define custom command-line flags such as
`flag.Bool("logtostderr", ...)`. This flag can be enabled with `--test_arg`, e.g.:

```
$ bazel test //go/util:util_test --test_arg=-logtostderr
```

As an alternative, command-line flags can be specified via the `args` argument of the Bazel test
target, as follows:

```
go_test(
    name = "my_test",
    srcs = ["my_test.go],
    args = ["-logtostderr"],
    ...
)
```

More on test arguments
[here](https://docs.bazel.build/versions/master/be/common-definitions.html#test.args).

### Overriding environment variables

By default, Bazel isolates test targets from the host system's environment variables, and sets the
environment with a number of variables with Bazel-specific information that some `*_test` rules
depend on (documented
[here](https://docs.bazel.build/versions/main/test-encyclopedia.html#initial-conditions)).

Use flag `--test_env` to specify any environment variables, e.g.

```
$ bazel test //path/to:my_cockroachdb_test --test_env=COCKROACHDB_EMULATOR_STORE_DIR=/tmp/crdb
```

To pipe through an environment variable from the host's system:

```
$ export COCKROACHDB_EMULATOR_STORE_DIR=/tmp/crdb
$ bazel test //path/to:my_cockroachdb_test --test_env=COCKROACHDB_EMULATOR_STORE_DIR
```

More on the `--test_env` flag
[here](https://docs.bazel.build/versions/main/command-line-reference.html#flag--test_env).

### Faster Sandboxing

By default, Bazel [sandboxes](https://docs.bazel.build/versions/main/sandboxing.html)
every build step. Effectively, it runs the compile command with only the given source files for a
particular rule and the specified dependencies visible, to force all dependencies to be
properly listed.

For steps that have a lot of files, this can have a bit of I/O overhead. To speed this up, one
can use tempfs (e.g. a RAM disk) for the sandbox by adding `--sandbox_base=/dev/shm` to the build
command. When compiling Skia, for example, this reduces compile time by 2-3x.

Sandboxing can make diagnosing failing rules a bit harder. To see what command got run and to be
able to view the sandbox after failure, add `--subcommands --sandbox_debug` to the command.

### BUILD.bazel file debugging

Bazel builds fast and correct by making use of cached outputs and reusing them when
the input file is identical. This can make it hard to debug a slow or non-deterministic
build.

To get a detailed log of all the actions your build is taking:

1. Add the following to your .bazelrc

```
# ensure there are no disk cache hits
build --disk_cache=/path/to/debugging/cache
# IMPORTANT Generate execution logs
build --experimental_execution_log_file=yourLogFile.log

```

2. Run `bazel clean --expunge`. We want all actions to get executed, so nothing cached.
3. Look at the yourLogFile.log, it will contain a record of every action bazel executed,
   environment variables, command line, input files, and output files of every action.

### Querying

[Bazel has a query feature](https://bazel.build/query/quickstart) that lets one extract
information from the build graph.

There's a `query` and `cquery` variant that lets one query for the maximal set of information
or the information in one specific case, respectively.

For example:

```
# Show all possible build flags (e.g. defines, copts) and other information about a label
bazel query 'kind("rule", //:skia_public)' --output xml

# Show the build flags for this specific build configuration (release)
bazel cquery 'kind("rule", //:skia_public)' --output jsonproto --config=release
```

This type of [querying is used extensively to generate .gni and CMakefiles](https://github.com/google/skia/blob/cf1fad655769f3e1a47a7b6d876efb0d7c5e0efa/bazel/exporter/bazel_query_command.go#L76-L116).
