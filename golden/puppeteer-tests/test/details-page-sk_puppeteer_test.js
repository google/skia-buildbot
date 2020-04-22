const expect = require('chai').expect;
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('details-page-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.
  const params = '?digest=6246b773851984c726cb2e1cb13510c2&test=My%20test%20has%20spaces';

  it('should render the demo page', async () => {
    await navigateTo(testBed.page, testBed.baseUrl, params);
    // Smoke test.
    expect(await testBed.page.$$('details-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('should show the default page', async () => {
      await navigateTo(testBed.page, testBed.baseUrl, params);
      await testBed.page.setViewport({
        width: 1300,
        height: 700,
      });
      await takeScreenshot(testBed.page, 'details-page-sk');
    });

    it('should show the digest even if it is not in the index', async () => {
      await navigateTo(testBed.page, testBed.baseUrl, params);
      await testBed.page.setViewport({
        width: 1300,
        height: 700,
      });
      await testBed.page.click('#simulate-not-found-in-index');
      await takeScreenshot(testBed.page, 'details-page-sk_not-in-index');
    });
  });
});

async function navigateTo(page, base, queryParams = '') {
  const eventPromise = await addEventListenersToPuppeteerPage(page, ['end-task']);
  const loaded = eventPromise('end-task'); // Emitted when page is loaded.
  await page.goto(`${base}/dist/details-page-sk.html${queryParams}`);
  await loaded;
}
