/**
 * Wraps the value toReturn in a Promise that will resolve after "delay"
 * milliseconds. Used to fake RPC latency in demo pages.
 * @param toReturn {Object} Object to be returned.
 * @param delay {number} Delay in milliseconds.
 * @return {Function}
 */
exports.delay = function(toReturn, delay = 100) {
  // We return a function that returns the promise so each call has a "fresh"
  // promise and waits for the time.
  return function() {
    return new Promise((resolve) => {
      setTimeout(resolve, delay);
    }).then(() => ({
      status: 200,
      body: JSON.stringify(toReturn),
      headers: { 'content-type': 'application/json' },
    }));
  };
};

/**
 * Returns true if the page is running from within a Puppeteer-managed browser.
 * @return {boolean}
 */
exports.isPuppeteerTest = () => document.cookie.indexOf('puppeteer') !== -1;
