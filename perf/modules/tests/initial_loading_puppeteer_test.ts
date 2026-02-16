import { TestBed, takeScreenshot, loadCachedTestBed } from '../../../puppeteer-tests/util';
import { Page } from 'puppeteer';

// Helper function to accept the cookie banner if it exists.
async function acceptCookieBanner(page: Page) {
  const selector = 'button.glue-cookie-notification-bar__accept';
  const cookieButton = await page
    .waitForSelector(selector, { timeout: 500, visible: true })
    .catch(() => null);

  if (cookieButton) {
    await cookieButton.click();
    await page.waitForNetworkIdle({ idleTime: 500 });
  }
}

// This contains simple sanity screenshot tests for real pages.
// Do not submit CLs, if they break these tests, because this
// is what the real user sees.
describe('initial_loading_test', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    const page = testBed.page;
    const timeout = 30000;
    page.setDefaultTimeout(timeout);
    await page.setViewport({
      width: 1200,
      height: 800,
    });
  });

  it('/e', async () => {
    await testBed.page.goto(`${testBed.baseUrl}/e`);
    await acceptCookieBanner(testBed.page);
    await takeScreenshot(testBed.page, 'perf-blocking', 'explore-page');
  });

  it('/m', async () => {
    await testBed.page.goto(`${testBed.baseUrl}/m`);
    await acceptCookieBanner(testBed.page);
    await takeScreenshot(testBed.page, 'perf-blocking', 'multigraph-page');
  });

  it('/a', async () => {
    await testBed.page.goto(`${testBed.baseUrl}/a`);
    await acceptCookieBanner(testBed.page);
    await takeScreenshot(testBed.page, 'perf-blocking', 'regressions-page');
  });
});
