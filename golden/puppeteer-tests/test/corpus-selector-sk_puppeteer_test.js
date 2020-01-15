const expect = require('chai').expect;
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('corpus-selector-sk', function() {
  setUpPuppeteerAndDemoPageServer();  // Sets up this.page and this.baseUrl.

  beforeEach(async function() {
    // Listen for custom event triggered when component finishes loading.
    const eventPromise =
        await addEventListenersToPuppeteerPage(
            this.page, ['corpus-selector-sk-loaded']);

    // Page has three corpus selectors so we wait until all of them have loaded.
    const loaded = Promise.all([
        eventPromise('corpus-selector-sk-loaded'),
        eventPromise('corpus-selector-sk-loaded'),
        eventPromise('corpus-selector-sk-loaded'),
    ]);
    await this.page.goto(`${this.baseUrl}/dist/corpus-selector-sk.html`);
    await loaded;
  });

  it('should render the demo page', async function() {
    // Basic sanity check.
    expect(await this.page.$$('corpus-selector-sk')).to.have.length(3);
  });

  it('should take a screenshot', async function() {
    await this.page.setViewport({ width: 1200, height: 1200 });
    await takeScreenshot(this.page, 'corpus-selector-sk');
  });
});
