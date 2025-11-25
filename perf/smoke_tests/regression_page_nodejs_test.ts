import { Browser, Page } from 'puppeteer';
import { expect } from 'chai';
import {
  applyPageDefaults,
  browserForSmokeTest,
  GetSubscriptionBaseUrl,
  PERF_BASE_URL,
} from './utils';

async function waitForTableOrClear(page: Page, testTimeout: number) {
  // Wait for the anomaly table to appear.
  const anomalyTableElem = await page.waitForSelector('#anomaly-table', { timeout: testTimeout });
  void expect(anomalyTableElem).to.not.be.null;

  // Wait for either anomaly table contents or the "All triaged" message to show up.
  // TODO(mordeckimarcin) avoid conditional logic, create a test instance with static data.
  const anomalyTablePromise = page.waitForSelector(
    '#anomaly-table sort-sk table[id^="anomalies-table-anomalies-table-sk"]:not([hidden])',
    { timeout: testTimeout }
  );
  const allTriagedPromise = page.waitForSelector(
    '#anomaly-table h1[id^="clear-msg-anomalies-table-sk"]:not([hidden])',
    { timeout: testTimeout }
  );
  // If there are untriaged anomalies, anomalyTable should become visible.
  // Otherwise, we expect to see the "All anomalies are triaged!" message.
  const foundElem = await Promise.race([anomalyTablePromise, allTriagedPromise]);
  void expect(foundElem).to.not.be.null;

  return;
}

describe('Regressions page test', async () => {
  let browser: Browser;
  let page: Page;
  const testTimeout = 10000;
  const V8JSSheriff = 'V8%20JavaScript%20Perf';
  const FuchsiaSheriff = 'Fuchsia%20Perf%20Sheriff';

  before(async () => {
    browser = await browserForSmokeTest();
  });

  beforeEach(async () => {
    page = await browser.newPage();
    page = await applyPageDefaults(page, PERF_BASE_URL);
  });

  afterEach(async () => {
    await page.close();
  });

  after(async () => {
    await browser.close();
  });

  it('should view V8 JS sheriff under 5s', async () => {
    const url = `${GetSubscriptionBaseUrl()}${V8JSSheriff}`;
    await page.goto(url);
    await waitForTableOrClear(page, testTimeout);
  }).timeout(testTimeout + 1000);

  it('should view Fuchsia sheriff under 5s', async () => {
    const url = `${GetSubscriptionBaseUrl()}${FuchsiaSheriff}`;
    await page.goto(url);
    await waitForTableOrClear(page, testTimeout);
  }).timeout(testTimeout + 1000);
});
