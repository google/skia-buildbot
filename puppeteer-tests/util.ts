import express from 'express';
import * as fs from 'fs';
import * as path from 'path';
import * as http from 'http';
import * as net from 'net';
import puppeteer from 'puppeteer';
import webpack from 'webpack';
import webpackDevMiddleware from 'webpack-dev-middleware';

// File inside $ENV_DIR containing the demo page server's TCP port. Only applies to Bazel tests
// using the test_on_env rule.
const ENV_PORT_FILE_BASE_NAME = 'port';

/** A DOM event name. */
export type EventName = string;

/**
 * Type of the function returned by addEventListenersToPuppeteerPage.
 *
 * It returns a promise that resolves when an event e of the given name is
 * caught, and returns e.detail (assumed to be of type T).
 *
 * The generic type variable T is analogous to T in e.g. CustomEvent<T>.
 *
 * Note: this works for standard DOM events as well, not just custom events.
 */
export type EventPromiseFactory = <T>(eventName: EventName) => Promise<T>;

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
 */
export const addEventListenersToPuppeteerPage = async (page: puppeteer.Page, eventNames: EventName[]) => {
  // Maps event names to FIFO queues of promise resolver functions.
  const resolverFnQueues = new Map<EventName, Function[]>();
  eventNames.forEach((eventName) => resolverFnQueues.set(eventName, []));

  // Use an unlikely prefix to reduce chances of name collision.
  await page.exposeFunction('__pptr_onEvent', (eventName: EventName, eventDetail: any) => {
    const resolverFn = resolverFnQueues.get(eventName)!.shift(); // Dequeue.
    if (resolverFn) { // Undefined if queue length was 0.
      resolverFn(eventDetail);
    }
  });

  // This function will be executed inside the Puppeteer page for each of the
  // events we want to listen for. It adds an event listener that will call the
  // function we've exposed in the previous step.
  const addEventListener = (name: EventName) => {
    document.addEventListener(name, (event: Event) => {
      (window as any).__pptr_onEvent(name, (event as any).detail);
    });
  };

  // Add an event listener for each one of the given events.
  const promises = eventNames.map((name) => page.evaluateOnNewDocument(addEventListener, name));
  await Promise.all(promises);

  // The returned function takes an event name and returns a promise that will
  // resolve to the event details when the event is caught.
  const eventPromiseFactory: EventPromiseFactory = (eventName: EventName) => {
    if (!resolverFnQueues.has(eventName)) {
      // Fail if the event wasn't included in eventNames.
      throw new Error(`no event listener for "${eventName}"`);
    }
    return new Promise(
      // Enqueue resolver function at the end of the queue.
      (resolve) => resolverFnQueues.get(eventName)!.push(resolve),
    );
  };

  return eventPromiseFactory;
};

/**
 * Returns true if running from within a Docker container, or false otherwise.
 */
export const inDocker = () => fs.existsSync('/.dockerenv');

/**
 * Returns true if running from Bazel (e.g. with "bazel test"), or false otherwise.
 */
export const inBazel = () => !!process.env.BAZEL_WORKSPACE;

/**
 * Launches a Puppeteer browser.
 */
export const launchBrowser = () => puppeteer.launch(
  // These options are required to run Puppeteer from within a Docker container, as is the case
  // under Bazel and RBE. See
  // https://github.com/puppeteer/puppeteer/blob/master/docs/troubleshooting.md#running-puppeteer-in-docker.
  //
  // Flag --no-sandbox is necessary to run Puppeteer tests under Bazel locally (i.e. not on RBE) on
  // a Swarming bot. If we do not provide said flag, we get the following error:
  //
  //     No usable sandbox! Update your kernel or see
  //     https://chromium.googlesource.com/chromium/src/+/master/docs/linux/suid_sandbox_development.md
  //     for more information on developing with the SUID sandbox. If you want to live dangerously
  //     and need an immediate workaround, you can try using --no-sandbox.
  { args: ['--disable-dev-shm-usage', '--no-sandbox'] },
);

/**
 * Type of the object returned by setUpPuppeteerAndDemoPageServer.
 *
 * A test suite should reuse this object in all its test cases. This object's
 * fields will be automatically updated with a fresh page and base URL before
 * each test case is executed.
 */
export interface TestBed {
  page: puppeteer.Page;
  baseUrl: string;
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
 */
export const startWebpackDemoPageServer = async (pathToWebpackConfigTs: string) => {
  // Load Webpack configuration.
  const webpackConfigFactory = require(pathToWebpackConfigTs) as webpack.ConfigurationFactory;
  const configuration = webpackConfigFactory('', { mode: 'development' }) as webpack.Configuration;

  // Prevent the creation of source maps, which can consume lots of memory.
  // https://github.com/webpack/webpack-sources/issues/66#issuecomment-537526984
  delete configuration.devtool;

  // This is equivalent to running "npx webpack-dev-server" on the terminal.
  const middleware = webpackDevMiddleware(webpack(configuration), {
    logLevel: 'warn', // Do not print summary on startup.
  } as any);
  await new Promise((resolve) => middleware.waitUntilValid(resolve));

  // Start an HTTP server on a random, unused port. Serve the above middleware.
  const app = express();
  app.use(configuration.output!.publicPath! || '', middleware); // Serve on e.g. /dist.
  let server: http.Server;
  await new Promise((resolve) => { server = app.listen(0, resolve); });

  return {
    // Base URL for the demo page server.
    baseUrl: `http://localhost:${(server!.address() as net.AddressInfo).port}`,

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
 * Returns the output directory where tests should e.g. save screenshots.
 * Screenshots saved in this directory will be uploaded to Gold.
 */
export const outputDir = () => {
  // When running via "bazel test", screenshots for e.g. //path/to/my:puppeteer_test will be found
  // at //bazel-testlogs/path/to/my/puppeteer_test/test.outputs/outputs.zip. This is true when
  // running on RBE as well (e.g. "bazel test --config=remote").
  //
  // See the following link for more:
  // https://docs.bazel.build/versions/master/test-encyclopedia.html#test-interaction-with-the-filesystem.
  if (exports.inBazel()) {
    const undeclaredOutputsDir = process.env.TEST_UNDECLARED_OUTPUTS_DIR;
    if (!undeclaredOutputsDir) {
      throw new Error('required environment variable TEST_UNDECLARED_OUTPUTS_DIR is unset');
    }
    const outputDir = path.join(undeclaredOutputsDir, 'puppeteer-test-screenshots');
    if (!fs.existsSync(outputDir)) {
      fs.mkdirSync(outputDir);
    }
    return outputDir;
  }

  // In the pre-Bazel world, Puppeteer tests either run inside the puppeteer-tests Docker container
  // (e.g. Infra-PerCommit-Puppeteer), or locally during development.
  if (exports.inDocker()) {
    return '/out';
  }

  // Resolves to //puppeteer-tests/output when running locally.
  return path.join(__dirname, 'output');
};

/**
 * Takes a screenshot and saves it to the tests output directory to be uploaded
 * to Gold.
 *
 * The screenshot will be saved as <appName>_<testName>.png. Using the
 * application name as a prefix prevents name collisions between different apps
 * and increases consistency among test names.
 */
export const takeScreenshot =
    (handle: puppeteer.Page | puppeteer.ElementHandle, appName: string, testName: string) =>
  handle.screenshot({path: path.join(exports.outputDir(), `${appName}_${testName}.png`)});


let browser: puppeteer.Browser;
let testBed: Partial<TestBed>;

/**
 * Once per Mocha invocation, loadCachedTestBed will compile all the demo pages and launch a
 * Puppeteer browser window to run the tests. On all subsequent calls, it will return essentially
 * a cached handle to that invocation.
 *
 * Test cases can access the demo page server's base URL and a Puppeteer page ready to be used via
 * the return value's baseUrl and page objects, respectively.
 *
 * This function assumes that each test case uses exactly one Puppeteer page (that's why it doesn't
 * expose the Browser instance to tests). The page is set up with a cookie (name: "puppeteer",
 * value: "true") to give demo pages a means to detect whether they are running within Puppeteer or
 * not.
 */
export async function loadCachedTestBed(pathToWebpackConfigTs: string) {
  if (testBed) {
    return testBed as TestBed;
  }
  const newTestBed: Partial<TestBed> = {};

  if (inBazel()) {
    // Read the demo page server's TCP port.
    const envDir = process.env.ENV_DIR;
    if (!envDir) throw new Error('required environment variable ENV_DIR is unset');
    const port = parseInt(fs.readFileSync(path.join(envDir, ENV_PORT_FILE_BASE_NAME), 'utf8'));
    newTestBed.baseUrl = `http://localhost:${port}`
  } else {
    // Not in Bazel - Start the Webpack-based demo page server.
    let baseUrl;
    ({ baseUrl } = await startWebpackDemoPageServer(pathToWebpackConfigTs));
    newTestBed.baseUrl = baseUrl; // Make baseUrl available to tests.
  }

  browser = await launchBrowser();
  testBed = newTestBed;
  setBeforeAfterHooks();
  return testBed as TestBed;
}

// This sets up some handy helpers to load a new page and shut it down w/o having to expose
// the puppeteer.Browser object to the callers.
function setBeforeAfterHooks() {
  beforeEach(async () => {
    testBed.page = await browser.newPage(); // Make page available to tests.

    // Tell demo pages this is a Puppeteer test. Demo pages should not fake RPC
    // latency, render animations or exhibit any other non-deterministic
    // behavior that could result in differences in the screenshots uploaded to
    // Gold.
    await testBed.page.setCookie({
      url: testBed.baseUrl,
      name: 'puppeteer',
      value: 'true'
    });
  });

  afterEach(async () => {
    await testBed.page!.close();
  });

  // When running under Bazel, we need to explicitly shut down Puppeteer, otherwise tests will run
  // forever, eventually timing out and failing.
  if (inBazel()) {
    after(async () => {
      await browser.close();
    });
  }
}