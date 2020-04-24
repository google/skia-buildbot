const expect = require('chai').expect;
const path = require('path');
const addEventListenersToPuppeteerPage = require('../../../puppeteer-tests/util').addEventListenersToPuppeteerPage;
const setUpPuppeteerAndDemoPageServer = require('../../../puppeteer-tests/util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('../../../puppeteer-tests/util').takeScreenshot;

describe('corpus-selector-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.js'));

  beforeEach(async () => {
    // Listen for custom event triggered when component finishes loading.
    const eventPromise = await addEventListenersToPuppeteerPage(
      testBed.page, ['corpus-selector-sk-loaded'],
    );

    // Page has three corpus selectors so we wait until all of them have loaded.
    const loaded = Promise.all([
      eventPromise('corpus-selector-sk-loaded'),
      eventPromise('corpus-selector-sk-loaded'),
      eventPromise('corpus-selector-sk-loaded'),
    ]);
    await testBed.page.goto(`${testBed.baseUrl}/dist/corpus-selector-sk.html`);
    await loaded;
  });

  it('should render the demo page', async () => {
    // Basic smoke test that things loaded.
    expect(await testBed.page.$$('corpus-selector-sk')).to.have.length(3);
  });

  it('shows the default corpus renderer function', async () => {
    const selector = await testBed.page.$('#default');
    await takeScreenshot(selector, 'corpus-selector-sk');
  });

  it('supports a custom corpus renderer function', async () => {
    const selector = await testBed.page.$('#custom-fn');
    await takeScreenshot(selector, 'corpus-selector-sk_custom-fn');
  });

  it('handles very long strings', async () => {
    const selector = await testBed.page.$('#custom-fn-long-corpus');
    await takeScreenshot(selector, 'corpus-selector-sk_long-strings');
  });
});
