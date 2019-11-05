const express = require('express');
const webpack = require('webpack');
const webpackDevMiddleware = require('webpack-dev-middleware');
const webpackConfigJs = require('../webpack.config.js');

/**
 * Starts a web server that serves custom element demo pages. Equivalent to
 * running "npx webpack-dev-server" on the terminal.
 *
 * Demo pages can be accessed at the returned baseUrl, e.g.
 * `${baseUrl}/dist/my-component.html`.
 *
 * This function should be called once at the beginning of any test suite that
 * requires the custom elements demo pages. The returned function
 * stopDemoPageServer should be called at the end of the test suite, otherwise
 * the Node interpreter will hang.
 *
 * @return {Promise<{baseUrl: string, stopDemoPageServer: function}>}
 */
exports.startDemoPageServer = async () => {
  // Load and tweak Webpack configuration.
  const configuration = webpackConfigJs(null, {mode: 'development'});
  configuration.mode = 'development'; // Webpack complains if this is not set.
  // Quiet down the CleanWebpackPlugin.
  // TODO(lovisolo): Move this change to the Pulito repo.
  configuration
      .plugins
      .filter(p => p.constructor.name === 'CleanWebpackPlugin')
      .forEach(p => p.options.verbose = false);

  // This is equivalent to running "npx webpack-dev-server" on the terminal.
  const middleware = webpackDevMiddleware(webpack(configuration), {
    logLevel: 'warn',
  });
  await new Promise(resolve => middleware.waitUntilValid(resolve));

  // Start an HTTP server on a random, unused port. Serve the above middleware.
  const app = express();
  app.use(configuration.output.publicPath, middleware); // Serve on /dist.
  let server;
  const port = await new Promise(resolve => {
    server = app.listen(0, () => resolve(server.address().port));
  });

  return {
    // Base URL for the demo page server.
    baseUrl: `http://localhost:${port}`,

    // This clean-up function shuts down the HTTP server, and should be called
    // before exiting the test, otherwise the Node.js interpreter will hang.
    stopDemoPageServer: async () => {
      await Promise.all([
        new Promise(resolve => middleware.close(resolve)),
        new Promise(resolve => server.close(resolve))
      ]);
    },
  };
};

// This is a dictionary from custom event names to promise resolver functions.
const customEventPromiseResolvers = {};

/**
 * Takes a Puppeteer page and a list of custom event names, and adds event
 * listeners to the page's document for the given event names.
 *
 * Caller code can await for the given events using function customEventPromise.
 *
 * This function should be called before the page is loaded with page.goto().
 *
 * @param page A Puppeteer page.
 * @param {Array<string>>} eventNames A list of strings with event names.
 * @return {Promise<void>}
 */
exports.registerCustomEvents = async (page, eventNames) => {
  // Expose a function that will be called by the listeners for the custom
  // events.
  await page.exposeFunction('onCustomEvent', (name, details) => {
    if (customEventPromiseResolvers[name]) {
      customEventPromiseResolvers[name](details);
    }
  });

  // Add an event listener for each one of the given events.
  await eventNames.forEach(async name => {
    await page.evaluateOnNewDocument((name) => {
      document.addEventListener(name, (event) => {
        window.onCustomEvent(name, event.detail);
      })
    }, name);
  });
};

/**
 * Returns a promise that will resolve when the given custom event is caught.
 * The promise resolves the contents of the caught JS event's detail field.
 *
 * The event name should be registered with registerCustomEvents for it to work.
 *
 * @param {string} eventName
 * @return {Promise<Object>>}
 */
exports.customEventPromise = (eventName) => {
  return new Promise(resolve => {
    customEventPromiseResolvers[eventName] = resolve
  });
};