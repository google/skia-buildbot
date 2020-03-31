const expect = require('chai').expect;
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('corpus-selector-sk', () => {
  const pp = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    // Listen for custom event triggered when component finishes loading.
    const eventPromise = await addEventListenersToPuppeteerPage(
      pp.page, ['corpus-selector-sk-loaded'],
    );

    // Page has three corpus selectors so we wait until all of them have loaded.
    const loaded = Promise.all([
      eventPromise('corpus-selector-sk-loaded'),
      eventPromise('corpus-selector-sk-loaded'),
      eventPromise('corpus-selector-sk-loaded'),
    ]);
    await pp.page.goto(`${pp.baseUrl}/dist/corpus-selector-sk.html`);
    await loaded;
  });

  it('should render the demo page', async () => {
    // Basic sanity check.
    expect(await pp.page.$$('corpus-selector-sk')).to.have.length(3);
  });

  it('should take a screenshot', async () => {
    await pp.page.setViewport({ width: 1200, height: 1200 });
    await takeScreenshot(pp.page, 'corpus-selector-sk');
  });
});
