/**
 * Wraps the value toReturn in a Promise that will resolve after "delay"
 * milliseconds. Used to fake RPC latency in demo pages.
 * @param toReturn {Object|Function} Either the body to be returned in a 200 JSON response or a
 *     function that returns the fetch-mock response. RPC latency will be reduced when running as
 *     a Puppeteer test.
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
    let returnValue;
    if (typeof toReturn === 'function') {
      returnValue = toReturn();
    } else {
      returnValue = {
        status: 200,
        body: JSON.stringify(toReturn),
        headers: { 'content-type': 'application/json' },
      };
    }

    return new Promise((resolve) => {
      setTimeout(resolve, delayMs);
    }).then(() => returnValue);
  };
}

/**
 * Returns true if the page is running from within a Puppeteer-managed browser.
 * @return {boolean}
 */
export function isPuppeteerTest() {
  return document.cookie.indexOf('puppeteer') !== -1;
}
