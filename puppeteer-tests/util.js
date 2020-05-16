const express = require('express');
const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer');
const webpack = require('webpack');
const webpackDevMiddleware = require('webpack-dev-middleware');

/**
 * https://www.typescriptlang.org/docs/handbook/jsdoc-supported-types.html#import-types
 *
 * @typedef { import("puppeteer").Browser } Browser
 * @typedef { import("puppeteer").Page } Page
 * @typedef { import("puppeteer").ElementHandle } ElementHandle
 */

/**
 * This function allows tests to catch document-level events in a Puppeteer
 * page.
 *
 * It takes a Puppeteer page and a list of event names, and adds event listeners
 * to the page's document for the given events. It must be called before the
 * page is loaded with e.g. page.goto() for it to work.
 *
 * The returned function takes an event name in eventNames and returns a promise
 * that will resolve to the corresponding Event object's "detail" field when the
 * event is caught. Multiple promises for the same event will be resolved in the
 * order that they were created, i.e. one caught event resolves the oldest
 * pending promise.
 *
 * @param {Page} page A Puppeteer page.
 * @param {Array<string>} eventNames Event names to listen to.
 * @return {Promise<Function>} Event promise builder function.
 */
exports.addEventListenersToPuppeteerPage = async (page, eventNames) => {
  // Maps event names to FIFO queues of promise resolver functions.
  const resolverFnQueues = {};
  eventNames.forEach((eventName) => resolverFnQueues[eventName] = []);

  // Use an unlikely prefix to reduce chances of name collision.
  await page.exposeFunction('__pptr_onEvent', (eventName, eventDetail) => {
    const resolverFn = resolverFnQueues[eventName].shift(); // Dequeue.
    if (resolverFn) { // Undefined if queue length was 0.
      resolverFn(eventDetail);
    }
  });

  // Add an event listener for each one of the given events.
  await eventNames.forEach(async (name) => {
    await page.evaluateOnNewDocument((name) => {
      document.addEventListener(name, (event) => {
        window.__pptr_onEvent(name, event.detail);
      });
    }, name);
  });

  // The returned function takes an event name and returns a promise that will
  // resolve to the event details when the event is caught.
  return (eventName) => {
    if (resolverFnQueues[eventName] === undefined) {
      // Fail if the event wasn't included in eventNames.
      throw new Error(`no event listener for "${eventName}"`);
    }
    return new Promise(
      // Enqueue resolver function at the end of the queue.
      (resolve) => resolverFnQueues[eventName].push(resolve),
    );
  };
};

/**
 * Returns true if running from within a Docker container, or false otherwise.
 * @return {boolean}
 */
exports.inDocker = () => fs.existsSync('/.dockerenv');

/**
 * Launches a Puppeteer browser with the right platform-specific arguments.
 * @return {Promise<Browser>}
 */
exports.launchBrowser = () => puppeteer.launch(
  // See
  // https://github.com/puppeteer/puppeteer/blob/master/docs/troubleshooting.md#running-puppeteer-in-docker.
  exports.inDocker()
    ? { args: ['--disable-dev-shm-usage', '--no-sandbox'] }
    : {},
);

/**
 * Returns the output directory where tests should e.g. save screenshots.
 * Screenshots saved in this directory will be uploaded to Gold.
 * @return {string}
 */
exports.outputDir = () => (exports.inDocker()
  ? '/out'
  : path.join(__dirname, 'output')); // Resolves to //puppeteer-tests/output for local development.

/**
 * This function sets up the before(Each) and after(Each) hooks required for
 * test suites that take screenshots of demo pages.
 *
 * Test cases can access the demo page server's base URL and a Puppeteer page
 * ready to be used via the return value's baseUrl and page objects, respectively.
 *
 * This function assumes that each test case uses exactly one Puppeteer page
 * (that's why it doesn't expose the Browser instance to tests). The page is set
 * up with a cookie (name: "puppeteer", value: "true") to give demo pages a
 * means to detect whether they are running within Puppeteer or not.
 *
 * Call this function at the beginning of a Mocha describe() block.
 *
 * @param {string} pathToWebpackConfigJs Path to the webpack.config.js file.
 * @return {{page: Page, baseUrl: string}} Test bed object.
 */
exports.setUpPuppeteerAndDemoPageServer = (pathToWebpackConfigJs) => {
  let browser;
  let stopDemoPageServer;
  const testBed = {
    page: null,
    baseUrl: null,
  };

  before(async () => {
    let baseUrl;
    ({ baseUrl, stopDemoPageServer } = await exports.startDemoPageServer(pathToWebpackConfigJs));
    testBed.baseUrl = baseUrl; // Make baseUrl available to tests.
    browser = await exports.launchBrowser();
  });

  after(async () => {
    await browser.close();
    await stopDemoPageServer();
  });

  beforeEach(async () => {
    testBed.page = await browser.newPage(); // Make page available to tests.
    // Tell demo pages this is a Puppeteer test. Demo pages should not fake RPC
    // latency, render animations or exhibit any other non-deterministic
    // behavior that could result in differences in the screenshots uploaded to
    // Gold.
    await testBed.page.setCookie(
      { url: testBed.baseUrl, name: 'puppeteer', value: 'true' },
    );
  });

  afterEach(async () => {
    await testBed.page.close();
  });

  return testBed;
};

/**
 * Starts a web server that serves custom element demo pages. Equivalent to
 * running "npx webpack-dev-server" on the terminal.
 *
 * Demo pages can be accessed at the returned baseUrl. For example, page
 * my-component-sk-demo.html is found at `${baseUrl}/dist/my-component-sk.html`.
 *
 * This function should be called once at the beginning of any test suite that
 * requires custom element demo pages. The returned function stopDemoPageServer
 * should be called at the end of the test suite.
 *
 * @param {string} pathToWebpackConfigJs Path to the webpack.config.js file.
 * @return {Promise<{baseUrl: string, stopDemoPageServer: function}>}
 */
exports.startDemoPageServer = async (pathToWebpackConfigJs) => {
  // Load Webpack configuration.
  const webpackConfigJs = require(pathToWebpackConfigJs);
  const configuration = webpackConfigJs(null, {});

  // See https://webpack.js.org/configuration/mode/.
  configuration.mode = 'development';

  // Quiet down the CleanWebpackPlugin.
  // TODO(lovisolo): Move this change to the Pulito repo.
  configuration
    .plugins
    .filter((p) => p.constructor.name === 'CleanWebpackPlugin')
    .forEach((p) => p.options.verbose = false);

  // This is equivalent to running "npx webpack-dev-server" on the terminal.
  const middleware = webpackDevMiddleware(webpack(configuration), {
    logLevel: 'warn', // Do not print summary on startup.
  });
  await new Promise((resolve) => middleware.waitUntilValid(resolve));

  // Start an HTTP server on a random, unused port. Serve the above middleware.
  const app = express();
  app.use(configuration.output.publicPath, middleware); // Serve on /dist.
  let server;
  await new Promise((resolve) => { server = app.listen(0, resolve); });

  return {
    // Base URL for the demo page server.
    baseUrl: `http://localhost:${server.address().port}`,

    // Call this function to shut down the HTTP server after tests are finished.
    stopDemoPageServer: async () => {
      await Promise.all([
        new Promise((resolve) => middleware.close(resolve)),
        new Promise((resolve) => server.close(resolve)),
      ]);
    },
  };
};

/**
 * Takes a screenshot and saves it to the tests output directory to be uploaded
 * to Gold.
 *
 * The screenshot will be saved as <appName>_<testName>.png. Using the
 * application name as a prefix prevents name collisions between different apps
 * and increases consistency among test names.
 *
 * @param {Page | ElementHandle} handle Puppeteer Page or ElementHandle instance.
 * @param {string} appName Application name, e.g. 'gold'.
 * @param {string} testName Test name, e.g. 'my-component-sk_mouse-over'.
 * @return {Promise<Buffer>}
 */
exports.takeScreenshot = (handle, appName, testName) => handle.screenshot({
  path: path.join(exports.outputDir(), `${appName}_${testName}.png`),
});
