const expect = require('chai').expect;
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('byblame-page-sk', function() {
  setUpPuppeteerAndDemoPageServer();  // Sets up this.page and this.baseUrl.

  beforeEach(async function() {
    const eventPromise =
        await addEventListenersToPuppeteerPage(this.page, ['end-task']);
    const loaded = eventPromise('end-task');  // Emitted when page is loaded.
    await this.page.goto(`${this.baseUrl}/dist/byblame-page-sk.html`);
    await loaded;
  });

  it('should render the demo page', async function() {
    // Smoke test.
    expect(await this.page.$$('byblame-page-sk')).to.have.length(1);
  });

  it('should take a screenshot', async function() {
    await this.page.setViewport({ width: 1200, height: 7300 });
    await takeScreenshot(this.page, 'Test-Byblame-Page-Sk');
  });
});
