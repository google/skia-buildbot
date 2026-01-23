import { expect } from 'chai';
import { Browser, launch, Page } from 'puppeteer';

describe('Example.com Load Test', () => {
  let baseUrl: string;
  let browser: Browser;
  let page: Page;

  before(async () => {
    baseUrl = `https://example.com`;
    browser = await launch({
      executablePath: process.env.CHROME_BIN,
      args: ['--disable-dev-shm-usage', '--no-sandbox'],
    });
  });

  after(async () => {
    await browser.close();
  });

  beforeEach(async () => {
    page = await browser.newPage();
  });

  it('should load example.com and verify the title', async () => {
    await page.goto(baseUrl);
    const title = await page.title();
    expect(title).to.equal('Example Domain');
  });
});
