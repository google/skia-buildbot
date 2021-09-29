/**
 * Wraps the value toReturn in a Promise that will resolve after "delay" milliseconds. Used to fake
 * RPC latency in demo pages.
 *
 * @param toReturn Either the body to be returned in a 200 JSON response or a function that
 *     returns the fetch-mock response. RPC latency will be reduced when running as a
 *     Puppeteer test.
 * @param delayMs Delay in milliseconds.
 */
export function delay<T>(toReturn: T | (()=> T), delayMs = 100): ()=> Promise<T | Response> {
  // We return a function that returns the promise so each call has a "fresh" promise and waits
  // for the allotted time.
  return function() {
    // For puppeteer tests, we want more deterministic page loads; removing the time does this.
    if (isPuppeteerTest()) {
      delayMs = 0;
    }
    let returnValue: T | Response;
    if (toReturn instanceof Function) {
      returnValue = toReturn();
    } else {
      returnValue = {
        status: 200,
        body: JSON.stringify(toReturn),
        headers: { 'content-type': 'application/json' },
      } as unknown as Response;
    }

    return new Promise((resolve) => {
      setTimeout(resolve, delayMs);
    }).then(() => returnValue);
  };
}

/** Returns true if the page is running from within a Puppeteer-managed browser. */
export function isPuppeteerTest(): boolean {
  return document.cookie.includes('puppeteer');
}

/** Returns true if the page is served under Bazel via an sk_demo_page_server Bazel rule. */
export function isBazelDemoPage(): boolean {
  return document.cookie.includes('bazel=true');
}
