# Puppeteer Tests

This directory contains JS tests that make use of [Puppeteer](https://pptr.dev).
Puppeteer is a Node.js library that provides an API to instantiate and control a
headless Chromium browser. Most things that can be done manually in the browser
can be done using Puppeteer.

Examples of such tests might include:

 - Screenshot-grabbing tests. For example, in the case of lit-html components,
   such a test might perform the following steps:
   1. Load the component's demo page on a headless Chromium instance.
   2. Perform some basic assertions on the structure of the webpage to make sure
      it loads correctly.
   3. Grab screenshots.
   4. Upload those screenshots to the Skia Infra Gold instance.
      <!-- TODO(lovisolo): Set up a Gold instance for Skia Infra.
                           Add a link to it here. -->

 - Integration tests. Example steps:
   1. Fire up a Go web server configured to use fake/mock instances of its
      dependencies.
      - e.g. use the Firestore emulator instead of hitting GCP.
      - Ideally in a
        [hermetic](https://testing.googleblog.com/2012/10/hermetic-servers.html)
        way for increased speed and reduced flakiness.
   2. Drive the app with Puppeteer. Make assertions along the way.
   3. Optionally grab screenshots and upload them to Gold.

Tests under this directory use the [Mocha](https://mochajs.org/) test runner.

Any output files generated from these tests (e.g. screenshots) will be found in
`$SKIA_INFRA_ROOT/golden/puppeteer-tests/output`.

### Docker container

Puppeteer tests run inside a Docker container. This provides a more stable
testing environment and reduces screenshot flakiness.

The corresponding `Dockerfile` can be found in
`$SKIA_INFRA_ROOT/golden/puppeteer-tests/docker`.

## Usage

Run `make puppeteer-test` from `$SKIA_INFRA_ROOT/golden`. This will build and
run a Docker container that executes the Mocha test runner.

If you wish to run these tests outside of Docker, try
`make puppeteer-test-nodocker`. Or equivalently, `cd` into
`$SKIA_INFRA_ROOT/golden/puppeteer-tests/test` and run `npx mocha`.

## Debugging

The two options below run the tests outside of Docker.

 - `cd` into `$SKIA_INFRA_ROOT/golden/puppeteer-tests/test` and run
   `npx mocha debug`. This will start the tests and immediately drop into the
   Node.js inspector.
 - Alternatively, `cd` into `$SKIA_INFRA_ROOT/golden/puppeteer-tests/test` and
   run `npx mocha --inspect-brk`. This will start the tests and wait until a
   debugger is attached. Attach any debgger of your liking, e.g. Chrome Dev
   Tools, VSCode, IntelliJ, etc. See
   [here](https://mochajs.org/#-debug-inspect-debug-brk-inspect-brk-debug-inspect)
   for more.
