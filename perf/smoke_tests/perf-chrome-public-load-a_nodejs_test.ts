import { Browser, Page } from 'puppeteer';
import { expect } from 'chai';
import { launchBrowser, takeScreenshot } from '../../puppeteer-tests/util';

describe('Perf page load test', async () => {
  const perfBaseUrl = 'http://localhost:8003';
  let browser: Browser;
  let page: Page;

  before(async () => {
    browser = await launchBrowser();
  });

  beforeEach(async () => {
    page = await browser.newPage();
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
    const url = `${perfBaseUrl}/a`;
    try {
      await page.goto(url);

      // Wait for the anomaly table to appear.
      const element = await page.waitForSelector('#anomaly-table', { timeout: 5000 });
      void expect(element).to.not.be.null;
    } catch (e) {
      await takeScreenshot(page, 'perf', 'page-load-failure');
      throw e;
    }
  }).timeout(6000); // 5 seconds for loading + 1 second buffer

  it('should load the /m/ test picker within 5 seconds', async () => {
    const url = `${perfBaseUrl}/m`;
    try {
      await page.goto(url);

      // Wait for the test picker to appear.
      const element = await page.waitForSelector('#test-picker', { timeout: 5000 });
      void expect(element).to.not.be.null;
    } catch (e) {
      await takeScreenshot(page, 'perf', 'page-load-failure');
      throw e;
    }
  }).timeout(6000); // 5 seconds for loading + 1 second buffer

  it('should load the query-dialog for the New Query page within 5 seconds', async () => {
    const url = `${perfBaseUrl}/e`;
    try {
      await page.goto(url);

      // Wait for the test picker to appear.
      const element = await page.waitForSelector('#query-dialog', { timeout: 5000 });
      void expect(element).to.not.be.null;
    } catch (e) {
      await takeScreenshot(page, 'perf', 'page-load-failure');
      throw e;
    }
  }).timeout(6000); // 5 seconds for loading + 1 second buffer
});
