const expect = require('chai').expect;
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('corpus-selector-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

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
    // Basic sanity check.
    expect(await testBed.page.$$('corpus-selector-sk')).to.have.length(3);
  });

  it('should take a screenshot', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    await takeScreenshot(testBed.page, 'corpus-selector-sk');
  });
});
