const expect = require('chai').expect;
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('ignores-page-sk', function() {
  setUpPuppeteerAndDemoPageServer();  // Sets up this.page and this.baseUrl.

  beforeEach(async function() {
    const eventPromise =
      await addEventListenersToPuppeteerPage(this.page, ['end-task']);
    const loaded = eventPromise('end-task');  // Emitted when page is loaded.
    await this.page.goto(`${this.baseUrl}/dist/ignores-page-sk.html`);
    await loaded;
  });

  it('should render the demo page', async function() {
    // Smoke test.
    expect(await this.page.$$('ignores-page-sk')).to.have.length(1);
  });

  it('should take a screenshot', async function() {
    await this.page.setViewport({ width: 1300, height: 2100 });
    await takeScreenshot(this.page, 'Test-Ignores-Page-Sk');
  });
});
