# Puppeteer Tests

This directory contains utility functions and infrastructure for JavaScript
tests that that make use of [Puppeteer](https://pptr.dev). Puppeteer is a
Node.js library that provides an API to instantiate and control a headless
Chromium browser. Most things that can be done manually in the browser can be
done using Puppeteer.

Examples of tests that make use of Puppeteer might include:

 - Screenshot-grabbing tests. In the case of lit-html components, such a test
   might perform the following steps:
   1. Load the component's demo page on a headless Chromium instance.
   2. Perform some basic assertions on the structure of the webpage to make sure
      it loads correctly.
   3. Grab screenshots.
   4. Upload those screenshots to the
      [Skia Infra Gold](https://skia-infra-gold.skia.org/) instance.

 - Integration tests. For example:
   1. Fire up a Go web server configured to use fake/mock instances of its
      dependencies.
      - e.g. use the Firestore emulator instead of hitting GCP.
      - Ideally in a
        [hermetic](https://testing.googleblog.com/2012/10/hermetic-servers.html)
        way for increased speed and reduced flakiness.
   2. Drive the app with Puppeteer (e.g. by clicking on things, using the
      browser's back and forward buttons, etc.). Make assertions along the way.
   3. Optionally grab screenshots and upload them to Gold.

## Running Puppeteer tests

First, make sure to run `npm ci` from the repository root if you haven't
already.

Execute `make puppeteer-tests` from this repository's root directory. This will
run all Puppeteer tests in the buildbot repository inside a Docker container.

(If the container image is missing, please run `make container` from
`//puppeteer-tests/docker`.)

## Adding new tests

If your application isn't already configured to run its Puppeteer tests as part
of the `Infra-PerCommit-Puppeteer` tryjob, please edit
`//puppeteer-tests/docker/run-tests.sh` and make any necessary changes. See
[below](#enabling-puppeteer-tests-for-your-application) for more.

## How to write Puppeteer tests

Puppeteer tests run on Node.js and are assumed to use the
[Mocha](https://mochajs.org/) test runner.

Puppeteer tests make use of the utility functions defined in
`//puppeteer-tests/util.js` to launch a Puppeteer instance and take screenshots.

Screenshots will be found at `//puppeteer-tests/output` after the tests finish.
The `Infra-PerCommit-Puppeteer` tryjob will upload to
[Gold](https://skia-infra-gold.skia.org/) any screenshots found in this
directory, using the file name as the test name (excluding the `.png`
extension).

The best way to learn how to write Puppeteer tests is by example. See
`//golden/**/*_puppeteer_test.js` for some.

### Test naming scheme

The recommended naming scheme for screenshots is
`<application>_<component-name-sk>_<scenario>`. This is to avoid name collisions
and to be consistent with existing tests.

For example, tests for Gold's `dots-sk` component are named as follows:
- `gold_dots-sk` (default view).
- `gold_dots-sk_highlighted` (mouse over).

### Debugging tips

Run `npx mocha` from your application's subdirectory to execute Puppeteer tests
locally (outside of Docker). Examples:

```
# First we "cd" into Gold's subdirectory, as we will only run Gold tests below.
cd golden

# Run all of Gold's Puppeteer tests.
npx mocha ./**/*_puppeteer_test.js

# Run all tests containing "dots-sk" in their name.
npx mocha ./**/*_puppeteer_test.js -g dots-sk

# Wait for a debugger to attach to the Node process before running the tests.
npx mocha ./**/*_puppeteer_test.js --inspect-brk
```

## Docker container

To reduce flakiness, the `Infra-PerCommit-Puppeteer` tryjob runs all Puppeteer
tests run inside the `gcr.io/skia-public/puppeteer-tests:latest` Docker
container. This container is based on the `node:12.13` image and includes some
additional dependencies required by Puppeteer.

The container is defined in `//puppeteer-tests/docker/Dockerfile`, and can be
rebuilt by running `make container` from the `//puppeteer-tests/docker`
directory.

To push a new version of the container to the GCP's Container Registry, please
run `make push`.

## How the `Infra-PerCommit-Puppeteer` tryjob works

This tryjob is orchestrated by the `//infra/bots/recipe/puppeteer_tests.py`
recipe, which performs the following steps:

  1. It runs `make puppeteer-tests` from the root directory of the checked
     out repository.
     1. This runs the `gcr.io/skia-public/puppeteer-tests:latest` Docker
        container.
     2. The checked out repository at `/src`, and the `//puppeteer-tests/output`
        directory is mounted at `/out`.
     3. Script `//puppeteer-tests/docker/run-tests.sh` is executed *inside* the
        container.
     4. This script copies any files required to run the Puppeteer tests from
        `/src` to `/tests` inside the container, installs the Node dependencies
        with `npm ci` and runs the tests with `npx mocha`.
  2. After the tests finish running, it executes the
     `//puppeteer-tests/upload-screenshots-to-gold.py` script, which invokes
     `goldctl` with any screenshots found in `//puppeteer-tests/output`.

## Enabling Puppeteer tests for your application

In order to include an application's Puppeteer tests in the
`Infra-PerCommit-Puppeteer` tryjob, file `//puppeteer-tests/docker/run-tests.sh`
must copy any necessary files into the Docker container, install any necessary
dependencies and actually run the tests.

This is done in two steps:

1. Make a new directory inside `/tests`, e.g. `/tests/my-app`, and copy the
   necessary files from `//my-app` in the repository.
2. Run the Mocha test runner from `/tests/my-app`, e.g.
   `npx mocha /tests/**/*_puppeteer_test.js`.
