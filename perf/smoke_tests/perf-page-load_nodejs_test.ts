import { Browser, Page } from 'puppeteer';
import { expect } from 'chai';
import { GoogleAuth } from 'google-auth-library';
import { launchBrowser, takeScreenshot } from '../../puppeteer-tests/util';

// TODO(mordeckimarcin): Automate puppeteer tests.
// Make it possible to hit internal instances.
// ----------------------------------------------------------------------
// Replace with your IAP Client ID. You can find this in the IAP settings
// in your Google Cloud Console for the chrome-perf.corp.goog application.
const IAP_CLIENT_ID = 'YOUR_IAP_CLIENT_ID.apps.googleusercontent.com';

async function pageWithAuth(browser: Browser) {
  const page = await browser.newPage();
  console.log('Attempting to authenticate...');
  const auth = new GoogleAuth();
  const client = await auth.getIdTokenClient(IAP_CLIENT_ID);
  const headers = await client.getRequestHeaders();
  await page.setExtraHTTPHeaders({
    Authorization: headers.Authorization,
  });
  console.log('Authentication headers set.');
  return page;
}

describe('Perf page load test', async () => {
  // Won't work if we hit a SSO protected instance,
  // or a local instance based off a protected one, and not run auth-proxy.
  const perfBaseUrl = 'http://localhost:8002';
  let browser: Browser;
  let page: Page;

  before(async () => {
    browser = await launchBrowser();
    page = await pageWithAuth(browser);

    page.on('console', (msg) => console.log(`PAGE LOG[${msg.type()}]: ${msg.text()}`));
    page.on('pageerror', (message) => console.log('PAGE ERROR: ', message));
    page.on('response', (response) =>
      console.log(`RESPONSE LOG[${response.status()}]: ${response.url()}`)
    );
    page.on('requestfailed', (request) =>
      console.log(`REQUEST FAILED: [${request.url()}] ${request.failure()?.errorText}`)
    );

    // Tell demo pages this is a Puppeteer test. Demo pages should not fake RPC
    // latency, render animations or exhibit any other non-deterministic
    // behavior that could result in differences in the screenshots uploaded to
    // Gold.
    await page.setCookie({
      url: perfBaseUrl,
      name: 'puppeteer',
      value: 'true',
    });
  });

  afterEach(async () => {
    await page.close();
  });

  after(async () => {
    await browser.close();
  });

  it('should load the /a/ page within 5 seconds', async () => {
    const url = `${perfBaseUrl}/a?selectedSubscription=V8%20JavaScript%20Perf`;
    try {
      await page.goto(url);

      // Wait for the anomaly table to appear.
      const element = await page.waitForSelector('#anomaly-table', { timeout: 5000 });
      void expect(element).to.not.be.null;
      const element1 = await page.waitForSelector(
        '#anomaly-table sort-sk table[id^="anomalies-table-anomalies-table-sk"]:not([hidden])',
        { timeout: 5000 }
      );
      void expect(element1).to.not.be.null;
    } catch (e) {
      await takeScreenshot(page, 'perf', 'page-load-failure');
      throw e;
    }
  }).timeout(6000); // 5 seconds for loading + 1 second buffer
});
