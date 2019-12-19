const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer');

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
 * @param {Object} page A Puppeteer page.
 * @param {Array<string>} eventNames Event names to listen to.
 * @return {Promise<Function>} Event promise builder function.
 */
exports.addEventListenersToPuppeteerPage = async (page, eventNames) => {
  // Maps event names to FIFO queues of promise resolver functions.
  const resolverFnQueues = {};
  eventNames.forEach((eventName) => resolverFnQueues[eventName] = []);

  // Use an unlikely prefix to reduce chances of name collision.
  await page.exposeFunction('__pptr_onEvent', (eventName, eventDetail) => {
    const resolverFn = resolverFnQueues[eventName].shift();  // Dequeue.
    if (resolverFn) {  // Undefined if queue length was 0.
      resolverFn(eventDetail);
    }
  });

  // Add an event listener for each one of the given events.
  await eventNames.forEach(async (name) => {
    await page.evaluateOnNewDocument((name) => {
      document.addEventListener(name, (event) => {
        window.__pptr_onEvent(name, event.detail);
      })
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
        (resolve) => resolverFnQueues[eventName].push(resolve));
  }
};

/**
 * Returns true if running from within a Docker container, or false otherwise.
 * @return {boolean}
 */
exports.inDocker = () => fs.existsSync('/.dockerenv');

/**
 * Launches a Puppeteer browser with the right platform-specific arguments.
 * @return {Promise}
 */
exports.launchBrowser =
    () => puppeteer.launch(
        // See
        // https://github.com/puppeteer/puppeteer/blob/master/docs/troubleshooting.md#running-puppeteer-in-docker.
        exports.inDocker()
            ? { args: ['--disable-dev-shm-usage', '--no-sandbox'] }
            : {});

/**
 * Returns the output directory where tests should e.g. save screenshots.
 * Screenshots saved in this directory will be uploaded to Gold.
 * @return {string}
 */
exports.outputDir =
    () => exports.inDocker()
        ? '/out'
        // Resolves to $SKIA_INFRA_ROOT/golden/puppeteer-tests/output.
        : path.join(__dirname, '..', 'output');

/**
 * Takes a screenshot and saves it to the tests output directory to be uploaded
 * to Gold.
 * @param {Object} page Puppeteer page.
 * @param {string} testName Test name, e.g. "Test-Foo-Bar".
 * @return {Promise}
 */
exports.takeScreenshot =
    (page, testName) =>
        page.screenshot({
          path: path.join(exports.outputDir(), `${testName}.png`)
        });
