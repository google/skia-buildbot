import { Browser, Page } from 'puppeteer';
import { GoogleAuth } from 'google-auth-library';
import { launchBrowser, takeScreenshot } from '../../puppeteer-tests/util';
import { PERF_BASE_URL, applyPageDefaults } from './utils';

// TODO(b/473814161): Update IAP_CLIENT_ID with correct value.
const IAP_CLIENT_ID = 'YOUR_IAP_CLIENT_ID.apps.googleusercontent.com';

async function pageWithAuth(browser: Browser) {
  const page = await browser.newPage();
  const auth = new GoogleAuth();
  const client = await auth.getIdTokenClient(IAP_CLIENT_ID);
  const headers = await client.getRequestHeaders();
  await page.setExtraHTTPHeaders({
    Authorization: headers.Authorization,
  });
  return page;
}

describe('Alerts Smoke Test', async () => {
  let browser: Browser;
  let page: Page;

  before(async () => {
    browser = await launchBrowser();
    page = await pageWithAuth(browser);
    await applyPageDefaults(page, PERF_BASE_URL);
  });

  after(async () => {
    await page.close();
    await browser.close();
  });

  it('should load the alerts page', async () => {
    await page.goto(`${PERF_BASE_URL}/alerts`, { waitUntil: 'networkidle0' });
    try {
      await page.waitForSelector('alerts-page-sk', { timeout: 10000 });
    } catch (e) {
      await takeScreenshot(page, 'perf', 'alerts-smoke-failure');
      throw e;
    }
  }).timeout(15000);
});
