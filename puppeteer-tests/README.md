# Puppeteer Tests

This directory contains utility functions and infrastructure for TypeScript
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

### Test naming scheme

The recommended naming scheme for screenshots is
`<application>_<component-name-sk>_<scenario>`. This is to avoid name collisions
and to be consistent with existing tests.

For example, tests for Gold's `dots-sk` component are named as follows:
- `gold_dots-sk` (default view).
- `gold_dots-sk_highlighted` (mouse over).
