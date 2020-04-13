/**
 * Wraps the value toReturn in a Promise that will resolve after "delay" milliseconds. It is used
 * to fake RPC latency in demo pages. RPC latency will be reduced when running as a Puppeteer test.
 * @param toReturn {Object} Object to be returned.
 * @param delayMs {number} Delay in milliseconds.
 * @return {Function}
 */
export function delay(toReturn, delayMs = 100) {
  // We return a function that returns the promise so each call has a "fresh" promise and waits
  // for the allotted time.
  return function() {
    // For puppeteer tests, we want more deterministic page loads; removing the time does this.
    if (isPuppeteerTest()) {
      delayMs = 0;
    }
    return new Promise((resolve) => {
      setTimeout(resolve, delayMs);
    }).then(() => ({
      status: 200,
      body: JSON.stringify(toReturn),
      headers: { 'content-type': 'application/json' },
    }));
  };
}

/**
 * Returns true if the page is running from within a Puppeteer-managed browser.
 * @return {boolean}
 */
export function isPuppeteerTest() {
  return document.cookie.indexOf('puppeteer') !== -1;
}
